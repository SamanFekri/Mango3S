import { ServerIcon } from '../components/icons'
import { timeAgo } from '../lib/library'
import { SHORTCUT_BY_ID, formatBinding, formatShortcut, keysFor } from '../lib/shortcuts'
import { recentProfiles } from '../lib/tabs'
import { useApp } from '../state/AppContext'
import { getShortcutPlatform } from '../state/shortcuts'

const TIPS = ['conn.manager', 'query.run', 'tab.close', 'tab.next', 'results.tree', 'help.shortcuts']

export function WelcomeView() {
  const { openConnectionManager, profiles, sessions, connect, connecting, dispatchTabs, setShortcutsOpen } = useApp()
  const platform = getShortcutPlatform()
  const recent = recentProfiles(profiles).slice(0, 9)
  const quick = SHORTCUT_BY_ID['conn.quick']
  return (
    <div className="view welcome-view">
      <div className="welcome">
        <h1>Welcome to Mango3S</h1>
        <p className="muted">A desktop IDE for MongoDB.</p>
        <div className="welcome-actions">
          <button className="primary" onClick={() => openConnectionManager(true, 'new')}>
            New connection
          </button>
          <button onClick={() => openConnectionManager(true)} title={formatShortcut(SHORTCUT_BY_ID['conn.manager'], platform)}>
            Open saved connections…
          </button>
          <button onClick={() => setShortcutsOpen(true)} title={formatShortcut(SHORTCUT_BY_ID['help.shortcuts'], platform)}>
            Keyboard shortcuts
          </button>
          <button className="ghost" onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'settings' } })}>
            Settings
          </button>
        </div>

        <h2>Recent connections</h2>
        {recent.length === 0 ? (
          <p className="muted">
            No saved connections yet.{' '}
            <button className="link" onClick={() => openConnectionManager(true, 'new')}>
              Create one
            </button>{' '}
            to get started.
          </p>
        ) : (
          <ul className="welcome-list" aria-label="Recent connections">
            {recent.map((p, i) => {
              const live = sessions.find((s) => s.id === p.id)
              return (
                <li key={p.id}>
                  <ServerIcon live={!!live} />
                  <span className="welcome-name">
                    {p.folder && <span className="muted">{p.folder} / </span>}
                    {p.name}
                  </span>
                  <span className="muted small mono welcome-uri">{p.server?.mode === 'manual' ? p.server.hosts.map((h) => (h.port ? `${h.host}:${h.port}` : h.host)).join(', ') : p.uri}</span>
                  <span className="muted small">{p.lastConnectedAt ? timeAgo(p.lastConnectedAt) : 'never used'}</span>
                  <span className="spacer" />
                  <kbd title="Quick connect">{formatBinding(keysFor(quick, platform)[i], platform)}</kbd>
                  <button className="small ghost" onClick={() => openConnectionManager(true, p.id)}>
                    Edit
                  </button>
                  {live ? (
                    <button className="small" onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'connection', connId: live.id, connName: live.name } })}>
                      Open
                    </button>
                  ) : (
                    <button className="small primary" onClick={() => void connect(p.id)} disabled={connecting.has(p.id)}>
                      {connecting.has(p.id) ? 'Connecting…' : 'Connect'}
                    </button>
                  )}
                </li>
              )
            })}
          </ul>
        )}

        <h2>Keyboard</h2>
        <ul className="tips">
          {TIPS.map((id) => {
            const s = SHORTCUT_BY_ID[id]
            return (
              <li key={id}>
                <kbd>{formatShortcut(s, platform)}</kbd> {s.label}
              </li>
            )
          })}
          <li>Double-click a value in the Tree or Table view to edit just that field; Enter saves, Esc cancels.</li>
        </ul>
      </div>
    </div>
  )
}
