import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { backend, newOpId, onConnectionStep, onJobUpdate, onNewNotification, onUpdateStatus } from '../lib/backend'
import { errorTitle, toAppError, type AppError } from '../lib/errors'
import { setKeymapOverrides } from '../lib/shortcuts'
import { tabsReducer, type TabsAction, type TabsState } from '../lib/tabs'
import type {
  AppInfo,
  ConnectionStep,
  Job,
  PlatformNotification,
  Profile,
  ProfileInput,
  SessionInfo,
  Settings,
  TestResult,
  UpdateStatus,
} from '../lib/types'

export interface Toast {
  id: number
  kind: 'info' | 'success' | 'error'
  title: string
  message?: string
}

export interface ConfirmOptions {
  title: string
  message: ReactNode
  confirmLabel?: string
  danger?: boolean
  /** When set, the user must type this text to enable the confirm button. */
  typeToConfirm?: string
}

interface ConfirmRequest extends ConfirmOptions {
  resolve: (ok: boolean) => void
}

export interface PromptOptions {
  title: string
  label: string
  initial?: string
  confirmLabel?: string
}

interface PromptRequest extends PromptOptions {
  resolve: (value: string | null) => void
}

/** The connection log dialog: each step of a Test or Connect as it happens. */
export interface ConnectionLog {
  opId: string
  kind: 'test' | 'connect'
  /** For Connect: the cancel id (the backend runs it as connect:<profile id>). */
  cancelId: string
  title: string
  steps: ConnectionStep[]
  status: 'running' | 'ok' | 'failed'
  error?: AppError
  summary?: string
  startedAt: number
  finishedAt?: number
}

interface AppState {
  settings: Settings | null
  saveSettings: (s: Settings) => Promise<void>
  appInfo: AppInfo | null

  profiles: Profile[]
  reloadProfiles: () => Promise<void>
  /** Saves a new connection order (drag to reorder). */
  reorderProfiles: (ids: string[]) => Promise<void>
  sessions: SessionInfo[]
  connect: (profileId: string) => Promise<SessionInfo | null>
  disconnect: (connId: string) => Promise<void>
  connecting: Set<string>

  tabs: TabsState
  dispatchTabs: (a: TabsAction) => void

  toast: (t: Omit<Toast, 'id'>) => void
  toastError: (err: unknown, title?: string) => AppError
  toasts: Toast[]
  dismissToast: (id: number) => void

  confirm: (o: ConfirmOptions) => Promise<boolean>
  confirmRequest: ConfirmRequest | null
  resolveConfirm: (ok: boolean) => void

  connectionManagerOpen: boolean
  /** Opens (or closes) the connection manager; `select` picks a profile, 'new' starts a blank one. */
  openConnectionManager: (open: boolean, select?: string | 'new') => void
  connectionManagerSelect: string | 'new' | undefined

  /** Tabs with unsaved work: tab id → what would be lost. */
  dirtyTabs: Record<string, string>
  setTabDirty: (tabId: string, reason: string | null) => void
  /** Closes a tab, asking first when it has unsaved work (unless force). Resolves false if the user kept it. */
  closeTab: (tabId: string, force?: boolean) => Promise<boolean>
  closeOtherTabs: (tabId: string) => Promise<void>

  connectionLog: ConnectionLog | null
  closeConnectionLog: () => void
  /** Tests unsaved connection settings, showing the connection log. Throws on failure. */
  testConnection: (form: ProfileInput) => Promise<TestResult>

  shortcutsOpen: boolean
  setShortcutsOpen: (open: boolean) => void
  aboutOpen: boolean
  setAboutOpen: (open: boolean) => void

  /** Background jobs, newest first. */
  jobs: Job[]
  refreshJobs: () => void
  prompt: (o: PromptOptions) => Promise<string | null>
  promptRequest: PromptRequest | null
  resolvePrompt: (value: string | null) => void

  /** Platform check-in: auto-update + notifications. */
  updateStatus: UpdateStatus | null
  updateDialogOpen: boolean
  setUpdateDialogOpen: (open: boolean) => void
  notificationQueue: PlatformNotification[]
  dismissNotification: (id: string) => void
}

const Ctx = createContext<AppState | null>(null)

export function useApp(): AppState {
  const v = useContext(Ctx)
  if (!v) throw new Error('useApp must be used inside <AppProvider>')
  return v
}

export function AppProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [appInfo, setAppInfo] = useState<AppInfo | null>(null)
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [sessions, setSessions] = useState<SessionInfo[]>([])
  const [connecting, setConnecting] = useState<Set<string>>(new Set())
  const [tabs, dispatchTabs] = useReducer(tabsReducer, { tabs: [], activeId: null }, (s) =>
    tabsReducer(s, { type: 'open', tab: { kind: 'welcome' } }),
  )
  const [toasts, setToasts] = useState<Toast[]>([])
  const [confirmRequest, setConfirmRequest] = useState<ConfirmRequest | null>(null)
  const [connectionManagerOpen, setConnectionManagerOpen] = useState(false)
  const [connectionManagerSelect, setConnectionManagerSelect] = useState<string | 'new' | undefined>()
  const openConnectionManager = useCallback((open: boolean, select?: string | 'new') => {
    setConnectionManagerSelect(select)
    setConnectionManagerOpen(open)
  }, [])
  const [dirtyTabs, setDirtyTabs] = useState<Record<string, string>>({})
  const dirtyRef = useRef(dirtyTabs)
  dirtyRef.current = dirtyTabs
  const tabsRef = useRef<TabsState>({ tabs: [], activeId: null })
  const [shortcutsOpen, setShortcutsOpen] = useState(false)
  const [aboutOpen, setAboutOpen] = useState(false)
  const [connectionLog, setConnectionLog] = useState<ConnectionLog | null>(null)
  const closeConnectionLog = useCallback(() => setConnectionLog(null), [])
  // Steps arrive as events while the call runs; keep them in first-seen order.
  useEffect(
    () =>
      onConnectionStep(({ opId, step }) =>
        setConnectionLog((log) => {
          if (!log || log.opId !== opId) return log
          const i = log.steps.findIndex((s) => s.id === step.id)
          const steps = i < 0 ? [...log.steps, step] : log.steps.map((s, j) => (j === i ? step : s))
          return { ...log, steps }
        }),
      ),
    [],
  )
  const startLog = (kind: ConnectionLog['kind'], title: string, cancelId: string) => {
    const opId = newOpId(kind === 'test' ? 'test' : 'connect')
    setConnectionLog({ opId, kind, title, cancelId: cancelId || opId, steps: [], status: 'running', startedAt: Date.now() })
    return opId
  }
  const finishLog = (opId: string, patch: Partial<ConnectionLog>) =>
    setConnectionLog((log) => (log && log.opId === opId ? { ...log, ...patch, finishedAt: Date.now() } : log))
  const setTabDirty = useCallback((tabId: string, reason: string | null) => {
    setDirtyTabs((d) => {
      if ((d[tabId] ?? null) === reason) return d
      const n = { ...d }
      if (reason) n[tabId] = reason
      else delete n[tabId]
      return n
    })
  }, [])
  // Lets the window ask before quitting with unsaved work.
  const anyDirty = Object.keys(dirtyTabs).length > 0
  useEffect(() => {
    backend.setUnsavedWork?.(anyDirty)?.catch?.(() => undefined)
  }, [anyDirty])
  const [jobMap, setJobMap] = useState<Record<string, Job>>({})
  const jobMapRef = useRef<Record<string, Job>>({})
  const [promptRequest, setPromptRequest] = useState<PromptRequest | null>(null)
  const toastSeq = useRef(0)
  tabsRef.current = tabs

  const dismissToast = useCallback((id: number) => setToasts((ts) => ts.filter((t) => t.id !== id)), [])

  const toast = useCallback(
    (t: Omit<Toast, 'id'>) => {
      toastSeq.current += 1
      const id = toastSeq.current
      setToasts((ts) => [...ts.slice(-4), { ...t, id }])
      window.setTimeout(() => dismissToast(id), t.kind === 'error' ? 9000 : 4000)
    },
    [dismissToast],
  )

  const toastError = useCallback(
    (err: unknown, title?: string) => {
      const e = toAppError(err)
      if (e.code !== 'CANCELED') toast({ kind: 'error', title: title ?? errorTitle(e), message: e.message })
      return e
    },
    [toast],
  )

  const reloadProfilesRef = useRef<(() => void) | undefined>(undefined)
  const profilesRef = useRef<Profile[]>([])
  profilesRef.current = profiles
  const reloadProfiles = useCallback(async () => {
    try {
      setProfiles(await backend.listProfiles())
    } catch (err) {
      toastError(err, 'Could not load connections')
    }
  }, [toastError])

  reloadProfilesRef.current = () => void reloadProfiles()
  const reorderProfiles = useCallback(
    async (ids: string[]) => {
      // Show the new order at once; the server's answer replaces it.
      setProfiles((list) => [...list].sort((a, b) => ids.indexOf(a.id) - ids.indexOf(b.id)))
      try {
        setProfiles(await backend.reorderProfiles(ids))
      } catch (err) {
        toastError(err, 'Could not save the order')
        void reloadProfiles()
      }
    },
    [toastError, reloadProfiles],
  )
  useEffect(() => {
    backend.getSettings().then(setSettings).catch((err) => toastError(err, 'Could not load settings'))
    backend.getAppInfo().then(setAppInfo).catch(() => undefined)
    backend.listConnections().then(setSessions).catch(() => undefined)
    reloadProfiles()
  }, [reloadProfiles, toastError])

  // Platform check-in: current status, then live updates as the backend
  // checks for updates / sends notifications (initial check ~5s after the
  // backend starts, then every 10 minutes while the app is open).
  const [updateStatus, setUpdateStatus] = useState<UpdateStatus | null>(null)
  const [updateDialogOpen, setUpdateDialogOpen] = useState(false)
  const wasReadyToInstall = useRef(false)
  const [notificationQueue, setNotificationQueue] = useState<PlatformNotification[]>([])
  const seenNotificationIds = useRef<Set<string>>(new Set())
  const queueNotification = useCallback((n: PlatformNotification) => {
    if (seenNotificationIds.current.has(n.id)) return
    seenNotificationIds.current.add(n.id)
    setNotificationQueue((q) => [...q, n])
  }, [])

  useEffect(() => {
    backend.getUpdateStatus().then(setUpdateStatus).catch(() => undefined)
    backend.fetchNotifications().then((list) => list.forEach(queueNotification)).catch(() => undefined)
    return onUpdateStatus(setUpdateStatus)
  }, [queueNotification])
  useEffect(() => onNewNotification(queueNotification), [queueNotification])

  // Auto-open the dialog the moment a download finishes, so the user doesn't
  // have to notice a status-bar badge to know an update is ready.
  useEffect(() => {
    const isReady = updateStatus?.state === 'ready_to_install'
    if (isReady && !wasReadyToInstall.current) setUpdateDialogOpen(true)
    wasReadyToInstall.current = isReady
  }, [updateStatus])

  const dismissNotification = useCallback((id: string) => {
    setNotificationQueue((q) => q.filter((n) => n.id !== id))
    backend.ackNotification(id).catch(() => undefined)
  }, [])

  // Background jobs: initial list plus live updates pushed by the backend.
  const refreshJobs = useCallback(() => {
    backend
      .listJobs()
      .then((list) => {
        jobMapRef.current = Object.fromEntries(list.map((j) => [j.id, j]))
        setJobMap(jobMapRef.current)
      })
      .catch(() => undefined)
  }, [])
  useEffect(() => {
    refreshJobs()
    return onJobUpdate((j) => {
      const prev = jobMapRef.current[j.id]
      if (prev && prev.state !== j.state) {
        if (j.state === 'succeeded') toast({ kind: 'success', title: j.title, message: j.result })
        else if (j.state === 'failed') toast({ kind: 'error', title: `${j.title} failed`, message: j.error })
      }
      jobMapRef.current = { ...jobMapRef.current, [j.id]: j }
      setJobMap(jobMapRef.current)
    })
  }, [refreshJobs, toast])
  const jobs = useMemo(() => Object.values(jobMap).sort((a, b) => b.createdAt.localeCompare(a.createdAt)), [jobMap])

  const prompt = useCallback(
    (o: PromptOptions) => new Promise<string | null>((resolve) => setPromptRequest({ ...o, resolve })),
    [],
  )
  const resolvePrompt = useCallback(
    (value: string | null) => {
      promptRequest?.resolve(value)
      setPromptRequest(null)
    },
    [promptRequest],
  )

  // The keymap is applied while rendering, before any child renders its
  // shortcut hints (an effect would run after them).
  setKeymapOverrides(settings?.keymap)

  // Apply theme and editor font size.
  useEffect(() => {
    if (!settings) return
    const root = document.documentElement
    root.dataset.theme = settings.theme
    root.style.setProperty('--editor-font-size', `${settings.editorFontSize}px`)
  }, [settings])

  const saveSettings = useCallback(
    async (s: Settings) => {
      const saved = await backend.saveSettings(s)
      setSettings(saved)
    },
    [],
  )

  const connect = useCallback(
    async (profileId: string) => {
      setConnecting((c) => new Set(c).add(profileId))
      const name = profilesRef.current.find((p) => p.id === profileId)?.name ?? 'connection'
      const opId = startLog('connect', `Connecting to ${name}`, `connect:${profileId}`)
      try {
        const info = await backend.connectWithLog(opId, profileId)
        finishLog(opId, { status: 'ok', summary: `Connected to ${info.name} · MongoDB ${info.serverVersion} · ${info.topology}` })
        setSessions((ss) => [...ss.filter((s) => s.id !== info.id), info].sort((a, b) => a.name.localeCompare(b.name)))
        // Each connection gets its own workspace tab (Welcome stays open).
        dispatchTabs({ type: 'open', tab: { kind: 'connection', connId: info.id, connName: info.name } })
        reloadProfilesRef.current?.()
        toast({ kind: 'success', title: `Connected to ${info.name}`, message: `MongoDB ${info.serverVersion} · ${info.topology}` })
        return info
      } catch (err) {
        const e = toAppError(err)
        finishLog(opId, { status: 'failed', error: e })
        return null
      } finally {
        setConnecting((c) => {
          const n = new Set(c)
          n.delete(profileId)
          return n
        })
      }
    },
    [toast, toastError],
  )

  const confirm = useCallback(
    (o: ConfirmOptions) => new Promise<boolean>((resolve) => setConfirmRequest({ ...o, resolve })),
    [],
  )

  const testConnection = useCallback(async (form: ProfileInput) => {
    const opId = startLog('test', `Testing ${form.name || 'the connection'}`, '')
    try {
      const res = await backend.testConnectionWithLog(opId, form)
      finishLog(opId, { status: 'ok', summary: `Connection OK · MongoDB ${res.serverVersion} · ${res.topology} · ${res.latencyMs} ms${res.via ? ` · via ${res.via}` : ''}` })
      return res
    } catch (err) {
      finishLog(opId, { status: 'failed', error: toAppError(err) })
      throw err
    }
  }, [])

  /** Asks before discarding the unsaved work of the given tabs. */
  const confirmDiscard = useCallback(
    async (tabIds: string[]) => {
      const dirty = tabIds.filter((id) => dirtyRef.current[id])
      if (dirty.length === 0) return true
      const titles = dirty.map((id) => tabsRef.current.tabs.find((t) => t.id === id)?.title ?? id)
      return confirm({
        title: dirty.length === 1 ? `Discard changes in “${titles[0]}”?` : `Discard changes in ${dirty.length} tabs?`,
        message: (
          <ul className="plain-list">
            {dirty.map((id, i) => (
              <li key={id}>
                <strong>{titles[i]}</strong>: {dirtyRef.current[id]}
              </li>
            ))}
          </ul>
        ),
        confirmLabel: 'Discard',
        danger: true,
      })
    },
    [confirm],
  )

  const closeTab = useCallback(
    async (tabId: string, force = false) => {
      if (!force && !(await confirmDiscard([tabId]))) return false
      setTabDirty(tabId, null)
      dispatchTabs({ type: 'close', id: tabId })
      return true
    },
    [confirmDiscard, setTabDirty],
  )

  const closeOtherTabs = useCallback(
    async (tabId: string) => {
      const others = tabsRef.current.tabs.filter((t) => t.id !== tabId && t.kind !== 'welcome').map((t) => t.id)
      if (!(await confirmDiscard(others))) return
      for (const id of others) {
        setTabDirty(id, null)
        dispatchTabs({ type: 'close', id })
      }
    },
    [confirmDiscard, setTabDirty],
  )

  const disconnect = useCallback(
    async (connId: string) => {
      const affected = tabsRef.current.tabs.filter((t) => t.connId === connId).map((t) => t.id)
      if (!(await confirmDiscard(affected))) return
      try {
        await backend.disconnect(connId)
      } catch (err) {
        toastError(err, 'Disconnect failed')
      }
      setSessions((ss) => ss.filter((s) => s.id !== connId))
      for (const id of affected) setTabDirty(id, null)
      dispatchTabs({ type: 'closeConnection', connId })
    },
    [toastError, confirmDiscard, setTabDirty],
  )
  const resolveConfirm = useCallback(
    (ok: boolean) => {
      confirmRequest?.resolve(ok)
      setConfirmRequest(null)
    },
    [confirmRequest],
  )

  const value = useMemo<AppState>(
    () => ({
      settings,
      saveSettings,
      appInfo,
      profiles,
      reloadProfiles,
      reorderProfiles,
      sessions,
      connect,
      disconnect,
      connecting,
      tabs,
      dispatchTabs,
      toast,
      toastError,
      toasts,
      dismissToast,
      confirm,
      confirmRequest,
      resolveConfirm,
      connectionManagerOpen,
      openConnectionManager,
      connectionManagerSelect,
      dirtyTabs,
      setTabDirty,
      closeTab,
      closeOtherTabs,
      shortcutsOpen,
      setShortcutsOpen,
      aboutOpen,
      setAboutOpen,
      connectionLog,
      closeConnectionLog,
      testConnection,
      jobs,
      refreshJobs,
      prompt,
      promptRequest,
      resolvePrompt,
      updateStatus,
      updateDialogOpen,
      setUpdateDialogOpen,
      notificationQueue,
      dismissNotification,
    }),
    [
      settings, saveSettings, appInfo, profiles, reloadProfiles, reorderProfiles, sessions, connect, disconnect, connecting,
      tabs, toast, toastError, toasts, dismissToast, confirm, confirmRequest, resolveConfirm, connectionManagerOpen,
      openConnectionManager, connectionManagerSelect, dirtyTabs, setTabDirty, closeTab, closeOtherTabs, shortcutsOpen, aboutOpen,
      connectionLog, closeConnectionLog, testConnection,
      jobs, refreshJobs, prompt, promptRequest, resolvePrompt,
      updateStatus, updateDialogOpen, notificationQueue, dismissNotification,
    ],
  )

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

/** Reports a tab's unsaved work so closing it (or quitting) asks first. */
export function useTabDirty(tabId: string, reason: string | null | false | undefined) {
  const { setTabDirty } = useApp()
  const value = reason || null
  useEffect(() => {
    setTabDirty(tabId, value)
  }, [tabId, value, setTabDirty])
  useEffect(() => () => setTabDirty(tabId, null), [tabId, setTabDirty])
}
