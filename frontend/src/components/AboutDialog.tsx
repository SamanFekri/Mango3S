import appIcon from '../assets/appicon.png'
import type { AppInfo } from '../lib/types'
import { useApp } from '../state/AppContext'
import { Modal } from './common'

const SECRET_STORE: Record<string, string> = { keyring: 'OS credential store', file: 'Encrypted file', memory: 'Memory (tests)' }

/** Plain-text details for bug reports (no secrets). */
export function aboutText(i: AppInfo): string {
  const lines = [
    `Mango3S ${i.version}`,
    i.commit && `Build: ${i.commit.slice(0, 12)}${i.modified ? ' (modified)' : ''}${i.buildTime ? ` · ${i.buildTime}` : ''}`,
    `Platform: ${i.platform} · ${i.goVersion}`,
    ...Object.entries(i.components ?? {})
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([k, v]) => `${k}: ${v}`),
    `Config: ${i.configDir}`,
    i.logFile && `Log: ${i.logFile}`,
    `Backups: ${i.backupDir}`,
    `Passwords: ${SECRET_STORE[i.secretBackend] ?? i.secretBackend}`,
  ]
  return lines.filter(Boolean).join('\n')
}

/** About Mango3S (app menu on macOS, Help menu elsewhere). */
export function AboutDialog({ onClose }: { onClose: () => void }) {
  const { appInfo: i, toast } = useApp()
  if (!i) return null
  const built = i.buildTime ? new Date(i.buildTime) : null
  return (
    <Modal
      title="About Mango3S"
      onClose={onClose}
      footer={
        <>
          <button
            onClick={() => {
              void navigator.clipboard?.writeText(aboutText(i))
              toast({ kind: 'info', title: 'Details copied', message: 'Paste them into a bug report.' })
            }}
          >
            Copy details
          </button>
          <span className="spacer" />
          <button className="primary" onClick={onClose} autoFocus>
            Close
          </button>
        </>
      }
    >
      <div className="about">
        <div className="about-head">
          <img src={appIcon} alt="" width={72} height={72} />
          <div>
            <h3>Mango3S</h3>
            <p className="muted">A desktop IDE for MongoDB</p>
            <p>
              Version <strong>{i.version}</strong>
              {i.commit && (
                <span className="muted">
                  {' '}
                  · build <code>{i.commit.slice(0, 7)}</code>
                  {i.modified && ' (local changes)'}
                  {built && !isNaN(built.getTime()) && ` · ${built.toLocaleDateString()}`}
                </span>
              )}
            </p>
          </div>
        </div>

        <h4>Features</h4>
        <p className="small">
          Connection manager with SSH tunnels, proxies and TLS · tree, table and JSON views with in-place field editing · query, aggregation, SQL and
          IntelliShell editors · import/export · schema analysis · explain plans · compare &amp; sync · SQL migration · reschema and data masking ·
          scheduled tasks · AI query assistant (Anthropic API, Claude Code, Codex or local models) · MCP server · customizable Studio 3T-style keymap.
        </p>

        <h4>This installation</h4>
        <dl className="stats about-stats">
          <dt>Platform</dt>
          <dd>
            {i.platform} · {i.goVersion}
          </dd>
          <dt>Settings &amp; data</dt>
          <dd className="mono">{i.configDir}</dd>
          {i.logFile && (
            <>
              <dt>Log file</dt>
              <dd className="mono">{i.logFile}</dd>
            </>
          )}
          <dt>Backups</dt>
          <dd className="mono">{i.backupDir}</dd>
          <dt>Passwords</dt>
          <dd>{SECRET_STORE[i.secretBackend] ?? i.secretBackend}</dd>
          {i.executable && (
            <>
              <dt>Program</dt>
              <dd className="mono">{i.executable}</dd>
            </>
          )}
        </dl>

        {Object.keys(i.components ?? {}).length > 0 && (
          <>
            <h4>Built with</h4>
            <table className="grid compact about-components">
              <tbody>
                {Object.entries(i.components)
                  .sort(([a], [b]) => a.localeCompare(b))
                  .map(([name, v]) => (
                    <tr key={name}>
                      <td>{name}</td>
                      <td className="mono">{v}</td>
                    </tr>
                  ))}
                <tr>
                  <td>React, CodeMirror, Vite</td>
                  <td className="muted">frontend</td>
                </tr>
              </tbody>
            </table>
          </>
        )}

        <p className="muted small about-note">
          Inspired by Studio 3T’s workflow; an independent implementation that shares no code, branding or assets with it. MongoDB is a trademark of
          MongoDB, Inc.
        </p>
      </div>
    </Modal>
  )
}
