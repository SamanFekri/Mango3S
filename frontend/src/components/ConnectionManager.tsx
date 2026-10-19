import { useEffect, useMemo, useState } from 'react'
import { backend } from '../lib/backend'
import { moveId } from '../lib/tabs'
import { toAppError, type AppError } from '../lib/errors'
import type { AdvancedConfig, Capability, CollectionHistoryEntry, Profile, ProfileInput, TestResult } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Modal, Spinner } from './common'
import { ConnectionExportDialog, ConnectionImportDialog } from './ConnectionIODialogs'
import { FolderIcon } from './icons'
import { AdvancedTab, AuthTab, HistoryTab, ProxyTab, ServerTab, ShellTab, SSHTab, TLSTab, ToolsTab, type Patch, type Setter } from './ConnectionTabs'

export const CONNECTION_TABS = [
  { id: 'server', label: 'Server' },
  { id: 'auth', label: 'Authentication' },
  { id: 'tls', label: 'SSL / TLS' },
  { id: 'ssh', label: 'SSH' },
  { id: 'proxy', label: 'Proxy' },
  { id: 'shell', label: 'Shell' },
  { id: 'tools', label: 'MongoDB Tools' },
  { id: 'advanced', label: 'Advanced' },
  { id: 'history', label: 'Collection History' },
] as const
type TabId = (typeof CONNECTION_TABS)[number]['id']

/** Error messages start with the tab to fix ("SSH: …"); this finds it. */
const ERROR_TABS: [string, TabId][] = [
  ['Server:', 'server'],
  ['Authentication', 'auth'],
  ['SSL / TLS', 'tls'],
  ['TLS', 'tls'],
  ['SSH', 'ssh'],
  ['Proxy', 'proxy'],
  ['Advanced', 'advanced'],
]

const emptyAdvanced = (): AdvancedConfig => ({
  serverSelectionTimeoutSeconds: 0, operationTimeoutSeconds: 0, heartbeatSeconds: 0, localThresholdMs: 0, appName: '',
  readPreference: '', readPreferenceTags: '', maxStalenessSeconds: 0, readConcern: '', writeConcernW: '', writeConcernJournal: '',
  maxPoolSize: 0, minPoolSize: 0, maxConnecting: 0, maxIdleTimeSeconds: 0, retryWrites: '', retryReads: '', compressors: [],
})

export function blankProfile(): ProfileInput {
  return {
    id: '', name: '', folder: '', uri: 'mongodb://localhost:27017',
    server: { mode: 'manual', type: 'standalone', hosts: [{ host: 'localhost', port: 27017 }], replicaSet: '', directConnection: false },
    username: '', password: '', clearPassword: false, authSource: '', authMechanism: '', authProperties: '',
    connectTimeoutSeconds: 10, readOnly: false,
    tls: { enabled: false, caFile: '', certKeyFile: '', allowInvalidCertificates: false, allowInvalidHostnames: false },
    ssh: { enabled: false, host: '', port: 22, username: '', authMethod: 'password', privateKeyFile: '', hostKey: '' },
    proxy: { type: 'none', host: '', port: 0, username: '' },
    shell: { defaultDb: '', initScript: '', runInitScript: false },
    tools: { dir: '', paths: {} },
    advanced: emptyAdvanced(),
    secrets: {},
  }
}

export function fromProfile(p: Profile): ProfileInput {
  const b = blankProfile()
  return {
    ...b,
    id: p.id,
    name: p.name,
    folder: p.folder ?? '',
    uri: p.uri,
    server: p.server?.mode ? { ...p.server, hosts: p.server.hosts ?? [] } : { ...b.server, mode: 'uri' },
    username: p.username ?? '',
    authSource: p.authSource ?? '',
    authMechanism: p.authMechanism ?? '',
    authProperties: p.authProperties ?? '',
    connectTimeoutSeconds: p.connectTimeoutSeconds,
    readOnly: p.readOnly,
    tls: { ...b.tls, ...p.tls },
    ssh: { ...b.ssh, ...p.ssh },
    proxy: { ...b.proxy, ...p.proxy, type: p.proxy?.type || 'none' },
    shell: { ...b.shell, ...p.shell },
    tools: { dir: p.tools?.dir ?? '', paths: p.tools?.paths ?? {} },
    advanced: { ...emptyAdvanced(), ...p.advanced, compressors: p.advanced?.compressors ?? [] },
  }
}

interface FolderNode {
  name: string
  path: string
  folders: FolderNode[]
  profiles: Profile[]
}

function folderTree(profiles: Profile[], filter: string): FolderNode {
  const root: FolderNode = { name: '', path: '', folders: [], profiles: [] }
  const f = filter.trim().toLowerCase()
  for (const p of profiles) {
    if (f && !`${p.name} ${p.folder ?? ''} ${p.uri}`.toLowerCase().includes(f)) continue
    let node = root
    for (const part of (p.folder ?? '').split('/').filter(Boolean)) {
      let child = node.folders.find((c) => c.name === part)
      if (!child) {
        child = { name: part, path: node.path ? `${node.path}/${part}` : part, folders: [], profiles: [] }
        node.folders.push(child)
        node.folders.sort((a, b) => a.name.localeCompare(b.name))
      }
      node = child
    }
    node.profiles.push(p)
  }
  return root
}

export function ConnectionManager() {
  const { profiles, reloadProfiles, reorderProfiles, openConnectionManager, connectionManagerSelect, connect, sessions, toast, confirm, disconnect, dispatchTabs, testConnection } = useApp()
  const initial = connectionManagerSelect === 'new' ? null : (profiles.find((p) => p.id === connectionManagerSelect) ?? profiles[0] ?? null)
  const [selected, setSelected] = useState<string | null>(initial?.id ?? null)
  const [form, setForm] = useState<ProfileInput>(() => (initial ? fromProfile(initial) : blankProfile()))
  const [tab, setTab] = useState<TabId>('server')
  const [busy, setBusy] = useState<'' | 'test' | 'save' | 'connect'>('')
  const [error, setError] = useState<AppError | null>(null)
  const [test, setTest] = useState<TestResult | null>(null)
  const [dirty, setDirty] = useState(false)
  const [filter, setFilter] = useState('')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const [caps, setCaps] = useState<Record<string, Capability[]>>({})

  useEffect(() => {
    backend.connectionCapabilities().then(setCaps).catch(() => undefined)
  }, [])

  const current = profiles.find((p) => p.id === selected)
  const tree = useMemo(() => folderTree(profiles, filter), [profiles, filter])
  const folders = useMemo(() => [...new Set(profiles.map((p) => p.folder).filter(Boolean) as string[])].sort(), [profiles])

  const close = async () => {
    if (dirty && !(await confirm({ title: 'Discard changes?', message: 'This connection has unsaved changes.', confirmLabel: 'Discard', danger: true }))) return
    openConnectionManager(false)
  }

  const select = async (p: Profile | null) => {
    if (dirty && !(await confirm({ title: 'Discard changes?', message: `Discard the unsaved changes to “${form.name || 'the new connection'}”?`, confirmLabel: 'Discard', danger: true }))) {
      return
    }
    setSelected(p?.id ?? null)
    setForm(p ? fromProfile(p) : blankProfile())
    setDirty(false)
    setError(null)
    setTest(null)
    setTab('server')
  }

  const set: Setter = (k, v) => {
    setForm((f) => ({ ...f, [k]: v }))
    setDirty(true)
    setTest(null)
  }
  const patch: Patch = (k, v) => {
    setForm((f) => ({ ...f, [k]: { ...(f[k] as object), ...v } }))
    setDirty(true)
    setTest(null)
  }

  const run = async (kind: 'test' | 'save' | 'connect', f: ProfileInput = form) => {
    setBusy(kind)
    setError(null)
    setTest(null)
    try {
      if (kind === 'test') {
        setTest(await testConnection(f)) // shows the step-by-step connection log
        return
      }
      let id = f.id
      if (dirty || !id) {
        const saved = await backend.saveProfile(f)
        await reloadProfiles()
        setSelected(saved.id)
        setForm(fromProfile(saved))
        setDirty(false)
        id = saved.id
        if (sessions.some((s) => s.id === saved.id)) {
          toast({ kind: 'info', title: 'Saved', message: 'Reconnect to apply the changes to the open connection.' })
        } else if (kind === 'save') {
          toast({ kind: 'success', title: `Saved “${saved.name}”` })
        }
      }
      if (kind === 'connect') {
        const info = await connect(id)
        if (info) openConnectionManager(false)
        // On failure the connection log shows which step failed.
      }
    } catch (err) {
      const e = toAppError(err)
      setError(e)
      const t = ERROR_TABS.find(([prefix]) => e.message.startsWith(prefix))
      if (t) setTab(t[1])
    } finally {
      setBusy('')
    }
  }

  const trustHostKey = () => {
    if (!error?.detail) return
    const next = { ...form, ssh: { ...form.ssh, hostKey: error.detail } }
    setForm(next)
    setDirty(true)
    void run('test', next)
  }

  const [io, setIo] = useState<'import' | 'export' | null>(null)
  const copyURI = async () => {
    if (!current) return
    try {
      await navigator.clipboard.writeText(await backend.connectionURI(current.id, false))
      toast({ kind: 'success', title: 'Connection string copied', message: 'The password is not included.' })
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const duplicate = async () => {
    if (!current) return
    try {
      const cp = await backend.duplicateProfile(current.id)
      await reloadProfiles()
      setSelected(cp.id)
      setForm(fromProfile(cp))
      setDirty(false)
      toast({ kind: 'success', title: `Duplicated as “${cp.name}”` })
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const remove = async () => {
    if (!current) return
    const ok = await confirm({
      title: 'Delete connection',
      message: `Delete “${current.name}”, its stored passwords and its collection history?`,
      confirmLabel: 'Delete',
      danger: true,
    })
    if (!ok) return
    try {
      if (sessions.some((s) => s.id === current.id)) await disconnect(current.id)
      await backend.deleteProfile(current.id)
      await reloadProfiles()
      setDirty(false)
      setSelected(null)
      setForm(blankProfile())
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const reopen = async (e: CollectionHistoryEntry) => {
    if (!current) return
    const live = sessions.find((s) => s.id === current.id) ?? (await connect(current.id))
    if (!live) return
    dispatchTabs({ type: 'open', tab: { kind: 'collection', connId: live.id, connName: live.name, db: e.db, coll: e.coll }, reuse: true })
    openConnectionManager(false)
  }

  // Drag to reorder; dropping next to a connection in another folder (or on a
  // folder) also moves it into that folder.
  const [drag, setDrag] = useState<{ id: string; over?: string; before?: boolean } | null>(null)
  const ids = profiles.map((p) => p.id)
  const moveTo = async (id: string, target: Profile | null, folder: string, before: boolean) => {
    const moving = profiles.find((p) => p.id === id)
    setDrag(null)
    if (!moving) return
    try {
      if ((moving.folder ?? '') !== folder) {
        if (id === selected && dirty) {
          toast({ kind: 'info', title: 'Save or discard your changes first', message: 'Then drag the connection to another folder.' })
          return
        }
        await backend.setProfileFolder(id, folder)
        if (id === selected) setForm((f) => ({ ...f, folder }))
      }
      if (target && target.id !== id) await reorderProfiles(moveId(ids, id, target.id, before))
      else await reloadProfiles()
    } catch (err) {
      setError(toAppError(err))
    }
  }
  const itemDrag = (p: Profile) => ({
    draggable: true,
    onDragStart: (e: React.DragEvent) => {
      e.dataTransfer.effectAllowed = 'move'
      e.dataTransfer.setData('text/mango3s-connection', p.id)
      setDrag({ id: p.id })
    },
    onDragOver: (e: React.DragEvent) => {
      if (!drag || drag.id === p.id) return
      e.preventDefault()
      const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
      const before = e.clientY < r.top + r.height / 2
      if (drag.over !== p.id || drag.before !== before) setDrag({ ...drag, over: p.id, before })
    },
    onDrop: (e: React.DragEvent) => {
      e.preventDefault()
      e.stopPropagation()
      if (drag) void moveTo(drag.id, p, p.folder ?? '', drag.over === p.id ? !!drag.before : true)
    },
    onDragEnd: () => setDrag(null),
  })
  const folderDrop = (path: string) => ({
    onDragOver: (e: React.DragEvent) => {
      if (!drag) return
      e.preventDefault()
      if (drag.over !== `folder:${path}`) setDrag({ ...drag, over: `folder:${path}` })
    },
    onDrop: (e: React.DragEvent) => {
      e.preventDefault()
      e.stopPropagation()
      if (drag) void moveTo(drag.id, null, path, false)
    },
  })
  // Dropping on empty space in the list moves a connection out of its folder.
  const listDrop = {
    onDragOver: (e: React.DragEvent) => {
      if (drag && e.target === e.currentTarget) e.preventDefault()
    },
    onDrop: (e: React.DragEvent) => {
      if (drag && e.target === e.currentTarget) {
        e.preventDefault()
        void moveTo(drag.id, null, '', false)
      }
    },
  }
  const dropClass = (key: string) => (drag?.over === key ? (key.startsWith('folder:') ? ' drop-into' : drag.before ? ' drop-before' : ' drop-after') : '')

  const renderFolder = (node: FolderNode, depth: number) => (
    <>
      {node.folders.map((f) => {
        const open = !collapsed.has(f.path)
        return (
          <li key={`f:${f.path}`}>
            <button
              type="button"
              {...folderDrop(f.path)}
              className={'conn-folder' + dropClass(`folder:${f.path}`)}
              style={{ paddingLeft: 8 + depth * 14 }}
              aria-expanded={open}
              onClick={() =>
                setCollapsed((c) => {
                  const n = new Set(c)
                  if (n.has(f.path)) n.delete(f.path)
                  else n.add(f.path)
                  return n
                })
              }
            >
              <span className={open ? 'caret open' : 'caret'} />
              <FolderIcon open={open} />
              {f.name}
            </button>
            {open && <ul>{renderFolder(f, depth + 1)}</ul>}
          </li>
        )
      })}
      {node.profiles.map((p) => (
        <li key={p.id}>
          <button
            type="button"
            {...itemDrag(p)}
            className={(p.id === selected ? 'conn-item active' : 'conn-item') + dropClass(p.id) + (drag?.id === p.id ? ' dragging' : '')}
            style={{ paddingLeft: 8 + depth * 14 }}
            onClick={() => void select(p)}
            onDoubleClick={() => void run('connect', p.id === selected ? form : fromProfile(p))}
            title={`${p.uri} — drag to reorder or into a folder`}
          >
            <span className={sessions.some((s) => s.id === p.id) ? 'dot on' : 'dot'} />
            <span className="conn-name">{p.name}</span>
            {p.ssh?.enabled && <span className="badge">SSH</span>}
            {p.readOnly && <span className="badge">RO</span>}
          </button>
        </li>
      ))}
    </>
  )

  const tabProps = { form, set, patch, stored: current, caps }
  const isNew = !form.id

  return (
    <Modal title="Connection Manager" onClose={() => void close()} wide>
      {io === 'import' && <ConnectionImportDialog onClose={() => setIo(null)} />}
      {io === 'export' && <ConnectionExportDialog selected={current ?? null} onClose={() => setIo(null)} />}
      <div className="conn-manager">
        <aside className="conn-list">
          <div className="conn-list-actions">
            <button type="button" className="primary" onClick={() => void select(null)}>
              + New
            </button>
            <button type="button" onClick={duplicate} disabled={!current} title="Duplicate the selected connection">
              Duplicate
            </button>
            <button type="button" className="ghost danger-text" onClick={remove} disabled={!current}>
              Delete
            </button>
          </div>
          <div className="conn-list-actions">
            <button type="button" className="small ghost" onClick={() => setIo('import')} title="Import from a Mango3S file, a MongoDB Compass export or a list of connection strings">
              Import…
            </button>
            <button type="button" className="small ghost" onClick={() => setIo('export')} disabled={profiles.length === 0}>
              Export…
            </button>
            <button type="button" className="small ghost" onClick={copyURI} disabled={!current} title="Copy the connection string (without the password)">
              Copy URI
            </button>
          </div>
          <input className="conn-search" placeholder="Search connections" value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Search connections" />
          <ul role="tree" aria-label="Saved connections" className="conn-tree" {...listDrop}>
            {renderFolder(tree, 0)}
            {profiles.length === 0 && <li className="muted small pad">No saved connections yet.</li>}
          </ul>
        </aside>

        <form
          className="conn-form"
          onSubmit={(e) => {
            e.preventDefault()
            void run('connect')
          }}
        >
          <h3>
            {isNew ? 'New connection' : form.name || 'Connection'}
            {dirty && <span className="badge">unsaved</span>}
          </h3>
          <div className="conn-tabs" role="tablist" aria-label="Connection settings">
            {CONNECTION_TABS.map((t) => (
              <button
                key={t.id}
                type="button"
                role="tab"
                id={`conn-tab-${t.id}`}
                aria-selected={tab === t.id}
                aria-controls="conn-tab-panel"
                className={tab === t.id ? 'active' : ''}
                onClick={() => setTab(t.id)}
              >
                {t.label}
                {t.id === 'ssh' && form.ssh.enabled && <span className="tab-dot" aria-label="enabled" />}
                {t.id === 'tls' && form.tls.enabled && <span className="tab-dot" aria-label="enabled" />}
                {t.id === 'proxy' && form.proxy.type !== 'none' && <span className="tab-dot" aria-label="enabled" />}
              </button>
            ))}
          </div>
          <div className="conn-tab-panel" role="tabpanel" id="conn-tab-panel" aria-labelledby={`conn-tab-${tab}`}>
            {tab === 'server' && <ServerTab {...tabProps} folders={folders} />}
            {tab === 'auth' && <AuthTab {...tabProps} />}
            {tab === 'tls' && <TLSTab {...tabProps} />}
            {tab === 'ssh' && <SSHTab {...tabProps} />}
            {tab === 'proxy' && <ProxyTab {...tabProps} />}
            {tab === 'shell' && <ShellTab {...tabProps} />}
            {tab === 'tools' && <ToolsTab {...tabProps} />}
            {tab === 'advanced' && <AdvancedTab {...tabProps} />}
            {tab === 'history' && <HistoryTab {...tabProps} onReopen={reopen} />}
          </div>

          <ErrorBanner error={error} onDismiss={() => setError(null)} />
          {error?.code === 'SSH_HOST_KEY' && error.detail && (
            <div className="info-banner">
              <span>
                Verify with the server administrator that the SSH host key fingerprint is <code>{error.detail}</code>.
              </span>
              <button type="button" className="small" onClick={trustHostKey}>
                Trust this key and retry
              </button>
            </div>
          )}
          {test && (
            <div className="success-banner" role="status">
              <strong>Connection OK</strong> — MongoDB {test.serverVersion} · {test.topology} · {test.latencyMs} ms{test.via ? ` · via ${test.via}` : ''}
            </div>
          )}

          <div className="form-actions">
            <button type="button" onClick={() => void run('test')} disabled={!!busy}>
              {busy === 'test' ? <Spinner small /> : null} Test connection
            </button>
            <span className="spacer" />
            <button type="button" onClick={() => void close()} disabled={!!busy}>
              Cancel
            </button>
            <button type="button" onClick={() => void run('save')} disabled={!!busy || (!dirty && !isNew)}>
              {busy === 'save' ? <Spinner small /> : null} Save
            </button>
            <button type="submit" className="primary" disabled={!!busy}>
              {busy === 'connect' ? <Spinner small /> : null} {dirty || isNew ? 'Save & connect' : 'Connect'}
            </button>
          </div>
        </form>
      </div>
    </Modal>
  )
}
