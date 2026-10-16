import { useEffect, useState } from 'react'
import { backend } from '../lib/backend'
import { formatDuration } from '../lib/format'
import type { ConnectionStep } from '../lib/types'
import { useApp, type ConnectionLog } from '../state/AppContext'
import { Modal, Spinner } from './common'

const ICON: Record<ConnectionStep['state'], string> = { pending: '○', running: '', ok: '✓', warning: '!', failed: '✗', skipped: '–' }

/** Plain-text version of the log for Copy log (no secrets are ever in it). */
export function logText(log: ConnectionLog): string {
  const lines = [`${log.title} — ${new Date(log.startedAt).toLocaleString()}`]
  for (const s of log.steps) {
    lines.push(`[${s.state.toUpperCase()}] ${s.label}${s.durationMs ? ` (${s.durationMs} ms)` : ''}`)
    if (s.error) lines.push(`    error: ${s.error}`)
    for (const d of (s.detail ?? '').split('\n').filter(Boolean)) lines.push(`    ${d}`)
  }
  if (log.status === 'failed' && log.error && !log.steps.some((s) => s.state === 'failed')) lines.push(`error: ${log.error.message}`)
  if (log.summary) lines.push(log.summary)
  return lines.join('\n')
}

/**
 * Shows each step of Test or Connect as it runs. Failed attempts stay open
 * with the reason; a successful Connect closes by itself unless kept open.
 */
export function ConnectionLogDialog() {
  const { connectionLog: log, closeConnectionLog, toast } = useApp()
  const [keep, setKeep] = useState(false)
  useEffect(() => setKeep(false), [log?.opId])
  useEffect(() => {
    if (log?.kind === 'connect' && log.status === 'ok' && !keep) {
      const id = window.setTimeout(closeConnectionLog, 1500)
      return () => window.clearTimeout(id)
    }
  }, [log?.kind, log?.status, keep, closeConnectionLog])
  if (!log) return null
  const running = log.status === 'running'
  const failedStep = log.steps.find((s) => s.state === 'failed')
  const total = log.finishedAt ? log.finishedAt - log.startedAt : 0
  return (
    <Modal
      title={log.title}
      onClose={() => {
        if (running) void backend.cancel(log.cancelId).catch(() => undefined)
        closeConnectionLog()
      }}
      footer={
        <>
          <button
            onClick={() => {
              void navigator.clipboard?.writeText(logText(log))
              toast({ kind: 'info', title: 'Connection log copied' })
            }}
            disabled={log.steps.length === 0}
          >
            Copy log
          </button>
          {log.kind === 'connect' && log.status === 'ok' && !keep && (
            <button className="ghost small" onClick={() => setKeep(true)}>
              Keep open
            </button>
          )}
          <span className="spacer" />
          {running ? (
            <button
              onClick={() => {
                void backend.cancel(log.cancelId).catch(() => undefined)
              }}
            >
              Cancel
            </button>
          ) : (
            <button className="primary" onClick={closeConnectionLog} autoFocus>
              Close
            </button>
          )}
        </>
      }
    >
      <div className="conn-log" aria-live="polite">
        <ol className="conn-steps">
          {log.steps.map((s) => (
            <li key={s.id} className={`conn-step ${s.state}`}>
              <span className="step-icon" aria-hidden>
                {s.state === 'running' ? <Spinner small /> : ICON[s.state]}
              </span>
              <div className="step-body">
                <div className="step-head">
                  <span className="step-label">{s.label}</span>
                  <span className="step-state sr-only">{s.state}</span>
                  {s.durationMs > 0 && <span className="muted small">{formatDuration(s.durationMs)}</span>}
                </div>
                {s.error && (
                  <div className="step-error" role="alert">
                    {s.error}
                  </div>
                )}
                {s.detail && <pre className="step-detail">{s.detail}</pre>}
              </div>
            </li>
          ))}
          {log.steps.length === 0 && running && (
            <li className="conn-step running">
              <span className="step-icon">
                <Spinner small />
              </span>
              <div className="step-body">Starting…</div>
            </li>
          )}
        </ol>
        {log.status === 'ok' && <div className="success-banner">{log.summary}{total > 0 && ` · ${formatDuration(total)} in total`}</div>}
        {log.status === 'failed' && !failedStep && log.error && (
          <div className="error-banner" role="alert">
            {log.error.message}
          </div>
        )}
        {log.status === 'failed' && failedStep && (
          <p className="muted small">Failed at “{failedStep.label}”. The steps after it were not run. Copy the log to share it; it contains no passwords.</p>
        )}
      </div>
    </Modal>
  )
}
