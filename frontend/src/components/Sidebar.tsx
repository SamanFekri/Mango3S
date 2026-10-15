import { useCallback, useEffect, useState } from 'react'
import type { MouseEvent as ReactMouseEvent } from 'react'
import { backend } from '../lib/backend'
import { formatBytes } from '../lib/format'
import type { CollectionInfo, DatabaseInfo, SessionInfo } from '../lib/types'
import { useApp } from '../state/AppContext'
import { Modal, Spinner } from './common'

interface MenuItem {
  label: string
  action: () => void
  danger?: boolean
  disabled?: boolean
}
interface MenuState {
  x: number
  y: number
  items: MenuItem[]
}

type Loadable<T> = { loading: boolean; error?: string; data?: T }

export function Sidebar() {
  const { sessions, profiles, connect, connecting, disconnect, dispatchTabs, openConnectionManager, confirm, toast, toastError } =
    useApp()
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [dbs, setDbs] = useState<Record<string, Loadable<DatabaseInfo[]>>>({})
  const [colls, setColls] = useState<Record<string, Loadable<CollectionInfo[]>>>({})
  const [menu, setMenu] = useState<MenuState | null>(null)
  const [filter, setFilter] = useState('')
  const [createFor, setCreateFor] = useState<{ conn: SessionInfo; db?: string } | null>(null)

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

  const disconnected = profiles.filter((p) => !sessions.some((s) => s.id === p.id))
  const f = filter.trim().toLowerCase()

  return (
    <nav className="sidebar" aria-label="Database explorer">
      <div className="sidebar-header">
        <span className="sidebar-title">Explorer</span>
        <button className="small" onClick={() => openConnectionManager(true)}>
          Connect…
        </button>
      </div>
      {sessions.length > 0 && (
        <div className="sidebar-filter">
          <input placeholder="Filter collections" value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Filter collections" />
        </div>
      )}

      <div className="tree" role="tree">
        {sessions.map((s) => {
          const open = expanded.has(s.id)
          const d = dbs[s.id]
          const connMenu: MenuItem[] = [
            { label: 'Refresh', action: () => loadDbs(s.id) },
            {
              label: 'Open shell',
              action: () => dispatchTabs({ type: 'open', tab: { kind: 'shell', connId: s.id, connName: s.name, db: 'test' } }),
            },
            { label: 'Create database…', action: () => setCreateFor({ conn: s }), disabled: s.readOnly },
            { label: 'Disconnect', action: () => disconnect(s.id) },
          ]
          return (
            <div key={s.id} role="treeitem" aria-expanded={open}>
              <div
                className="tree-row level-0"
                onClick={() => toggle(s.id, () => !d?.data && loadDbs(s.id))}
                onContextMenu={(e) =>
                  openMenu(e, connMenu)
                }
              >
                <span className={open ? 'caret open' : 'caret'} />
                <span className="dot on" />
                <span className="tree-label strong" title={`MongoDB ${s.serverVersion} · ${s.topology}`}>
                  {s.name}
                </span>
                {s.readOnly && <span className="badge">RO</span>}
                {d?.loading && <Spinner small />}
                <button
                  className="row-action"
                  title="More actions"
                  onClick={(e) =>
                    openMenu(e, connMenu)
                  }
                >
                  ⋯
                </button>
              </div>
              {open && d?.error && <div className="tree-error level-1">{d.error}</div>}
              {open &&
                d?.data?.map((db) => {
                  const k = `${s.id}/${db.name}`
                  const dbOpen = expanded.has(k)
                  const c = colls[k]
                  const visible = f ? c?.data?.filter((x) => x.name.toLowerCase().includes(f)) : c?.data
                  const dbMenu: MenuItem[] = [
                    { label: 'Refresh', action: () => loadColls(s.id, db.name) },
                    { label: 'Create collection…', action: () => setCreateFor({ conn: s, db: db.name }), disabled: s.readOnly },
                    {
                      label: 'Open shell here',
                      action: () => dispatchTabs({ type: 'open', tab: { kind: 'shell', connId: s.id, connName: s.name, db: db.name } }),
                    },
                    { label: 'Drop database…', action: () => dropDb(s, db.name), danger: true, disabled: s.readOnly },
                  ]
                  return (
                    <div key={k} role="treeitem" aria-expanded={dbOpen}>
                      <div
                        className="tree-row level-1"
                        onClick={() => toggle(k, () => !c?.data && loadColls(s.id, db.name))}
                        onContextMenu={(e) => openMenu(e, dbMenu)}
                      >
                        <span className={dbOpen ? 'caret open' : 'caret'} />
                        <span className="icon-db" aria-hidden />
                        <span className="tree-label">{db.name}</span>
                        <span className="tree-meta">{formatBytes(db.sizeOnDisk)}</span>
                        {c?.loading && <Spinner small />}
                        <button className="row-action" title="More actions" onClick={(e) => openMenu(e, dbMenu)}>
                          ⋯
                        </button>
                      </div>
                      {dbOpen && c?.error && <div className="tree-error level-2">{c.error}</div>}
                      {dbOpen && c?.data && c.data.length === 0 && <div className="tree-empty level-2">No collections</div>}
                      {dbOpen &&
                        visible?.map((coll) => {
                          const collMenu: MenuItem[] = [
                            { label: 'Open', action: () => openCollection(s, db.name, coll.name) },
                            {
                              label: 'Open in new tab',
                              action: () =>
                                dispatchTabs({
                                  type: 'open',
                                  tab: { kind: 'collection', connId: s.id, connName: s.name, db: db.name, coll: coll.name },
                                }),
                            },
                            {
                              label: 'Aggregation editor',
                              action: () =>
                                dispatchTabs({
                                  type: 'open',
                                  tab: { kind: 'aggregation', connId: s.id, connName: s.name, db: db.name, coll: coll.name },
                                }),
                            },
                            {
                              label: 'Drop collection…',
                              action: () => dropColl(s, db.name, coll.name),
                              danger: true,
                              disabled: s.readOnly || coll.type === 'view',
                            },
                          ]
                          return (
                            <div
                              key={coll.name}
                              role="treeitem"
                              className="tree-row level-2"
                              onClick={() => openCollection(s, db.name, coll.name)}
                              onContextMenu={(e) => openMenu(e, collMenu)}
                              title={`${db.name}.${coll.name}`}
                            >
                              <span className={coll.type === 'view' ? 'icon-view' : 'icon-coll'} aria-hidden />
                              <span className="tree-label">{coll.name}</span>
                              {coll.type !== 'collection' && <span className="tree-meta">{coll.type}</span>}
                              <button className="row-action" title="More actions" onClick={(e) => openMenu(e, collMenu)}>
                                ⋯
                              </button>
                            </div>
                          )
                        })}
                    </div>
                  )
                })}
            </div>
          )
        })}

        {disconnected.length > 0 && (
          <div className="saved-list">
            <div className="section-label">Saved connections</div>
            {disconnected.map((p) => (
              <div key={p.id} className="tree-row level-0 saved" onDoubleClick={() => connect(p.id)}>
                <span className="dot" />
                <span className="tree-label">{p.name}</span>
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
            <button className="primary" onClick={() => openConnectionManager(true)}>
              New connection
            </button>
          </div>
        )}
      </div>

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
                {it.label}
              </button>
            </li>
          ))}
        </ul>
      )}

      {createFor && (
        <CreateCollectionDialog
          db={createFor.db}
          onClose={() => setCreateFor(null)}
          onCreate={async (db, coll) => {
            await backend.createCollection(createFor.conn.id, db, coll)
            toast({ kind: 'success', title: `Created ${db}.${coll}` })
            setCreateFor(null)
            await loadDbs(createFor.conn.id)
            setExpanded((e) => new Set(e).add(`${createFor.conn.id}/${db}`))
            loadColls(createFor.conn.id, db)
          }}
        />
      )}
    </nav>
  )
}

function CreateCollectionDialog({
  db: fixedDb,
  onClose,
  onCreate,
}: {
  db?: string
  onClose: () => void
  onCreate: (db: string, coll: string) => Promise<void>
}) {
  const [db, setDb] = useState(fixedDb ?? '')
  const [coll, setColl] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      await onCreate(db.trim(), coll.trim())
    } catch (err) {
      setError((err as { message?: string }).message ?? String(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal
      title={fixedDb ? 'Create collection' : 'Create database'}
      onClose={onClose}
      footer={
        <>
          <button onClick={onClose}>Cancel</button>
          <button className="primary" disabled={busy || !db.trim() || !coll.trim()} onClick={submit}>
            {busy && <Spinner small />} Create
          </button>
        </>
      }
    >
      {!fixedDb && (
        <>
          <p className="muted small">MongoDB creates a database together with its first collection.</p>
          <label className="field">
            <span>Database name</span>
            <input value={db} onChange={(e) => setDb(e.target.value)} autoFocus />
          </label>
        </>
      )}
      <label className="field">
        <span>Collection name</span>
        <input
          value={coll}
          onChange={(e) => setColl(e.target.value)}
          autoFocus={!!fixedDb}
          onKeyDown={(e) => e.key === 'Enter' && submit()}
        />
      </label>
      {error && <div className="error-text">{error}</div>}
    </Modal>
  )
}
