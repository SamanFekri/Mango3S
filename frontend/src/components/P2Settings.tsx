import { PasswordInput } from './PasswordInput'
import { CodeBlock } from '../lib/syntax'
import { useCallback, useEffect, useState } from 'react'
import { backend } from '../lib/backend'
import { toAppError } from '../lib/errors'
import type { AIProviderId, AIProviders, AIStatus, CLIInfo, MaskRuleSet, MCPInfo, MCPPolicy, Settings } from '../lib/types'
import { useApp } from '../state/AppContext'
import { Spinner } from './common'

const PROVIDERS: { id: AIProviderId; name: string; blurb: string }[] = [
  { id: 'anthropic', name: 'Anthropic API', blurb: 'Claude models with your API key (billed per use).' },
  { id: 'claudeCode', name: 'Claude Code', blurb: 'Uses the Claude Code installed on this computer and its sign-in (your Claude plan).' },
  { id: 'codex', name: 'Codex', blurb: 'Uses the Codex CLI installed on this computer and its sign-in.' },
  { id: 'local', name: 'Local model', blurb: 'Ollama, LM Studio, llama.cpp, vLLM or any OpenAI-compatible server. Nothing leaves this computer.' },
]

const AI_FIELDS: (keyof Settings)[] = [
  'aiEnabled',
  'aiProvider',
  'aiModel',
  'aiTimeoutSeconds',
  'claudeCodePath',
  'claudeCodeModel',
  'codexPath',
  'codexModel',
  'localUrl',
  'localApi',
  'localModel',
]

function isLoopback(url: string) {
  try {
    const h = new URL(url).hostname
    return h === 'localhost' || h === '127.0.0.1' || h === '::1' || h === '[::1]'
  } catch {
    return false
  }
}

function CLIStatus({ info }: { info?: CLIInfo }) {
  if (!info) return <span className="muted small">Checking…</span>
  if (!info.found) return <span className="tool-status missing">not installed</span>
  if (info.signedIn === false) return <span className="tool-status incompatible">not signed in</span>
  return <span className="tool-status ok">{info.signedIn ? 'signed in' : 'installed'}</span>
}

/** AI and scheduler preferences; part of the Settings form. */
export function AISettings({ form, set }: { form: Settings; set: <K extends keyof Settings>(k: K, v: Settings[K]) => void }) {
  const { toast, toastError, settings } = useApp()
  const [status, setStatus] = useState<AIStatus | null>(null)
  const [found, setFound] = useState<AIProviders | null>(null)
  const [detecting, setDetecting] = useState(false)
  const [key, setKey] = useState('')
  const [localKey, setLocalKey] = useState('')
  const [models, setModels] = useState<string[] | null>(null)
  const [modelsError, setModelsError] = useState('')
  const [test, setTest] = useState<{ busy: boolean; ok?: string; error?: string }>({ busy: false })
  const refresh = useCallback(
    () =>
      backend
        .getAIStatus()
        .then(setStatus)
        .catch(() => undefined),
    [],
  )
  const detect = useCallback(async () => {
    setDetecting(true)
    try {
      setFound(await backend.detectAIProviders())
    } finally {
      setDetecting(false)
    }
  }, [])
  useEffect(() => {
    void refresh()
    void detect()
  }, [refresh, detect])

  const loadModels = useCallback(
    async (url = form.localUrl, api = form.localApi) => {
      setModelsError('')
      try {
        setModels(await backend.listLocalModels(url, api))
      } catch (err) {
        setModels(null)
        setModelsError(toAppError(err).message)
      }
    },
    [form.localUrl, form.localApi],
  )
  useEffect(() => {
    if (form.aiProvider === 'local') void loadModels()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [form.aiProvider])

  const unsaved = !!settings && AI_FIELDS.some((k) => settings[k] !== form[k])
  const runTest = async () => {
    setTest({ busy: true })
    try {
      setTest({ busy: false, ok: await backend.testAI('ai-test') })
    } catch (err) {
      setTest({ busy: false, error: toAppError(err).message })
    }
  }
  const p = form.aiProvider
  const remote = p === 'local' ? !isLoopback(form.localUrl) : true

  return (
    <>
      <section className="settings-group ai-settings">
        <h2>AI assistance</h2>
        <label className="check">
          <input type="checkbox" checked={form.aiEnabled} onChange={(e) => set('aiEnabled', e.target.checked)} />
          Enable natural-language query generation
        </label>

        <div className="field">
          <span>Provider</span>
          <div className="provider-grid" role="radiogroup" aria-label="AI provider">
            {PROVIDERS.map((pr) => {
              const badge =
                pr.id === 'claudeCode' ? (
                  <CLIStatus info={found?.claudeCode} />
                ) : pr.id === 'codex' ? (
                  <CLIStatus info={found?.codex} />
                ) : pr.id === 'local' ? (
                  found ? (
                    found.localServers.length ? (
                      <span className="tool-status ok">{found.localServers.map((s) => s.name).join(', ')}</span>
                    ) : (
                      <span className="tool-status missing">no server running</span>
                    )
                  ) : (
                    <span className="muted small">Checking…</span>
                  )
                ) : status?.hasKey ? (
                  <span className="tool-status ok">key set</span>
                ) : (
                  <span className="tool-status missing">no key</span>
                )
              return (
                <button
                  key={pr.id}
                  type="button"
                  role="radio"
                  aria-checked={p === pr.id}
                  className={p === pr.id ? 'provider-card active' : 'provider-card'}
                  onClick={() => set('aiProvider', pr.id)}
                >
                  <span className="provider-head">
                    <strong>{pr.name}</strong> {badge}
                  </span>
                  <span className="muted small">{pr.blurb}</span>
                </button>
              )
            })}
          </div>
          <div className="form-actions left">
            <button type="button" className="small ghost" onClick={() => void detect()} disabled={detecting}>
              {detecting && <Spinner small />} Detect again
            </button>
          </div>
        </div>

        {p === 'anthropic' && (
          <>
            <label className="field">
              <span>Model</span>
              <input className="mono" value={form.aiModel} onChange={(e) => set('aiModel', e.target.value)} aria-label="Anthropic model" />
            </label>
            <div className="field">
              <span>
                API key —{' '}
                {status?.keySource === 'stored' ? 'stored in the credential store' : status?.keySource === 'environment' ? 'from ANTHROPIC_API_KEY' : 'not set'}
              </span>
              <div className="file-row">
                <PasswordInput
                  placeholder="sk-ant-…"
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                  autoComplete="off"
                  aria-label="Anthropic API key"
                />
                <button
                  type="button"
                  disabled={!key}
                  onClick={async () => {
                    try {
                      await backend.setAIKey(key)
                      setKey('')
                      void refresh()
                      toast({ kind: 'success', title: 'API key stored' })
                    } catch (err) {
                      toastError(err, 'Could not store the key')
                    }
                  }}
                >
                  Save key
                </button>
                {status?.keySource === 'stored' && (
                  <button
                    type="button"
                    className="ghost danger-text"
                    onClick={async () => {
                      await backend.clearAIKey()
                      void refresh()
                    }}
                  >
                    Remove
                  </button>
                )}
              </div>
            </div>
          </>
        )}

        {(p === 'claudeCode' || p === 'codex') &&
          (() => {
            const info = p === 'claudeCode' ? found?.claudeCode : found?.codex
            const pathKey = p === 'claudeCode' ? 'claudeCodePath' : 'codexPath'
            const modelKey = p === 'claudeCode' ? 'claudeCodeModel' : 'codexModel'
            return (
              <>
                <div className="cli-status small">
                  {info?.found ? (
                    <>
                      {info.name} {info.version} · <span className="mono">{info.path}</span>
                      {info.signedIn && info.account ? ` · signed in with ${info.account}` : ''}
                      {info.message && <div className="muted">{info.message}</div>}
                    </>
                  ) : (
                    <span>{info?.message ?? 'Checking…'}</span>
                  )}
                </div>
                <div className="field-row">
                  <label className="field">
                    <span>Model (optional)</span>
                    <input
                      className="mono"
                      value={form[modelKey]}
                      onChange={(e) => set(modelKey, e.target.value)}
                      placeholder={p === 'claudeCode' ? "the CLI's default, or e.g. sonnet, opus" : "the CLI's default"}
                      aria-label={`${info?.name ?? 'CLI'} model`}
                    />
                  </label>
                  <div className="field">
                    <span>Program path (optional)</span>
                    <div className="file-row">
                      <input
                        className="mono"
                        value={form[pathKey]}
                        onChange={(e) => set(pathKey, e.target.value)}
                        placeholder="found automatically"
                        aria-label={`${info?.name ?? 'CLI'} path`}
                      />
                      <button
                        type="button"
                        onClick={async () => {
                          const f = await backend.chooseFile('tool', `Choose the ${p === 'claudeCode' ? 'claude' : 'codex'} program`).catch(() => '')
                          if (f) set(pathKey, f)
                        }}
                      >
                        Browse…
                      </button>
                    </div>
                  </div>
                </div>
                <p className="muted small">
                  {p === 'claudeCode'
                    ? 'Runs `claude -p` with every tool disabled, no MCP servers, no settings or hooks loaded and nothing saved, in an empty folder. Usage counts against your Claude plan.'
                    : 'Runs `codex exec` in its read-only sandbox without saving the session, in an empty folder. Codex is an agent: it can read files on this computer even though it is told not to. Prefer Claude Code, the API or a local model for databases you do not trust. Usage counts against your Codex/ChatGPT plan.'}{' '}
                  Anthropic’s and OpenAI’s terms apply to how you use your subscription.
                </p>
              </>
            )
          })()}

        {p === 'local' && (
          <>
            {found && found.localServers.length > 0 && (
              <div className="field">
                <span>Servers running on this computer</span>
                <div className="chip-list">
                  {found.localServers.map((srv) => (
                    <button
                      type="button"
                      key={srv.url}
                      className={form.localUrl === srv.url ? 'chip active' : 'chip'}
                      onClick={() => {
                        set('localUrl', srv.url)
                        set('localApi', srv.api)
                        if (srv.models[0] && !srv.models.includes(form.localModel)) set('localModel', srv.models[0])
                        setModels(srv.models)
                      }}
                    >
                      {srv.name} · {srv.models.length} model{srv.models.length === 1 ? '' : 's'}
                    </button>
                  ))}
                </div>
              </div>
            )}
            <div className="field-row">
              <label className="field">
                <span>Server URL</span>
                <input
                  className="mono"
                  value={form.localUrl}
                  onChange={(e) => set('localUrl', e.target.value)}
                  onBlur={() => void loadModels()}
                  aria-label="Server URL"
                />
              </label>
              <label className="field narrow">
                <span>API</span>
                <select
                  value={form.localApi}
                  onChange={(e) => {
                    set('localApi', e.target.value as Settings['localApi'])
                    void loadModels(form.localUrl, e.target.value as Settings['localApi'])
                  }}
                  aria-label="Server API"
                >
                  <option value="ollama">Ollama</option>
                  <option value="openai">OpenAI-compatible</option>
                </select>
              </label>
            </div>
            <div className="field">
              <span>Model</span>
              <div className="file-row">
                {models && models.length > 0 ? (
                  <select value={form.localModel} onChange={(e) => set('localModel', e.target.value)} aria-label="Local model">
                    {!models.includes(form.localModel) && <option value={form.localModel}>{form.localModel || 'Choose a model'}</option>}
                    {models.map((m) => (
                      <option key={m} value={m}>
                        {m}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    className="mono"
                    value={form.localModel}
                    onChange={(e) => set('localModel', e.target.value)}
                    placeholder="e.g. qwen2.5-coder:7b"
                    aria-label="Local model"
                  />
                )}
                <button type="button" className="small" onClick={() => void loadModels()}>
                  Refresh models
                </button>
              </div>
              {modelsError && (
                <small className="error-text">
                  {modelsError} {form.localApi === 'ollama' && 'Start it with `ollama serve`, then pull a model, e.g. `ollama pull qwen2.5-coder:7b`.'}
                </small>
              )}
              <small className="hint">
                Coding-tuned models (Qwen2.5-Coder, Llama 3.1+, Mistral) follow the JSON format best. Answers are validated before you can use them.
              </small>
            </div>
            {form.localApi === 'openai' && (
              <div className="field">
                <span>API key (only if the server needs one) — {status?.localHasKey ? 'stored' : 'none'}</span>
                <div className="file-row">
                  <PasswordInput value={localKey} onChange={(e) => setLocalKey(e.target.value)} autoComplete="off" aria-label="Local server API key" />
                  <button
                    type="button"
                    disabled={!localKey}
                    onClick={async () => {
                      await backend.setLocalAIKey(localKey)
                      setLocalKey('')
                      void refresh()
                    }}
                  >
                    Save key
                  </button>
                  {status?.localHasKey && (
                    <button
                      type="button"
                      className="ghost danger-text"
                      onClick={async () => {
                        await backend.clearLocalAIKey()
                        void refresh()
                      }}
                    >
                      Remove
                    </button>
                  )}
                </div>
              </div>
            )}
            {remote && (
              <div className="info-banner small">
                This server is not on this computer: the collection’s field names (and masked samples, if allowed) are sent to {form.localUrl}.
              </div>
            )}
          </>
        )}

        <div className="field-row">
          <label className="field narrow">
            <span>Timeout (s)</span>
            <input
              type="number"
              min={10}
              max={1800}
              value={form.aiTimeoutSeconds}
              onChange={(e) => set('aiTimeoutSeconds', Number(e.target.value))}
              aria-label="AI timeout"
            />
          </label>
        </div>
        <label className="check">
          <input type="checkbox" checked={form.aiAllowSamples} onChange={(e) => set('aiAllowSamples', e.target.checked)} />
          Allow sending a few masked sample documents (values are scrambled; field names and types are kept)
        </label>
        <p className="muted small">
          {remote ? 'Sent to the provider' : 'Given to the local model'}: your request, the collection name and its field names and types (plus masked samples
          if allowed). Connection strings, hosts, user names and passwords are never included. Every proposal is validated by the server without running it, and
          writes always need your confirmation.
        </p>
        <div className="form-actions left">
          <button
            type="button"
            onClick={() => void runTest()}
            disabled={test.busy || unsaved || !form.aiEnabled}
            title={unsaved ? 'Save settings first' : 'Send a tiny request to check the setup'}
          >
            {test.busy && <Spinner small />} Test
          </button>
          {unsaved && <span className="muted small">Save settings to test these changes.</span>}
          {!form.aiEnabled && !unsaved && <span className="muted small">Enable AI assistance to test it.</span>}
          {test.ok && <span className="success-text small">✓ {test.ok}</span>}
          {test.error && <span className="error-text small">{test.error}</span>}
        </div>
        {status && settings?.aiEnabled && !unsaved && (
          <p className={status.ready ? 'muted small' : 'error-text small'}>
            {status.providerName}: {status.ready ? 'ready' : 'not ready'}
            {status.detail ? ` — ${status.detail}` : ''}
          </p>
        )}
      </section>
      <section className="settings-group">
        <h2>Scheduler</h2>
        <label className="check">
          <input type="checkbox" checked={form.schedulerEnabled} onChange={(e) => set('schedulerEnabled', e.target.checked)} />
          Run approved scheduled tasks while Mango3S is open
        </label>
      </section>
    </>
  )
}

/** MCP server policy; saved separately from the settings form. */
export function MCPSettings() {
  const { profiles, toast, toastError } = useApp()
  const [info, setInfo] = useState<MCPInfo | null>(null)
  const [policy, setPolicy] = useState<MCPPolicy | null>(null)
  const [ruleSets, setRuleSets] = useState<MaskRuleSet[]>([])
  const [dbText, setDbText] = useState('')

  useEffect(() => {
    backend
      .getMCPInfo()
      .then((i) => {
        setInfo(i)
        setPolicy(i.policy)
        setDbText(i.policy.databases.join(', '))
      })
      .catch(() => undefined)
    backend
      .listMaskRuleSets()
      .then(setRuleSets)
      .catch(() => undefined)
  }, [])
  if (!info || !policy) return null

  const toggle = (list: string[], v: string) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v])
  const save = async () => {
    try {
      const saved = await backend.saveMCPPolicy({
        ...policy,
        databases: dbText
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      })
      setPolicy(saved)
      toast({ kind: 'success', title: 'MCP policy saved', message: 'MCP clients pick it up on their next start.' })
    } catch (err) {
      toastError(err, 'Could not save the MCP policy')
    }
  }

  return (
    <section className="settings-group">
      <h2>MCP server</h2>
      <p className="muted small">
        Lets MCP clients (Claude Desktop, Claude Code, IDE agents) use approved database tools by running <code>mango3s mcp</code>. Everything is denied unless
        allowed here.
      </p>
      <label className="check">
        <input type="checkbox" checked={policy.enabled} onChange={(e) => setPolicy({ ...policy, enabled: e.target.checked })} />
        Enable the MCP server
      </label>
      <div className="field">
        <span>Connections shared with MCP clients</span>
        {profiles.map((p) => (
          <label className="check" key={p.id}>
            <input
              type="checkbox"
              checked={policy.profiles.includes(p.id)}
              onChange={() => setPolicy({ ...policy, profiles: toggle(policy.profiles, p.id) })}
            />
            {p.name} {p.readOnly && <span className="badge">read-only</span>}
          </label>
        ))}
      </div>
      <label className="field">
        <span>Databases (comma-separated patterns, e.g. shop, analytics_*; system databases are always excluded)</span>
        <input className="mono" value={dbText} onChange={(e) => setDbText(e.target.value)} />
      </label>
      <div className="field">
        <span>Tools</span>
        <div className="tool-grid">
          {info.catalog.map((t) => (
            <label className="check" key={t.name} title={t.description}>
              <input type="checkbox" checked={policy.tools.includes(t.name)} onChange={() => setPolicy({ ...policy, tools: toggle(policy.tools, t.name) })} />
              <code>{t.name}</code> {t.write && <span className="badge danger-badge">write</span>}
            </label>
          ))}
        </div>
      </div>
      <label className="check">
        <input type="checkbox" checked={policy.allowWrites} onChange={(e) => setPolicy({ ...policy, allowWrites: e.target.checked })} />
        Allow write tools (still blocked for read-only connections; capped and backed up)
      </label>
      <div className="field-row">
        <label className="field narrow">
          <span>Max documents per read</span>
          <input type="number" min={1} max={500} value={policy.maxDocs} onChange={(e) => setPolicy({ ...policy, maxDocs: Number(e.target.value) })} />
        </label>
        <label className="field narrow">
          <span>Max documents per write</span>
          <input
            type="number"
            min={1}
            max={10000}
            value={policy.maxWriteDocs}
            onChange={(e) => setPolicy({ ...policy, maxWriteDocs: Number(e.target.value) })}
          />
        </label>
        <label className="field">
          <span>Mask returned documents with</span>
          <select value={policy.maskRuleSetId} onChange={(e) => setPolicy({ ...policy, maskRuleSetId: e.target.value })}>
            <option value="">no masking</option>
            {ruleSets.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </select>
        </label>
      </div>
      <label className="check">
        <input type="checkbox" checked={policy.auditLog} onChange={(e) => setPolicy({ ...policy, auditLog: e.target.checked })} />
        Log every tool call (tool, result, duration — not arguments)
      </label>
      <div className="form-actions">
        <button className="primary" onClick={save}>
          Save MCP policy
        </button>
      </div>
      <div className="field">
        <span>Client configuration (e.g. claude_desktop_config.json)</span>
        <CodeBlock code={info.snippet} lang="json" className="code-output small" />
        <button className="small" onClick={() => navigator.clipboard?.writeText(info.snippet)}>
          Copy
        </button>
      </div>
    </section>
  )
}
