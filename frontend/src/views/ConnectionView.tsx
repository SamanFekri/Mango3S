import { Fragment, useCallback, useEffect, useState } from 'react'
import { Spinner } from '../components/common'
import { CollectionIcon, DatabaseIcon } from '../components/icons'
import { backend } from '../lib/backend'
import { formatBytes } from '../lib/format'
import { timeAgo } from '../lib/library'
import { shortcutHint } from '../lib/shortcuts'
import type { Tab } from '../lib/tabs'
import type { CollectionHistoryEntry, CollectionInfo, DatabaseInfo } from '../lib/types'
import { useApp } from '../state/AppContext'
import { useShortcut } from '../state/shortcuts'

/** The workspace tab of one open connection. */
export function ConnectionView({ tab }: { tab: Tab }) {
  const { sessions, profiles, dispatchTabs, disconnect, openConnectionManager, toastError } = useApp()
  const connId = tab.connId!
  const session = sessions.find((s) => s.id === connId)
  const profile = profiles.find((p) => p.id === connId)
  const [dbs, setDbs] = useState<DatabaseInfo[] | null>(null)
  const [open, setOpen] = useState<Record<string, CollectionInfo[] | 'loading'>>({})
  const [recent, setRecent] = useState<CollectionHistoryEntry[]>([])

  const load = useCallback(async () => {
    try {
      setDbs(await backend.listDatabases(connId))
      setRecent(await backend.listCollectionHistory(connId))
    } catch (err) {
      toastError(err, 'Could not list databases')
      setDbs([])
    }
  }, [connId, toastError])
  useEffect(() => {
    if (session) void load()
  }, [session, load])
  useShortcut('view.refresh', () => void load())

  const toggle = async (db: string) => {
    if (open[db]) {
      setOpen((o) => {
        const n = { ...o }
        delete n[db]
        return n
      })
      return
    }
    setOpen((o) => ({ ...o, [db]: 'loading' }))
    try {
      const list = await backend.listCollections(connId, db)
      setOpen((o) => ({ ...o, [db]: list }))
    } catch (err) {
      toastError(err)
      setOpen((o) => ({ ...o, [db]: [] }))
    }
  }

  const openColl = (db: string, coll: string) =>
    dispatchTabs({ type: 'open', tab: { kind: 'collection', connId, connName: session?.name ?? tab.connName, db, coll }, reuse: true })

  if (!session) {
    return (
      <div className="view connection-view">
        <div className="empty-state">
          {tab.connName} is not connected.
          <button className="small" onClick={() => openConnectionManager(true, connId)}>
            Open in the connection manager
          </button>
        </div>
      </div>
    )
  }

  const via = [profile?.proxy?.type && profile.proxy.type !== 'none' && `${profile.proxy.type} proxy ${profile.proxy.host}`, profile?.ssh?.enabled && `SSH ${profile.ssh.username}@${profile.ssh.host}`]
    .filter(Boolean)
    .join(' → ')

  return (
    <div className="view connection-view">
      <div className="view-header">
        <div className="breadcrumbs">
          <span className="dot on" />
          <strong>{session.name}</strong>
          {session.readOnly && <span className="badge">read-only</span>}
        </div>
        <span className="spacer" />
        <button className="small ghost" onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'shell', connId, connName: session.name } })} title={`IntelliShell (${shortcutHint('tab.shell')})`}>
          IntelliShell
        </button>
        <button className="small ghost" onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'sql', connId, connName: session.name, db: 'test' } })} title={`SQL (${shortcutHint('tab.sql')})`}>
          SQL
        </button>
        <button className="small ghost" onClick={() => void load()} title={`Refresh (${shortcutHint('view.refresh')})`}>
          Refresh
        </button>
        <button className="small ghost" onClick={() => openConnectionManager(true, connId)}>
          Edit connection…
        </button>
        <button className="small" onClick={() => void disconnect(connId)} title={`Disconnect (${shortcutHint('conn.disconnect')})`}>
          Disconnect
        </button>
      </div>

      <dl className="stats conn-facts">
        <dt>Server</dt>
        <dd>
          MongoDB {session.serverVersion} · {session.topology}
        </dd>
        <dt>Hosts</dt>
        <dd className="mono">{profile ? (profile.server?.mode === 'manual' ? profile.server.hosts.map((h) => (h.port ? `${h.host}:${h.port}` : h.host)).join(', ') : profile.uri) : '—'}</dd>
        {profile?.username && (
          <>
            <dt>User</dt>
            <dd>
              {profile.username}
              {profile.authSource ? ` @ ${profile.authSource}` : ''}
              {profile.authMechanism ? ` · ${profile.authMechanism}` : ''}
            </dd>
          </>
        )}
        <dt>Network</dt>
        <dd>
          {profile?.tls?.enabled ? 'TLS' : 'No TLS'}
          {via ? ` · ${via}` : ''}
        </dd>
        <dt>Connected</dt>
        <dd>{timeAgo(session.connectedAt)}</dd>
      </dl>

      {recent.length > 0 && (
        <section className="panel">
          <h3>Recently opened</h3>
          <div className="chip-list">
            {recent.slice(0, 12).map((r) => (
              <button key={`${r.db}.${r.coll}`} className="chip" onClick={() => openColl(r.db, r.coll)} title={`Opened ${timeAgo(r.lastOpened)}`}>
                <CollectionIcon kind="collection" /> {r.db}.{r.coll}
              </button>
            ))}
          </div>
        </section>
      )}

      <section className="panel">
        <h3>Databases</h3>
        {dbs === null ? (
          <Spinner />
        ) : (
          <table className="grid compact">
            <thead>
              <tr>
                <th>Database</th>
                <th>Size on disk</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {dbs.map((d) => {
                const colls = open[d.name]
                return (
                  <Fragment key={d.name}>
                    <tr className="clickable" onClick={() => void toggle(d.name)}>
                      <td>
                        <span className={colls ? 'caret open' : 'caret'} /> <DatabaseIcon /> {d.name}
                      </td>
                      <td>{d.empty ? 'empty' : formatBytes(d.sizeOnDisk)}</td>
                      <td className="row-buttons">
                        <button
                          className="small ghost"
                          onClick={(e) => {
                            e.stopPropagation()
                            dispatchTabs({ type: 'open', tab: { kind: 'shell', connId, connName: session.name, db: d.name } })
                          }}
                        >
                          Shell
                        </button>
                      </td>
                    </tr>
                    {colls === 'loading' && (
                      <tr>
                        <td colSpan={3}>
                          <Spinner small />
                        </td>
                      </tr>
                    )}
                    {Array.isArray(colls) && (
                      <tr>
                        <td colSpan={3}>
                          <CollectionGroups colls={colls} onOpen={(c) => openColl(d.name, c)} />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                )
              })}
            </tbody>
          </table>
        )}
      </section>
    </div>
  )
}

function CollectionGroups({ colls, onOpen }: { colls: CollectionInfo[]; onOpen: (c: string) => void }) {
  const groups: [string, CollectionInfo[]][] = [
    ['Collections', colls.filter((c) => !c.system && c.type !== 'view')],
    ['Views', colls.filter((c) => !c.system && c.type === 'view')],
    ['System', colls.filter((c) => c.system)],
  ]
  return (
    <div className="coll-groups">
      {groups
        .filter(([, l]) => l.length)
        .map(([name, list]) => (
          <div key={name}>
            <div className="section-label">
              {name} ({list.length})
            </div>
            <div className="chip-list">
              {list.map((c) => (
                <button key={c.name} className="chip" onClick={() => onOpen(c.name)}>
                  <CollectionIcon kind={c.system ? 'system' : c.type === 'view' ? 'view' : c.type === 'timeseries' ? 'timeseries' : 'collection'} /> {c.name}
                </button>
              ))}
            </div>
          </div>
        ))}
      {colls.length === 0 && <span className="muted small">No collections.</span>}
    </div>
  )
}
