import { useApp } from '../state/AppContext'

export function WelcomeView() {
  const { openConnectionManager, profiles, sessions, connect, dispatchTabs } = useApp()
  return (
    <div className="view welcome-view">
      <div className="welcome">
        <h1>Mango3S</h1>
        <p className="muted">A desktop IDE for MongoDB.</p>
        <div className="welcome-actions">
          <button className="primary" onClick={() => openConnectionManager(true)}>
            New connection
          </button>
          <button onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'settings' } })}>Settings</button>
        </div>
        {profiles.length > 0 && (
          <>
            <h2>Recent connections</h2>
            <ul className="welcome-list">
              {profiles.slice(0, 8).map((p) => {
                const live = sessions.some((s) => s.id === p.id)
                return (
                  <li key={p.id}>
                    <span className={live ? 'dot on' : 'dot'} />
                    <span>{p.name}</span>
                    <span className="muted small mono">{p.uri}</span>
                    <span className="spacer" />
                    {!live && (
                      <button className="small" onClick={() => connect(p.id)}>
                        Connect
                      </button>
                    )}
                  </li>
                )
              })}
            </ul>
          </>
        )}
        <h2>Tips</h2>
        <ul className="tips">
          <li>
            <kbd>⌘/Ctrl</kbd> + <kbd>Enter</kbd> runs the query, pipeline or shell script in the current editor.
          </li>
          <li>Click a collection in the explorer to open it; right-click for more actions.</li>
          <li>
            Filters accept mongosh syntax: <code>{'{ age: { $gt: 30 }, _id: ObjectId("…") }'}</code>
          </li>
          <li>Mark a connection read-only to block writes when browsing production data.</li>
        </ul>
      </div>
    </div>
  )
}
