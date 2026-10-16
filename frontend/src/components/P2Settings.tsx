import { useEffect, useState } from 'react'
import { backend } from '../lib/backend'
import type { AIStatus, MaskRuleSet, MCPInfo, MCPPolicy, Settings } from '../lib/types'
import { useApp } from '../state/AppContext'

/** AI and scheduler preferences; part of the Settings form. */
export function AISettings({ form, set }: { form: Settings; set: <K extends keyof Settings>(k: K, v: Settings[K]) => void }) {
  const { toast, toastError } = useApp()
  const [status, setStatus] = useState<AIStatus | null>(null)
  const [key, setKey] = useState('')
  const refresh = () => backend.getAIStatus().then(setStatus).catch(() => undefined)
  useEffect(() => {
    refresh()
  }, [])

  return (
    <>
      <section className="settings-group">
        <h2>AI assistance</h2>
        <label className="check">
          <input type="checkbox" checked={form.aiEnabled} onChange={(e) => set('aiEnabled', e.target.checked)} />
          Enable natural-language query generation (Anthropic Claude)
        </label>
        <label className="field">
          <span>Model</span>
          <input className="mono" value={form.aiModel} onChange={(e) => set('aiModel', e.target.value)} />
        </label>
        <label className="check">
          <input type="checkbox" checked={form.aiAllowSamples} onChange={(e) => set('aiAllowSamples', e.target.checked)} />
          Allow sending a few masked sample documents (values are scrambled; field names and types are kept)
        </label>
        <p className="muted small">
          By default only the collection name and field names/types are sent. Connection strings, hosts, user names and passwords are never sent. Proposals
          are validated by the server without running them, and writes always need your confirmation.
        </p>
        <div className="field">
          <span>
            API key —{' '}
            {status?.keySource === 'stored' ? 'stored in the credential store' : status?.keySource === 'environment' ? 'from ANTHROPIC_API_KEY' : 'not set'}
          </span>
          <div className="file-row">
            <input type="password" placeholder="sk-ant-…" value={key} onChange={(e) => setKey(e.target.value)} autoComplete="off" />
            <button
              disabled={!key}
              onClick={async () => {
                try {
                  await backend.setAIKey(key)
                  setKey('')
                  refresh()
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
                className="ghost danger-text"
                onClick={async () => {
                  await backend.clearAIKey()
                  refresh()
                }}
              >
                Remove
              </button>
            )}
          </div>
        </div>
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
    backend.listMaskRuleSets().then(setRuleSets).catch(() => undefined)
  }, [])
  if (!info || !policy) return null

  const toggle = (list: string[], v: string) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v])
  const save = async () => {
    try {
      const saved = await backend.saveMCPPolicy({ ...policy, databases: dbText.split(',').map((s) => s.trim()).filter(Boolean) })
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
        Lets MCP clients (Claude Desktop, Claude Code, IDE agents) use approved database tools by running <code>mango3s mcp</code>. Everything is denied
        unless allowed here.
      </p>
      <label className="check">
        <input type="checkbox" checked={policy.enabled} onChange={(e) => setPolicy({ ...policy, enabled: e.target.checked })} />
        Enable the MCP server
      </label>
      <div className="field">
        <span>Connections shared with MCP clients</span>
        {profiles.map((p) => (
          <label className="check" key={p.id}>
            <input type="checkbox" checked={policy.profiles.includes(p.id)} onChange={() => setPolicy({ ...policy, profiles: toggle(policy.profiles, p.id) })} />
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
          <input type="number" min={1} max={10000} value={policy.maxWriteDocs} onChange={(e) => setPolicy({ ...policy, maxWriteDocs: Number(e.target.value) })} />
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
        <pre className="code-output small">{info.snippet}</pre>
        <button className="small" onClick={() => navigator.clipboard?.writeText(info.snippet)}>
          Copy
        </button>
      </div>
    </section>
  )
}
