import { useEffect, useState } from 'react'
import { Spinner } from './common'
import { backend, onPlatformTrace } from '../lib/backend'
import type { PlatformCheck, PlatformDebug, PlatformProbeStep, PlatformTraceEntry } from '../lib/types'

/** Settings › Diagnostics › Platform connection: which server this build talks
 * to, each platform API with its own Run button (or all of them in order), and
 * — when the app runs with MANGO3S_PLATFORM_TRACE (as `make dev` does) — every
 * request it sends. */
export function PlatformConnection() {
  const [debug, setDebug] = useState<PlatformDebug | null>(null)
  const [checks, setChecks] = useState<PlatformCheck[]>([])
  const [results, setResults] = useState<Record<string, PlatformProbeStep>>({})
  const [running, setRunning] = useState<string | null>(null) // check ID, or 'all'
  const [failure, setFailure] = useState('')
  const [requests, setRequests] = useState<PlatformTraceEntry[]>([])

  const refresh = () =>
    backend
      .getPlatformDebug()
      .then((d) => {
        setDebug(d)
        setRequests(d.requests ?? [])
      })
      .catch(() => undefined)

  useEffect(() => {
    refresh()
    backend
      .getPlatformChecks()
      .then(setChecks)
      .catch(() => undefined)
    return onPlatformTrace((e) => setRequests((rs) => [...rs.filter((r) => r.id !== e.id), e].slice(-300)))
  }, [])

  const run = async (id: string) => {
    setRunning(id)
    setFailure('')
    try {
      if (id === 'all') {
        const rep = await backend.testPlatformConnection(false)
        setResults(Object.fromEntries(rep.steps.map((s) => [s.id, s])))
      } else {
        const step = await backend.runPlatformCheck(id)
        setResults((r) => ({ ...r, [id]: step }))
      }
    } catch (err) {
      setFailure(err instanceof Error ? err.message : String(err))
    } finally {
      setRunning(null)
      refresh()
    }
  }

  const clear = async () => {
    await backend.clearPlatformTrace().catch(() => undefined)
    setRequests([])
  }

  if (!debug) return null
  return (
    <div className="platform-conn" id="settings-platform-connection">
      <h3>Platform connection</h3>
      <dl className="stats">
        <dt>Server</dt>
        <dd className="mono">
          {debug.baseUrl} <span className="muted">({debug.baseUrlSource})</span>
        </dd>
        <dt>Registration</dt>
        <dd>{debug.registered ? <span className="mono">{debug.installationId}</span> : 'Not registered yet — run Register installation first'}</dd>
        <dt>Last check-in</dt>
        <dd>
          {debug.status.revoked
            ? 'Blocked by an admin'
            : debug.status.connected
              ? 'Reached the server'
              : debug.status.error
                ? `Failed: ${debug.status.error}`
                : 'Not checked yet'}
          {debug.status.lastCheckedAt && <span className="muted"> · {new Date(debug.status.lastCheckedAt).toLocaleTimeString()}</span>}
        </dd>
      </dl>

      <h3>
        APIs
        <span className="spacer" />
        <button onClick={() => run('all')} disabled={running !== null}>
          {running === 'all' && <Spinner small />} Run all
        </button>
      </h3>
      {failure && <p className="small pc-fail">{failure}</p>}
      <div className="pc-checks" role="table" aria-label="Platform APIs">
        {checks.map((c) => {
          const r = results[c.id]
          const state = !r ? '' : r.skipped ? 'skip' : r.ok ? 'ok' : 'fail'
          return (
            <div key={c.id} className={`pc-check ${state}`} role="row">
              <span className="pc-mark" aria-hidden="true">
                {!r ? '' : r.skipped ? '–' : r.ok ? '✓' : '✗'}
              </span>
              <div className="pc-check-main">
                <div>
                  <strong>{c.name}</strong> <span className="mono small muted">{c.method} {r?.path || c.path}</span>
                  {c.writes && <span className="badge">WRITES</span>}
                </div>
                <div className="small muted">{c.description}</div>
                {r && (
                  <div className="small pc-detail">
                    {!r.skipped && (
                      <span className="mono">
                        {r.status || '—'} · {r.durationMs} ms ·{' '}
                      </span>
                    )}
                    {r.error || r.detail}
                  </div>
                )}
              </div>
              <button onClick={() => run(c.id)} disabled={running !== null} aria-label={`Run ${c.name}`}>
                {running === c.id ? <Spinner small /> : 'Run'}
              </button>
            </div>
          )
        })}
      </div>

      <h3>
        Requests {debug.traceEnabled && <span className="badge">LIVE</span>}
        {debug.traceEnabled && requests.length > 0 && (
          <button className="link small" onClick={clear}>
            Clear
          </button>
        )}
      </h3>
      {!debug.traceEnabled ? (
        <p className="small muted">
          Request logging is off. Start the app with <code>make dev</code> (or <code>MANGO3S_PLATFORM_TRACE=1</code>) to see every call it sends to the
          server here and in the terminal. <code>make platform-check CHECK=&lt;id&gt;</code> runs one API from the terminal.
        </p>
      ) : requests.length === 0 ? (
        <p className="small muted">No requests yet. The app checks in about 5 seconds after starting, then every 10 minutes; Run sends one now.</p>
      ) : (
        <div className="pc-log">
          {[...requests].reverse().map((r) => (
            <TraceRow key={r.id} entry={r} />
          ))}
        </div>
      )}
    </div>
  )
}

function TraceRow({ entry: e }: { entry: PlatformTraceEntry }) {
  const kind = e.status === 0 || e.status >= 400 ? 'fail' : 'ok'
  let path = e.url
  try {
    const u = new URL(e.url)
    path = u.pathname + u.search
  } catch {
    // keep the raw URL
  }
  return (
    <details className={`pc-row ${kind}`}>
      <summary>
        <span className="mono">{e.method}</span>
        <span className="mono pc-path">{path}</span>
        <span className="mono pc-status">{e.status || '—'}</span>
        <span className="small muted">{e.durationMs} ms</span>
        <span className="small muted">{new Date(e.time).toLocaleTimeString()}</span>
      </summary>
      {e.error && <p className="small pc-fail">{e.error}</p>}
      <Headers title="Request headers" headers={e.requestHeaders} />
      {e.requestBody && <Body title="Request body" text={e.requestBody} />}
      <Headers title="Response headers" headers={e.responseHeaders} />
      {e.responseBody && <Body title="Response body" text={e.responseBody} />}
    </details>
  )
}

function Headers({ title, headers }: { title: string; headers?: Record<string, string> }) {
  const keys = Object.keys(headers ?? {}).sort()
  if (keys.length === 0) return null
  return <Body title={title} text={keys.map((k) => `${k}: ${headers![k]}`).join('\n')} />
}

function Body({ title, text }: { title: string; text: string }) {
  return (
    <div className="pc-body">
      <div className="small muted">{title}</div>
      <pre className="mono">{text}</pre>
    </div>
  )
}
