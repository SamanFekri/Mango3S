import { TestDataDialog } from './TestDataDialog'
import { AppBuddy } from './AppBuddy'
import { useCallback, useEffect, useRef, useState } from 'react'
import type { KeyboardEvent as ReactKeyboardEvent, MouseEvent as ReactMouseEvent } from 'react'
import { backend } from '../lib/backend'
import { formatBytes } from '../lib/format'
import type { CollectionInfo, DatabaseInfo, SessionInfo } from '../lib/types'
import { useApp } from '../state/AppContext'
import { SHORTCUT_BY_ID, formatBinding, keysFor, shortcutHint } from '../lib/shortcuts'
import { moveId } from '../lib/tabs'
import { getShortcutPlatform, useShortcut } from '../state/shortcuts'
import { Spinner } from './common'
import { CollectionIcon, DatabaseIcon, FolderIcon, ServerIcon } from './icons'
import { BackupDialog, RestoreDialog } from './BackupDialogs'
import { IndexAdvisorDialog, ProfilerDialog, SqlExportDialog } from './WorkflowDialogs'
import { CollectionInfoDialog, CopyDialog, CreateCollectionDialog, DatabaseInfoDialog, RenameCollectionDialog, RenameDatabaseDialog } from './ManageDialogs'
import { ExportWizard } from './ExportWizard'
import { ImportWizard } from './ImportWizard'

interface MenuItem {
  label: string
  action: () => void
  danger?: boolean
  disabled?: boolean
  /** Shortcut id whose binding is shown next to the item (same command). */
  shortcut?: string
}
interface MenuState {
  x: number
  y: number
  items: MenuItem[]
}

type Loadable<T> = { loading: boolean; error?: string; data?: T }

type GroupKind = 'collections' | 'views' | 'system' | 'gridfs'
/** Folders under each database. */
const GROUPS: { kind: GroupKind; label: string; match: (c: CollectionInfo) => boolean }[] = [
  { kind: 'collections', label: 'Collections', match: (c) => !c.system && c.type !== 'view' },
  { kind: 'views', label: 'Views', match: (c) => !c.system && c.type === 'view' },
  { kind: 'system', label: 'System', match: (c) => c.system },
]

/** GridFS buckets: "<name>.files" collections with a matching "<name>.chunks". */
export function gridfsBuckets(colls: CollectionInfo[]): string[] {
  const names = new Set(colls.map((c) => c.name))
  return colls
    .map((c) => c.name)
    .filter((n) => n.endsWith('.files') && n.length > 6 && names.has(`${n.slice(0, -6)}.chunks`))
    .map((n) => n.slice(0, -6))
    .sort()
}

export function Sidebar() {
  const { sessions, profiles, reorderProfiles, connect, connecting, disconnect, dispatchTabs, openConnectionManager, confirm, toast, toastError } =
    useApp()
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [dbs, setDbs] = useState<Record<string, Loadable<DatabaseInfo[]>>>({})
  const [colls, setColls] = useState<Record<string, Loadable<CollectionInfo[]>>>({})
  const [menu, setMenu] = useState<MenuState | null>(null)
  const [filter, setFilter] = useState('')
  const [createFor, setCreateFor] = useState<{ conn: SessionInfo; db?: string } | null>(null)
  const [transfer, setTransfer] = useState<{ kind: 'import' | 'export'; conn: SessionInfo; db: string; coll?: string } | null>(null)
  const [manage, setManage] = useState<{
    kind: 'info' | 'copy' | 'rename' | 'profiler' | 'sql' | 'advisor' | 'datagen'
    conn: SessionInfo
    db: string
    coll?: string
    tab?: 'overview' | 'indexes' | 'validation'
  } | null>(null)
  const [backupFor, setBackupFor] = useState<{ kind: 'backup' | 'restore'; conn: SessionInfo; db?: string; coll?: string } | null>(null)
  const [closedGroups, setClosedGroups] = useState<Set<string>>(new Set())
  const [focused, setFocused] = useState<string | null>(null)
  const navRef = useRef<HTMLElement | null>(null)
  const treeRef = useRef<HTMLDivElement | null>(null)
  const filterRef = useRef<HTMLInputElement | null>(null)

  const loadDbs = useCallback(async (connId: string) => {
    setDbs((d) => ({ ...d, [connId]: { ...d[connId], loading: true, error: undefined } }))
    try {
      const data = await backend.listDatabases(connId)
      setDbs((d) => ({ ...d, [connId]: { loading: false, data } }))
    } catch (err) {
      const e = err as { message?: string }
      setDbs((d) => ({ ...d, [connId]: { loading: false, error: e.message ?? String(err) } }))
    }
  }, [])

  const loadColls = useCallback(async (connId: string, db: string) => {
    const k = `${connId}/${db}`
    setColls((c) => ({ ...c, [k]: { ...c[k], loading: true, error: undefined } }))
    try {
      const data = await backend.listCollections(connId, db)
      setColls((c) => ({ ...c, [k]: { loading: false, data } }))
    } catch (err) {
      const e = err as { message?: string }
      setColls((c) => ({ ...c, [k]: { loading: false, error: e.message ?? String(err) } }))
    }
  }, [])

  // Auto-expand newly opened connections; forget cached trees of closed ones
  // so a reconnect reloads fresh data.
  useEffect(() => {
    const live = new Set(sessions.map((s) => s.id))
    const stale = Object.keys(dbs).filter((id) => !live.has(id))
    if (stale.length > 0) {
      setDbs((d) => Object.fromEntries(Object.entries(d).filter(([id]) => live.has(id))))
      setColls((c) => Object.fromEntries(Object.entries(c).filter(([k]) => live.has(k.split('/')[0]))))
      return
    }
    for (const s of sessions) {
      if (!dbs[s.id]) {
        setExpanded((e) => new Set(e).add(s.id))
        loadDbs(s.id)
      }
    }
  }, [sessions, dbs, loadDbs])

  useEffect(() => {
    if (!menu) return
    const close = () => setMenu(null)
    window.addEventListener('click', close)
    window.addEventListener('blur', close)
    return () => {
      window.removeEventListener('click', close)
      window.removeEventListener('blur', close)
    }
  }, [menu])

  const toggle = (key: string, onOpen?: () => void) => {
    setExpanded((e) => {
      const n = new Set(e)
      if (n.has(key)) n.delete(key)
      else {
        n.add(key)
        onOpen?.()
      }
      return n
    })
  }

  const openMenu = (e: ReactMouseEvent, items: MenuItem[]) => {
    e.preventDefault()
    e.stopPropagation()
    setMenu({ x: e.clientX, y: e.clientY, items })
  }

  const openCollection = (s: SessionInfo, db: string, coll: string) =>
    dispatchTabs({ type: 'open', tab: { kind: 'collection', connId: s.id, connName: s.name, db, coll }, reuse: true })

  const dropDb = async (s: SessionInfo, db: string) => {
    const ok = await confirm({
      title: 'Drop database',
      message: (
        <>
          Permanently drop database <code>{db}</code> on <strong>{s.name}</strong> and all of its collections? This cannot be undone.
        </>
      ),
      confirmLabel: 'Drop database',
      danger: true,
      typeToConfirm: db,
    })
    if (!ok) return
    try {
      await backend.dropDatabase(s.id, db)
      toast({ kind: 'success', title: `Dropped database ${db}` })
      loadDbs(s.id)
    } catch (err) {
      toastError(err, 'Drop failed')
    }
  }

  const clearColl = async (s: SessionInfo, db: string, coll: string) => {
    const ns = `${db}.${coll}`
    const ok = await confirm({
      title: 'Clear collection',
      message: (
        <>
          Delete every document in <code>{ns}</code> on <strong>{s.name}</strong>? The collection, its indexes and validation rules are kept. This cannot be undone; back the
          collection up first if you may need the data.
        </>
      ),
      confirmLabel: 'Delete all documents',
      danger: true,
      typeToConfirm: ns,
    })
    if (!ok) return
    try {
      const n = await backend.clearCollection(s.id, db, coll, ns)
      toast({ kind: 'success', title: `Deleted ${n} document${n === 1 ? '' : 's'} from ${ns}` })
    } catch (err) {
      toastError(err, 'Clear failed')
    }
  }

  const dropColl = async (s: SessionInfo, db: string, coll: string) => {
    const ok = await confirm({
      title: 'Drop collection',
      message: (
        <>
          Permanently drop <code>
            {db}.{coll}
          </code>{' '}
          on <strong>{s.name}</strong>, including all documents and indexes?
        </>
      ),
      confirmLabel: 'Drop collection',
      danger: true,
      typeToConfirm: coll,
    })
    if (!ok) return
    try {
      await backend.dropCollection(s.id, db, coll)
      toast({ kind: 'success', title: `Dropped ${db}.${coll}` })
      loadColls(s.id, db)
    } catch (err) {
      toastError(err, 'Drop failed')
    }
  }

  // Connections follow the user's order (drag to reorder, or Alt+↑/↓).
  const profileIds = profiles.map((p) => p.id)
  const orderOf = (id: string) => {
    const i = profileIds.indexOf(id)
    return i < 0 ? Number.MAX_SAFE_INTEGER : i
  }
  const orderedSessions = [...sessions].sort((a, b) => orderOf(a.id) - orderOf(b.id))
  // Alt+1–9 connects to the Nth connection in this order.
  const quickKey = (id: string) => {
    const platform = getShortcutPlatform()
    const k = keysFor(SHORTCUT_BY_ID['conn.quick'], platform)[orderOf(id)]
    return k ? ` (${formatBinding(k, platform)})` : ''
  }
  const disconnected = profiles.filter((p) => !sessions.some((s) => s.id === p.id))
  const [drag, setDrag] = useState<{ id: string; over?: string; before?: boolean } | null>(null)
  const dragProps = (id: string) => ({
    draggable: true,
    onDragStart: (e: React.DragEvent) => {
      e.dataTransfer.effectAllowed = 'move'
      e.dataTransfer.setData('text/mango3s-connection', id)
      setDrag({ id })
    },
    onDragOver: (e: React.DragEvent) => {
      if (!drag || drag.id === id) return
      e.preventDefault()
      e.dataTransfer.dropEffect = 'move'
      const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
      const before = e.clientY < r.top + r.height / 2
      if (drag.over !== id || drag.before !== before) setDrag({ ...drag, over: id, before })
    },
    onDrop: (e: React.DragEvent) => {
      e.preventDefault()
      if (drag && drag.id !== id) void reorderProfiles(moveId(profileIds, drag.id, id, drag.over === id ? !!drag.before : true))
      setDrag(null)
    },
    onDragEnd: () => setDrag(null),
  })
  const dropClass = (id: string) => (drag?.over === id ? (drag.before ? ' drop-before' : ' drop-after') : '') + (drag?.id === id ? ' dragging' : '')
  /** Alt+↑/↓: move a connection one place in the user's order. */
  const nudge = (id: string, delta: number) => {
    const i = profileIds.indexOf(id)
    const j = i + delta
    if (i < 0 || j < 0 || j >= profileIds.length) return
    void reorderProfiles(moveId(profileIds, id, profileIds[j], delta < 0))
  }
  const f = filter.trim().toLowerCase()

  // ---- menus -----------------------------------------------------------------
  const connMenu = (s: SessionInfo): MenuItem[] => [
    { label: 'Open workspace', action: () => dispatchTabs({ type: 'open', tab: { kind: 'connection', connId: s.id, connName: s.name } }) },
    { label: 'Refresh', action: () => loadDbs(s.id), shortcut: 'view.refresh' },
    { label: 'Open shell', action: () => dispatchTabs({ type: 'open', tab: { kind: 'shell', connId: s.id, connName: s.name } }), shortcut: 'tab.shell' },
    { label: 'Server monitor', action: () => dispatchTabs({ type: 'open', tab: { kind: 'monitor', connId: s.id, connName: s.name } }) },
    { label: 'Watch changes', action: () => dispatchTabs({ type: 'open', tab: { kind: 'changes', connId: s.id, connName: s.name }, reuse: true }) },
    { label: 'SQL query', action: () => dispatchTabs({ type: 'open', tab: { kind: 'sql', connId: s.id, connName: s.name, db: 'test' } }), shortcut: 'tab.sql' },
    { label: 'Create database…', action: () => setCreateFor({ conn: s }), disabled: s.readOnly },
    { label: 'Migrate from SQL…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'migrate', connId: s.id, connName: s.name, db: '' } }), disabled: s.readOnly },
    { label: 'Compare / sync…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'compare', connId: s.id, connName: s.name } }) },
    { label: 'Back up…', action: () => setBackupFor({ kind: 'backup', conn: s }) },
    { label: 'Restore…', action: () => setBackupFor({ kind: 'restore', conn: s }), disabled: s.readOnly },
    { label: 'Edit connection…', action: () => openConnectionManager(true, s.id) },
    {
      label: 'Copy connection string',
      action: async () => {
        try {
          await navigator.clipboard.writeText(await backend.connectionURI(s.id, false))
          toast({ kind: 'success', title: 'Connection string copied', message: 'The password is not included.' })
        } catch (err) {
          toastError(err, 'Copy failed')
        }
      },
    },
    { label: 'Disconnect', action: () => disconnect(s.id), shortcut: 'conn.disconnect' },
  ]
  const dbMenu = (s: SessionInfo, db: string): MenuItem[] => [
    { label: 'Refresh', action: () => loadColls(s.id, db), shortcut: 'view.refresh' },
    { label: 'Create collection or view…', action: () => setCreateFor({ conn: s, db }), disabled: s.readOnly },
    { label: 'Database statistics…', action: () => setManage({ kind: 'info', conn: s, db }) },
    { label: 'Query profiler…', action: () => setManage({ kind: 'profiler', conn: s, db }) },
    { label: 'Index advisor…', action: () => setManage({ kind: 'advisor', conn: s, db }) },
    { label: 'Watch changes', action: () => dispatchTabs({ type: 'open', tab: { kind: 'changes', connId: s.id, connName: s.name, db }, reuse: true }) },
    { label: 'GridFS buckets…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'gridfs', connId: s.id, connName: s.name, db }, reuse: true }) },
    { label: 'Copy database…', action: () => setManage({ kind: 'copy', conn: s, db }) },
    { label: 'Rename database…', action: () => setManage({ kind: 'rename', conn: s, db }), disabled: s.readOnly },
    { label: 'Open shell here', action: () => dispatchTabs({ type: 'open', tab: { kind: 'shell', connId: s.id, connName: s.name, db } }), shortcut: 'tab.shell' },
    { label: 'SQL query', action: () => dispatchTabs({ type: 'open', tab: { kind: 'sql', connId: s.id, connName: s.name, db } }), shortcut: 'tab.sql' },
    { label: 'Import into new collection…', action: () => setTransfer({ kind: 'import', conn: s, db }), disabled: s.readOnly },
    { label: 'Back up database…', action: () => setBackupFor({ kind: 'backup', conn: s, db }) },
    { label: 'Restore into database…', action: () => setBackupFor({ kind: 'restore', conn: s, db }), disabled: s.readOnly },
    { label: 'Compare database…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'compare', connId: s.id, connName: s.name, db } }) },
    { label: 'Migrate from SQL into…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'migrate', connId: s.id, connName: s.name, db } }), disabled: s.readOnly },
    { label: 'Drop database…', action: () => dropDb(s, db), danger: true, disabled: s.readOnly, shortcut: 'explorer.drop' },
  ]
  const collMenu = (s: SessionInfo, db: string, coll: CollectionInfo): MenuItem[] => {
    const t = { connId: s.id, connName: s.name, db, coll: coll.name }
    const view = coll.type === 'view'
    return [
      { label: 'Open', action: () => openCollection(s, db, coll.name), shortcut: 'explorer.open' },
      { label: 'Open in new tab', action: () => dispatchTabs({ type: 'open', tab: { kind: 'collection', ...t } }) },
      { label: 'Aggregation editor', action: () => dispatchTabs({ type: 'open', tab: { kind: 'aggregation', ...t } }) },
      { label: 'Schema', action: () => dispatchTabs({ type: 'open', tab: { kind: 'schema', ...t }, reuse: true }) },
      { label: 'Watch changes', action: () => dispatchTabs({ type: 'open', tab: { kind: 'changes', ...t }, reuse: true }) },
      { label: 'Export…', action: () => setTransfer({ kind: 'export', conn: s, db, coll: coll.name }) },
      { label: 'Export to SQL…', action: () => setManage({ kind: 'sql', conn: s, db, coll: coll.name }) },
      { label: 'Back up collection…', action: () => setBackupFor({ kind: 'backup', conn: s, db, coll: coll.name }) },
      { label: 'Compare / sync…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'compare', ...t } }) },
      { label: 'Reschema…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'reschema', ...t } }), disabled: view },
      { label: 'Mask data…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'mask', ...t } }), disabled: view },
      { label: 'Import…', action: () => setTransfer({ kind: 'import', conn: s, db, coll: coll.name }), disabled: s.readOnly || view },
      { label: 'Generate test data…', action: () => setManage({ kind: 'datagen', conn: s, db, coll: coll.name }), disabled: s.readOnly || view },
      { label: 'Collection info…', action: () => setManage({ kind: 'info', conn: s, db, coll: coll.name }) },
      { label: 'Indexes…', action: () => setManage({ kind: 'info', conn: s, db, coll: coll.name, tab: 'indexes' }), disabled: view },
      { label: 'Index advisor…', action: () => setManage({ kind: 'advisor', conn: s, db, coll: coll.name }), disabled: view },
      { label: 'Validation rules…', action: () => setManage({ kind: 'info', conn: s, db, coll: coll.name, tab: 'validation' }), disabled: view },
      { label: 'Copy collection…', action: () => setManage({ kind: 'copy', conn: s, db, coll: coll.name }), disabled: view },
      { label: 'Rename…', action: () => setManage({ kind: 'rename', conn: s, db, coll: coll.name }), disabled: s.readOnly || view },
      { label: 'Clear (delete all documents)…', action: () => clearColl(s, db, coll.name), danger: true, disabled: s.readOnly || view },
      { label: 'Drop collection…', action: () => dropColl(s, db, coll.name), danger: true, disabled: s.readOnly, shortcut: 'explorer.drop' },
    ]
  }

  // ---- the visible tree as rows (also drives keyboard navigation) -------------
  type Row =
    | { key: string; level: number; kind: 'conn'; s: SessionInfo }
    | { key: string; level: number; kind: 'db'; s: SessionInfo; db: DatabaseInfo }
    | { key: string; level: number; kind: 'group'; s: SessionInfo; db: string; group: GroupKind; count: number }
    | { key: string; level: number; kind: 'coll'; s: SessionInfo; db: string; coll: CollectionInfo }
    | { key: string; level: number; kind: 'bucket'; s: SessionInfo; db: string; bucket: string }
    | { key: string; level: number; kind: 'note'; text: string; error?: boolean; loading?: boolean }
  const rows: Row[] = []
  for (const s of orderedSessions) {
    const ck = s.id
    rows.push({ key: `c:${ck}`, level: 0, kind: 'conn', s })
    if (!expanded.has(ck)) continue
    const d = dbs[s.id]
    if (d?.error) rows.push({ key: `e:${ck}`, level: 1, kind: 'note', text: d.error, error: true })
    for (const db of d?.data ?? []) {
      const dk = `${s.id}/${db.name}`
      const c = colls[dk]
      const visible = f ? c?.data?.filter((x) => x.name.toLowerCase().includes(f)) : c?.data
      if (f && c?.data && !visible?.length && !db.name.toLowerCase().includes(f)) continue
      rows.push({ key: `d:${dk}`, level: 1, kind: 'db', s, db })
      if (!expanded.has(dk) && !(f && visible?.length)) continue
      if (c?.error) rows.push({ key: `e:${dk}`, level: 2, kind: 'note', text: c.error, error: true })
      if (c?.loading && !c.data) rows.push({ key: `l:${dk}`, level: 2, kind: 'note', text: 'Loading…', loading: true })
      if (c?.data && c.data.length === 0) rows.push({ key: `n:${dk}`, level: 2, kind: 'note', text: 'No collections' })
      for (const g of GROUPS) {
        const list = (visible ?? []).filter(g.match)
        if (!list.length) continue
        const gk = `${dk}/${g.kind}`
        rows.push({ key: `g:${gk}`, level: 2, kind: 'group', s, db: db.name, group: g.kind, count: list.length })
        if (!isGroupOpen(gk, g.kind) && !f) continue
        for (const coll of list) rows.push({ key: `k:${dk}/${coll.name}`, level: 3, kind: 'coll', s, db: db.name, coll })
      }
      const buckets = gridfsBuckets(c?.data ?? []).filter((b) => !f || b.toLowerCase().includes(f) || db.name.toLowerCase().includes(f))
      if (buckets.length) {
        const gk = `${dk}/gridfs`
        rows.push({ key: `g:${gk}`, level: 2, kind: 'group', s, db: db.name, group: 'gridfs', count: buckets.length })
        if (isGroupOpen(gk, 'gridfs') || f) for (const b of buckets) rows.push({ key: `b:${dk}/${b}`, level: 3, kind: 'bucket', s, db: db.name, bucket: b })
      }
    }
  }

  function isGroupOpen(gk: string, kind: GroupKind) {
    // Collections start open; Views and System start closed.
    return kind === 'collections' ? !closedGroups.has(gk) : expanded.has(gk)
  }
  const toggleGroup = (gk: string, kind: GroupKind, open?: boolean) => {
    const isOpen = isGroupOpen(gk, kind)
    const want = open ?? !isOpen
    if (want === isOpen) return
    if (kind === 'collections') {
      setClosedGroups((c) => {
        const n = new Set(c)
        if (want) n.delete(gk)
        else n.add(gk)
        return n
      })
    } else {
      setExpanded((e) => {
        const n = new Set(e)
        if (want) n.add(gk)
        else n.delete(gk)
        return n
      })
    }
  }
  const setOpenKey = (key: string, open: boolean, onOpen?: () => void) =>
    setExpanded((e) => {
      if (e.has(key) === open) return e
      const n = new Set(e)
      if (open) {
        n.add(key)
        onOpen?.()
      } else n.delete(key)
      return n
    })

  const focusedRow = rows.find((r) => r.key === focused)
  const activate = (r: Row) => {
    setFocused(r.key)
    switch (r.kind) {
      case 'conn':
        toggle(r.s.id, () => !dbs[r.s.id]?.data && loadDbs(r.s.id))
        break
      case 'db':
        toggle(`${r.s.id}/${r.db.name}`, () => !colls[`${r.s.id}/${r.db.name}`]?.data && loadColls(r.s.id, r.db.name))
        break
      case 'group':
        toggleGroup(`${r.s.id}/${r.db}/${r.group}`, r.group)
        break
      case 'coll':
        openCollection(r.s, r.db, r.coll.name)
        break
      case 'bucket':
        dispatchTabs({ type: 'open', tab: { kind: 'gridfs', connId: r.s.id, connName: r.s.name, db: r.db, coll: r.bucket }, reuse: true })
        break
    }
  }
  const expandRow = (r: Row, open: boolean) => {
    if (r.kind === 'conn') setOpenKey(r.s.id, open, () => !dbs[r.s.id]?.data && loadDbs(r.s.id))
    if (r.kind === 'db') setOpenKey(`${r.s.id}/${r.db.name}`, open, () => !colls[`${r.s.id}/${r.db.name}`]?.data && loadColls(r.s.id, r.db.name))
    if (r.kind === 'group') toggleGroup(`${r.s.id}/${r.db}/${r.group}`, r.group, open)
  }
  const isOpenRow = (r: Row) =>
    r.kind === 'conn' ? expanded.has(r.s.id) : r.kind === 'db' ? expanded.has(`${r.s.id}/${r.db.name}`) : r.kind === 'group' ? isGroupOpen(`${r.s.id}/${r.db}/${r.group}`, r.group) : false
  const nav = rows.filter((r) => r.kind !== 'note')

  const onTreeKey = (e: ReactKeyboardEvent) => {
    if (e.target !== treeRef.current) return // typing in the filter etc.
    const i = nav.findIndex((r) => r.key === focused)
    const cur = nav[i]
    if (e.altKey && (e.key === 'ArrowUp' || e.key === 'ArrowDown') && cur?.kind === 'conn') {
      e.preventDefault()
      nudge(cur.s.id, e.key === 'ArrowUp' ? -1 : 1)
      return
    }
    const go = (r: Row | undefined) => {
      if (!r) return
      setFocused(r.key)
      treeRef.current?.querySelector(`[data-key="${CSS.escape(r.key)}"]`)?.scrollIntoView?.({ block: 'nearest' })
    }
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        go(i < 0 ? nav[0] : nav[i + 1])
        break
      case 'ArrowUp':
        e.preventDefault()
        go(i < 0 ? nav[nav.length - 1] : nav[i - 1])
        break
      case 'Home':
        e.preventDefault()
        go(nav[0])
        break
      case 'End':
        e.preventDefault()
        go(nav[nav.length - 1])
        break
      case 'ArrowRight':
        if (!cur) return
        e.preventDefault()
        if (cur.kind !== 'coll' && !isOpenRow(cur)) expandRow(cur, true)
        else if (nav[i + 1]?.level > cur.level) go(nav[i + 1])
        break
      case 'ArrowLeft':
        if (!cur) return
        e.preventDefault()
        if (cur.kind !== 'coll' && isOpenRow(cur)) expandRow(cur, false)
        else go([...nav.slice(0, i)].reverse().find((r) => r.level < cur.level))
        break
      case 'Enter':
        if (!cur) return
        e.preventDefault()
        activate(cur)
        break
      case 'Delete': // Del (macOS: Fn+Delete)
        if (!cur || e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) return
        e.preventDefault()
        if (cur.kind === 'coll' && !cur.s.readOnly) void dropColl(cur.s, cur.db, cur.coll.name)
        else if (cur.kind === 'db' && !cur.s.readOnly) void dropDb(cur.s, cur.db.name)
        break
    }
  }

  useShortcut(
    'view.refresh',
    () => {
      const r = focusedRow
      if (!r || r.kind === 'note') return false
      if (r.kind === 'conn') loadDbs(r.s.id)
      else loadColls(r.s.id, r.kind === 'db' ? r.db.name : r.db)
    },
    { within: navRef },
  )
  useShortcut(
    'explorer.refreshAll',
    () => {
      for (const s of sessions) loadDbs(s.id)
      for (const k of Object.keys(colls)) {
        const [id, ...rest] = k.split('/')
        loadColls(id, rest.join('/'))
      }
    },
    { within: navRef },
  )
  // With focus in the tree these act on the selected item, like its menu.
  const focusedConn = focusedRow && focusedRow.kind !== 'note' ? focusedRow.s : undefined
  const focusedDb = focusedRow?.kind === 'db' ? focusedRow.db.name : focusedRow?.kind === 'coll' || focusedRow?.kind === 'group' ? focusedRow.db : undefined
  useShortcut('conn.disconnect', () => (focusedConn ? void disconnect(focusedConn.id) : false), { within: navRef })
  useShortcut(
    'tab.shell',
    () => (focusedConn ? dispatchTabs({ type: 'open', tab: { kind: 'shell', connId: focusedConn.id, connName: focusedConn.name, db: focusedDb } }) : false),
    { within: navRef },
  )
  useShortcut(
    'tab.sql',
    () => (focusedConn ? dispatchTabs({ type: 'open', tab: { kind: 'sql', connId: focusedConn.id, connName: focusedConn.name, db: focusedDb ?? 'test' } }) : false),
    { within: navRef },
  )
  useShortcut('explorer.filter', () => (sessions.length ? filterRef.current?.focus() : false))
  useShortcut('focus.explorer', () => {
    if (!focused && nav[0]) setFocused(nav[0].key)
    treeRef.current?.focus()
  })

  const rowProps = (r: Row) => ({
    'data-key': r.key,
    id: `tree-${r.key}`,
    role: 'treeitem',
    'aria-level': r.level + 1,
    'aria-selected': focused === r.key,
    'aria-expanded': r.kind === 'coll' || r.kind === 'note' ? undefined : isOpenRow(r),
  })

  return (
    <nav className="sidebar" aria-label="Database explorer" ref={navRef}>
      <div className="sidebar-header">
        <span className="sidebar-title">Explorer</span>
        <button className="small" onClick={() => openConnectionManager(true)} title={`Connection Manager (${shortcutHint('conn.manager')})`}>
          Connect…
        </button>
      </div>
      {sessions.length > 0 && (
        <div className="sidebar-filter">
          <input ref={filterRef} placeholder="Filter collections" value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Filter collections" />
        </div>
      )}

      <div
        className="tree"
        role="tree"
        tabIndex={0}
        ref={treeRef}
        aria-activedescendant={focused ? `tree-${focused}` : undefined}
        aria-label="Connections, databases and collections"
        onKeyDown={onTreeKey}
        onFocus={() => !focused && nav[0] && setFocused(nav[0].key)}
      >
        {rows.map((r) => {
          const cls = `tree-row level-${r.level}${focused === r.key ? ' focused' : ''}`
          if (r.kind === 'note') {
            return (
              <div key={r.key} className={`${r.error ? 'tree-error' : 'tree-empty'} level-${r.level}`}>
                {r.loading && <Spinner small />} {r.text}
              </div>
            )
          }
          if (r.kind === 'conn') {
            const s = r.s
            const open = expanded.has(s.id)
            return (
              <div
                key={r.key}
                {...rowProps(r)}
                {...dragProps(s.id)}
                className={cls + dropClass(s.id)}
                onClick={() => activate(r)}
                onDoubleClick={() => dispatchTabs({ type: 'open', tab: { kind: 'connection', connId: s.id, connName: s.name } })}
                onContextMenu={(e) => openMenu(e, connMenu(s))}
                title={`MongoDB ${s.serverVersion} · ${s.topology}${quickKey(s.id)} — double-click for the workspace`}
              >
                <span className={open ? 'caret open' : 'caret'} />
                <ServerIcon live />
                <span className="tree-label strong">{s.name}</span>
                {s.readOnly && <span className="badge">RO</span>}
                {dbs[s.id]?.loading && <Spinner small />}
                <button className="row-action" title="More actions" onClick={(e) => openMenu(e, connMenu(s))}>
                  ⋯
                </button>
              </div>
            )
          }
          if (r.kind === 'db') {
            const k = `${r.s.id}/${r.db.name}`
            return (
              <div key={r.key} {...rowProps(r)} className={cls} onClick={() => activate(r)} onContextMenu={(e) => openMenu(e, dbMenu(r.s, r.db.name))}>
                <span className={expanded.has(k) ? 'caret open' : 'caret'} />
                <DatabaseIcon />
                <span className="tree-label">{r.db.name}</span>
                <span className="tree-meta">{formatBytes(r.db.sizeOnDisk)}</span>
                {colls[k]?.loading && colls[k]?.data && <Spinner small />}
                <button className="row-action" title="More actions" onClick={(e) => openMenu(e, dbMenu(r.s, r.db.name))}>
                  ⋯
                </button>
              </div>
            )
          }
          if (r.kind === 'group') {
            const open = isGroupOpen(`${r.s.id}/${r.db}/${r.group}`, r.group) || !!f
            const label = r.group === 'gridfs' ? 'GridFS buckets' : GROUPS.find((x) => x.kind === r.group)!.label
            return (
              <div key={r.key} {...rowProps(r)} className={`${cls} tree-group`} onClick={() => activate(r)}>
                <span className={open ? 'caret open' : 'caret'} />
                <FolderIcon open={open} />
                <span className="tree-label">{label}</span>
                <span className="tree-meta">{r.count}</span>
              </div>
            )
          }
          if (r.kind === 'bucket') {
            return (
              <div key={r.key} {...rowProps(r)} className={cls} onClick={() => activate(r)} title={`${r.db} · GridFS bucket ${r.bucket}`}>
                <FolderIcon open={false} />
                <span className="tree-label">{r.bucket}</span>
                <span className="tree-meta">GridFS</span>
              </div>
            )
          }
          const c = r.coll
          return (
            <div
              key={r.key}
              {...rowProps(r)}
              className={cls}
              onClick={() => activate(r)}
              onContextMenu={(e) => openMenu(e, collMenu(r.s, r.db, c))}
              title={`${r.db}.${c.name}${c.type !== 'collection' ? ` (${c.type})` : ''}`}
            >
              <CollectionIcon kind={c.system ? 'system' : c.type === 'view' ? 'view' : c.type === 'timeseries' ? 'timeseries' : 'collection'} />
              <span className="tree-label">{c.name}</span>
              {c.type === 'timeseries' && <span className="tree-meta">time series</span>}
              <button className="row-action" title="More actions" onClick={(e) => openMenu(e, collMenu(r.s, r.db, c))}>
                ⋯
              </button>
            </div>
          )
        })}

        {disconnected.length > 0 && (
          <div className="saved-list">
            <div className="section-label">Saved connections</div>
            {disconnected.map((p) => (
              <div
                key={p.id}
                className={'tree-row level-0 saved' + dropClass(p.id)}
                onDoubleClick={() => connect(p.id)}
                title={`${p.folder ? `${p.folder} / ${p.name}` : p.name}${quickKey(p.id)} — drag to reorder`}
                {...dragProps(p.id)}
              >
                <ServerIcon />
                <span className="tree-label">
                  {p.folder && <span className="muted">{p.folder} / </span>}
                  {p.name}
                </span>
                {connecting.has(p.id) ? (
                  <Spinner small />
                ) : (
                  <button className="small ghost" onClick={() => connect(p.id)}>
                    Connect
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
        {profiles.length === 0 && (
          <div className="sidebar-empty">
            <p>No connections yet.</p>
            <button className="primary" onClick={() => openConnectionManager(true, 'new')}>
              New connection
            </button>
          </div>
        )}
      </div>

      <AppBuddy />

      {menu && (
        <ul className="context-menu" style={{ left: menu.x, top: menu.y }} role="menu">
          {menu.items.map((it) => (
            <li key={it.label}>
              <button
                role="menuitem"
                className={it.danger ? 'danger-text' : ''}
                disabled={it.disabled}
                onClick={() => {
                  setMenu(null)
                  it.action()
                }}
              >
                <span>{it.label}</span>
                {it.shortcut && shortcutHint(it.shortcut) && <kbd className="menu-kbd">{shortcutHint(it.shortcut)}</kbd>}
              </button>
            </li>
          ))}
        </ul>
      )}

      {backupFor?.kind === 'backup' && <BackupDialog conn={backupFor.conn} db={backupFor.db} coll={backupFor.coll} onClose={() => setBackupFor(null)} />}
      {backupFor?.kind === 'restore' && <RestoreDialog conn={backupFor.conn} db={backupFor.db} onClose={() => setBackupFor(null)} />}
      {transfer?.kind === 'import' && (
        <ImportWizard
          connId={transfer.conn.id}
          db={transfer.db}
          coll={transfer.coll}
          onClose={() => {
            setTransfer(null)
            loadColls(transfer.conn.id, transfer.db)
          }}
        />
      )}
      {transfer?.kind === 'export' && transfer.coll && (
        <ExportWizard
          query={{ connId: transfer.conn.id, db: transfer.db, coll: transfer.coll, filter: '{}', projection: '', sort: '', skip: 0, limit: 0, page: 0, pageSize: 50 }}
          onClose={() => setTransfer(null)}
        />
      )}

      {createFor && (
        <CreateCollectionDialog
          conn={createFor.conn}
          db={createFor.db}
          onClose={() => setCreateFor(null)}
          onCreated={async (db, coll) => {
            toast({ kind: 'success', title: `Created ${db}.${coll}` })
            setCreateFor(null)
            await loadDbs(createFor.conn.id)
            setExpanded((e) => new Set(e).add(`${createFor.conn.id}/${db}`))
            loadColls(createFor.conn.id, db)
          }}
        />
      )}
      {manage?.kind === 'info' && manage.coll && <CollectionInfoDialog conn={manage.conn} db={manage.db} coll={manage.coll} tab={manage.tab} onClose={() => setManage(null)} />}
      {manage?.kind === 'info' && !manage.coll && <DatabaseInfoDialog conn={manage.conn} db={manage.db} onClose={() => setManage(null)} />}
      {manage?.kind === 'advisor' && <IndexAdvisorDialog conn={manage.conn} db={manage.db} coll={manage.coll} onClose={() => setManage(null)} />}
      {manage?.kind === 'profiler' && <ProfilerDialog conn={manage.conn} db={manage.db} onClose={() => setManage(null)} />}
      {manage?.kind === 'sql' && manage.coll && <SqlExportDialog conn={manage.conn} db={manage.db} coll={manage.coll} onClose={() => setManage(null)} />}
      {manage?.kind === 'datagen' && manage.coll && <TestDataDialog conn={manage.conn} db={manage.db} coll={manage.coll} onClose={() => setManage(null)} />}
      {manage?.kind === 'copy' && <CopyDialog conn={manage.conn} db={manage.db} coll={manage.coll} onClose={() => setManage(null)} />}
      {manage?.kind === 'rename' && manage.coll && (
        <RenameCollectionDialog
          conn={manage.conn}
          db={manage.db}
          coll={manage.coll}
          onClose={() => setManage(null)}
          onDone={() => {
            toast({ kind: 'success', title: `Renamed ${manage.db}.${manage.coll}` })
            loadDbs(manage.conn.id)
            loadColls(manage.conn.id, manage.db)
            setManage(null)
          }}
        />
      )}
      {manage?.kind === 'rename' && !manage.coll && <RenameDatabaseDialog conn={manage.conn} db={manage.db} onClose={() => (setManage(null), loadDbs(manage.conn.id))} />}
    </nav>
  )
}
