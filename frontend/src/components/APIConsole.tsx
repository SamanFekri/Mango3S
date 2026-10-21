import { useEffect, useMemo, useState } from 'react'
import { Spinner } from './common'
import { backend } from '../lib/backend'
import type { APIConsoleState, APIEndpoint, APIRequest, APIResponse } from '../lib/types'

const AUTH_LABEL: Record<string, string> = {
  none: 'No auth',
  installation: 'Installation token',
  account: 'Account token',
  device: 'Installation token (X-Installation-Token, optional)',
}

/** Settings › API console (test mode only): every web platform route the app
 * talks to, each with its own Send button (the admin dashboard's routes are
 * not here). Each request is editable before sending, and {variables} in it
 * are filled from the panel at the top, which responses update (tokens, IDs). */
export function APIConsole() {
  const [state, setState] = useState<APIConsoleState | null>(null)
  const [filter, setFilter] = useState('')
  const [open, setOpen] = useState<string | null>(null)
  const [drafts, setDrafts] = useState<Record<string, APIRequest>>({})
  const [responses, setResponses] = useState<Record<string, APIResponse>>({})
  const [sending, setSending] = useState<string | null>(null)
  const [varsOpen, setVarsOpen] = useState(true)

  useEffect(() => {
    backend
      .getAPIConsole()
      .then(setState)
      .catch(() => undefined)
  }, [])

  const groups = useMemo(() => {
    const out = new Map<string, APIEndpoint[]>()
    const f = filter.trim().toLowerCase()
    for (const e of state?.endpoints ?? []) {
      if (f && !`${e.group} ${e.name} ${e.method} ${e.path} ${e.id}`.toLowerCase().includes(f)) continue
      out.set(e.group, [...(out.get(e.group) ?? []), e])
    }
    return out
  }, [state, filter])

  if (!state) return <Spinner />
  if (!state.enabled) {
    return (
      <section className="settings-group">
        <h2>API console</h2>
        <p className="small muted">Available in test mode only: start the app with <code>make dev</code>.</p>
      </section>
    )
  }

  const draftOf = (e: APIEndpoint): APIRequest =>
    drafts[e.id] ?? { endpointId: e.id, method: e.method, path: e.path, query: e.query ?? '', body: e.body ?? '', auth: e.auth }
  const edit = (e: APIEndpoint, patch: Partial<APIRequest>) => setDrafts((d) => ({ ...d, [e.id]: { ...draftOf(e), ...patch } }))

  const send = async (e: APIEndpoint) => {
    setSending(e.id)
    setOpen(e.id)
    try {
      const res = await backend.sendAPIRequest(draftOf(e))
      setResponses((r) => ({ ...r, [e.id]: res }))
      if (res.captured && Object.keys(res.captured).length > 0) {
        const fresh = await backend.getAPIConsole()
        setState(fresh)
      }
    } catch (err) {
      setResponses((r) => ({
        ...r,
        [e.id]: { endpointId: e.id, method: e.method, url: '', status: 0, statusText: '', durationMs: 0, requestHeaders: {}, error: err instanceof Error ? err.message : String(err) },
      }))
    } finally {
      setSending(null)
    }
  }

  const setVar = async (name: string, value: string) => {
    setState((s) => (s ? { ...s, vars: { ...s.vars, [name]: value } } : s))
    await backend.setAPIConsoleVar(name, value).catch(() => undefined)
  }

  const resetVars = async () => {
    const vars = await backend.resetAPIConsole().catch(() => null)
    if (vars) setState((s) => (s ? { ...s, vars } : s))
  }

  const varNames = Object.keys(state.vars).sort((a, b) => a.localeCompare(b))
  return (
    <section className="settings-group api-console" id="settings-api-console">
      <h2>API console</h2>
      <p className="small muted">
        Sends requests to <span className="mono">{state.baseUrl}</span>, one at a time. <code>{'{name}'}</code> in a path, query or body is filled from the
        variables below; responses fill them in too (tokens, IDs). Every request also appears in Diagnostics › Requests.
      </p>

      <details className="ac-vars" open={varsOpen} onToggle={(ev) => setVarsOpen((ev.target as HTMLDetailsElement).open)}>
        <summary>
          Variables
          <button
            className="link small"
            onClick={(ev) => {
              ev.preventDefault()
              resetVars()
            }}
          >
            Reset
          </button>
        </summary>
        <div className="ac-var-grid">
          {varNames.map((name) => (
            <label key={name} className="ac-var">
              <span className="mono small">{name}</span>
              <input
                className="mono"
                type={state.secret.includes(name) ? 'password' : 'text'}
                value={state.vars[name] ?? ''}
                onChange={(ev) => setVar(name, ev.target.value)}
                spellCheck={false}
                autoComplete="off"
                aria-label={`Variable ${name}`}
              />
            </label>
          ))}
        </div>
      </details>

      <input className="ac-filter" placeholder="Filter endpoints" value={filter} onChange={(ev) => setFilter(ev.target.value)} aria-label="Filter endpoints" />

      {[...groups.entries()].map(([group, endpoints]) => (
        <div key={group} className="ac-group">
          <h3>{group}</h3>
          {endpoints.map((e) => {
            const d = draftOf(e)
            const res = responses[e.id]
            const isOpen = open === e.id
            return (
              <div key={e.id} className={`ac-endpoint${isOpen ? ' open' : ''}`}>
                <div className="ac-row">
                  <button className="ac-toggle" onClick={() => setOpen(isOpen ? null : e.id)} aria-expanded={isOpen} aria-label={`Edit ${e.name}`}>
                    <span className={`ac-method m-${e.method.toLowerCase()}`}>{e.method}</span>
                    <span className="ac-name">{e.name}</span>
                    <span className="mono small muted ac-path">{e.path}</span>
                  </button>
                  {res && <StatusPill res={res} />}
                  <button className="primary" onClick={() => send(e)} disabled={sending !== null} aria-label={`Send ${e.name}`}>
                    {sending === e.id ? <Spinner small /> : 'Send'}
                  </button>
                </div>
                {isOpen && (
                  <div className="ac-editor">
                    {e.description && <p className="small muted">{e.description}</p>}
                    <div className="ac-line">
                      <select value={d.method} onChange={(ev) => edit(e, { method: ev.target.value })} aria-label="Method">
                        {['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map((m) => (
                          <option key={m}>{m}</option>
                        ))}
                      </select>
                      <input className="mono" value={d.path} onChange={(ev) => edit(e, { path: ev.target.value })} aria-label="Path" spellCheck={false} />
                    </div>
                    <label className="field">
                      <span>Query</span>
                      <input className="mono" value={d.query} onChange={(ev) => edit(e, { query: ev.target.value })} placeholder="a=1&b=2" spellCheck={false} />
                    </label>
                    <label className="field">
                      <span>Auth</span>
                      <select value={d.auth} onChange={(ev) => edit(e, { auth: ev.target.value })}>
                        {Object.entries(AUTH_LABEL).map(([k, v]) => (
                          <option key={k} value={k}>
                            {v}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label className="field">
                      <span>Body (JSON)</span>
                      <textarea className="mono" rows={Math.min(12, Math.max(3, d.body.split('\n').length + 1))} value={d.body} onChange={(ev) => edit(e, { body: ev.target.value })} spellCheck={false} />
                    </label>
                    <div className="form-actions left">
                      <button
                        onClick={() =>
                          setDrafts((all) => {
                            const next = { ...all }
                            delete next[e.id]
                            return next
                          })
                        }
                      >
                        Reset request
                      </button>
                    </div>
                    {res && <ResponseView res={res} />}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      ))}
    </section>
  )
}

function StatusPill({ res }: { res: APIResponse }) {
  const bad = !!res.error || res.status === 0 || res.status >= 400
  return (
    <span className={`ac-pill ${bad ? 'bad' : 'good'}`}>
      {res.status ? `${res.status} · ${res.durationMs} ms` : 'not sent'}
    </span>
  )
}

function ResponseView({ res }: { res: APIResponse }) {
  return (
    <div className="ac-response">
      <div className="ac-line">
        <StatusPill res={res} />
        <span className="mono small">{res.status ? `${res.status} ${res.statusText}` : ''}</span>
        <span className="mono small muted ac-path">
          {res.method} {res.url}
        </span>
      </div>
      {res.error && <p className="small pc-fail">{res.error}</p>}
      {res.captured && Object.keys(res.captured).length > 0 && (
        <p className="small">
          Saved to variables:{' '}
          {Object.entries(res.captured).map(([k, v]) => (
            <span key={k} className="mono">
              {k} = {v}{' '}
            </span>
          ))}
        </p>
      )}
      {res.responseBody && (
        <div className="pc-body">
          <div className="small muted">Response body</div>
          <pre className="mono">{res.responseBody}</pre>
        </div>
      )}
      <details>
        <summary className="small muted">Headers</summary>
        <div className="pc-body">
          <pre className="mono">
            {Object.entries(res.requestHeaders ?? {})
              .sort()
              .map(([k, v]) => `> ${k}: ${v}`)
              .concat(
                Object.entries(res.responseHeaders ?? {})
                  .sort()
                  .map(([k, v]) => `< ${k}: ${v}`),
              )
              .join('\n')}
          </pre>
        </div>
      </details>
    </div>
  )
}
