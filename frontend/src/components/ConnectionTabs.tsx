import { PasswordInput } from './PasswordInput'
// The nine tab panels of the connection dialog. They all edit one
// ProfileInput owned by ConnectionManager, so switching tabs never loses
// changes.
import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { backend } from '../lib/backend'
import { timeAgo } from '../lib/library'
import type { Capability, CollectionHistoryEntry, ParsedURI, Profile, ProfileInput, SecretKind, ServerType, ToolInfo } from '../lib/types'
import { Spinner } from './common'

export type Setter = <K extends keyof ProfileInput>(k: K, v: ProfileInput[K]) => void
export type Patch = <K extends 'server' | 'tls' | 'ssh' | 'proxy' | 'shell' | 'tools' | 'advanced'>(k: K, v: Partial<ProfileInput[K]>) => void

export interface TabProps {
  form: ProfileInput
  set: Setter
  patch: Patch
  stored?: Profile
  caps: Record<string, Capability[]>
}

function Field({ label, hint, children, narrow }: { label: string; hint?: ReactNode; children: ReactNode; narrow?: boolean }) {
  return (
    <label className={narrow ? 'field narrow' : 'field'}>
      <span>{label}</span>
      {children}
      {hint && <small className="hint">{hint}</small>}
    </label>
  )
}

function NumberInput({ value, onChange, min = 0, max, placeholder, label }: { value: number; onChange: (n: number) => void; min?: number; max?: number; placeholder?: string; label?: string }) {
  return (
    <input
      type="number"
      min={min}
      max={max}
      value={value || ''}
      placeholder={placeholder ?? 'default'}
      aria-label={label}
      onChange={(e) => onChange(e.target.value === '' ? 0 : Number(e.target.value))}
    />
  )
}

/** A secret field: empty keeps what is stored; typing replaces it; the checkbox removes it. */
export function SecretField({ label, kind, form, set, stored, placeholder }: { label: string; kind: SecretKind | 'password'; form: ProfileInput; set: Setter; stored?: Profile; placeholder?: string }) {
  const isPw = kind === 'password'
  const has = isPw ? !!stored?.hasPassword : !!stored?.storedSecrets?.includes(kind as SecretKind)
  const value = isPw ? form.password : (form.secrets[kind as SecretKind]?.value ?? '')
  const clear = isPw ? form.clearPassword : !!form.secrets[kind as SecretKind]?.clear
  const update = (v: string, c: boolean) => {
    if (isPw) {
      set('password', v)
      set('clearPassword', c)
    } else set('secrets', { ...form.secrets, [kind]: { value: v, clear: c } })
  }
  return (
    <div className="field">
      <span>{label}</span>
      <PasswordInput
        aria-label={label}
        value={value}
        onChange={(e) => update(e.target.value, false)}
        placeholder={has && !clear ? '•••••• (saved in the credential store)' : (placeholder ?? '')}
        autoComplete="new-password"
        disabled={clear}
      />
      {has && (
        <label className="check small">
          <input type="checkbox" checked={clear} onChange={(e) => update('', e.target.checked)} />
          Remove the saved {label.toLowerCase()}
        </label>
      )}
    </div>
  )
}

function FileField({ label, value, onChange, kind, hint }: { label: string; value: string; onChange: (v: string) => void; kind: 'cert' | 'key' | 'tool' | 'folder'; hint?: ReactNode }) {
  return (
    <div className="field">
      <span>{label}</span>
      <div className="file-row">
        <input className="mono" aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} spellCheck={false} placeholder={kind === 'folder' ? '/path/to/folder' : '/path/to/file'} />
        <button
          type="button"
          onClick={async () => {
            const p = await backend.chooseFile(kind, label).catch(() => '')
            if (p) onChange(p)
          }}
        >
          Browse…
        </button>
        {value && (
          <button type="button" className="ghost" onClick={() => onChange('')} aria-label={`Clear ${label}`}>
            ×
          </button>
        )}
      </div>
      {hint && <small className="hint">{hint}</small>}
    </div>
  )
}

function Unsupported({ items }: { items: Capability[] }) {
  const list = items.filter((c) => !c.supported)
  if (!list.length) return null
  return (
    <div className="unsupported">
      <strong>Not supported</strong>
      <ul>
        {list.map((c) => (
          <li key={c.id}>
            {c.label}: <span className="muted">{c.note}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

// ---- 1. Server -----------------------------------------------------------------

const TYPES: { value: ServerType; label: string }[] = [
  { value: 'standalone', label: 'Standalone' },
  { value: 'replicaSet', label: 'Replica Set' },
  { value: 'sharded', label: 'Sharded Cluster' },
  { value: 'srv', label: 'DNS Seed List (SRV)' },
]

export function ServerTab({ form, set, patch, folders }: TabProps & { folders: string[] }) {
  const s = form.server
  const [preview, setPreview] = useState<{ uri?: string; error?: string }>({})
  const [parsed, setParsed] = useState<ParsedURI | null>(null)
  const [parseError, setParseError] = useState('')

  useEffect(() => {
    if (s.mode !== 'manual') return
    const id = window.setTimeout(() => {
      backend
        .buildConnectionString(s)
        .then((uri) => setPreview({ uri }))
        .catch((e: { message?: string }) => setPreview({ error: e.message ?? String(e) }))
    }, 200)
    return () => window.clearTimeout(id)
  }, [s])

  const setType = (type: ServerType) => {
    const hosts = s.hosts.length ? s.hosts : [{ host: 'localhost', port: 27017 }]
    patch('server', {
      type,
      hosts: type === 'standalone' || type === 'srv' ? [{ host: hosts[0].host, port: type === 'srv' ? 0 : hosts[0].port || 27017 }] : hosts,
      directConnection: type === 'srv' ? false : s.directConnection,
    })
    if (type === 'srv' && !form.tls.enabled) patch('tls', { enabled: true }) // SRV implies TLS by default
  }
  const setHost = (i: number, h: Partial<{ host: string; port: number }>) => patch('server', { hosts: s.hosts.map((x, j) => (j === i ? { ...x, ...h } : x)) })

  const importURI = async () => {
    setParseError('')
    try {
      const p = await backend.parseConnectionString(form.uri)
      setParsed(p)
      patch('server', { ...p.server, mode: 'manual' })
      if (p.username) set('username', p.username)
      if (p.authSource) set('authSource', p.authSource)
      if (p.authMechanism) set('authMechanism', p.authMechanism)
      if (p.tls) patch('tls', { enabled: true })
      if (p.appName) patch('advanced', { appName: p.appName })
      if (p.readPreference) patch('advanced', { readPreference: p.readPreference as ProfileInput['advanced']['readPreference'] })
    } catch (e) {
      setParseError((e as { message?: string }).message ?? String(e))
    }
  }

  return (
    <div className="tab-panel-body">
      <div className="field-row">
        <Field label="Connection name">
          <input value={form.name} onChange={(e) => set('name', e.target.value)} placeholder="Local dev" autoFocus aria-label="Connection name" />
        </Field>
        <Field label="Folder" hint="Group connections, e.g. Clients/ACME">
          <input value={form.folder} onChange={(e) => set('folder', e.target.value)} list="conn-folders" placeholder="(none)" aria-label="Folder" />
          <datalist id="conn-folders">
            {folders.map((f) => (
              <option key={f} value={f} />
            ))}
          </datalist>
        </Field>
      </div>
      <div className="segmented-row">
        <div className="segmented" role="radiogroup" aria-label="How to configure the server">
          <button type="button" role="radio" aria-checked={s.mode === 'manual'} className={s.mode === 'manual' ? 'active' : ''} onClick={() => patch('server', { mode: 'manual' })}>
            Manual
          </button>
          <button type="button" role="radio" aria-checked={s.mode === 'uri'} className={s.mode === 'uri' ? 'active' : ''} onClick={() => patch('server', { mode: 'uri' })}>
            Connection string
          </button>
        </div>
      </div>

      {s.mode === 'manual' ? (
        <>
          <Field label="Connection type">
            <select value={s.type} onChange={(e) => setType(e.target.value as ServerType)} aria-label="Connection type">
              {TYPES.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </select>
          </Field>
          <div className="field">
            <span>{s.type === 'srv' ? 'Cluster host name' : s.type === 'sharded' ? 'mongos routers' : s.type === 'replicaSet' ? 'Members (seed list)' : 'Server'}</span>
            {s.hosts.map((h, i) => (
              <div className="host-row" key={i}>
                <input className="mono" aria-label={`Host ${i + 1}`} value={h.host} onChange={(e) => setHost(i, { host: e.target.value.trim() })} placeholder="hostname" spellCheck={false} />
                {s.type !== 'srv' && (
                  <input className="narrow-input" type="number" aria-label={`Port ${i + 1}`} min={1} max={65535} value={h.port || ''} onChange={(e) => setHost(i, { port: Number(e.target.value) })} placeholder="27017" />
                )}
                {(s.type === 'replicaSet' || s.type === 'sharded') && s.hosts.length > 1 && (
                  <button type="button" className="ghost" aria-label={`Remove host ${i + 1}`} onClick={() => patch('server', { hosts: s.hosts.filter((_, j) => j !== i) })}>
                    ×
                  </button>
                )}
              </div>
            ))}
            {(s.type === 'replicaSet' || s.type === 'sharded') && (
              <button type="button" className="small" onClick={() => patch('server', { hosts: [...s.hosts, { host: '', port: 27017 }] })}>
                + Add host
              </button>
            )}
          </div>
          {s.type === 'replicaSet' && (
            <div className="field-row">
              <Field label="Replica set name">
                <input value={s.replicaSet} onChange={(e) => patch('server', { replicaSet: e.target.value })} placeholder="rs0" aria-label="Replica set name" />
              </Field>
            </div>
          )}
          {s.type !== 'srv' && (
            <label className="check">
              <input type="checkbox" checked={s.directConnection} onChange={(e) => patch('server', { directConnection: e.target.checked })} />
              Direct connection (talk to this member only; don't discover the topology)
            </label>
          )}
          <div className="uri-preview">
            {preview.error ? <span className="error-text">{preview.error}</span> : <code>{preview.uri ?? '…'}</code>}
          </div>
        </>
      ) : (
        <>
          <Field
            label="Connection string"
            hint={/^mongodb(\+srv)?:\/\/[^/@]*:[^/@]*@/.test(form.uri) ? 'The password in this string will be moved to the OS credential store and removed from the saved string.' : 'Options in the string apply unless the other tabs set them explicitly.'}
          >
            <textarea
              className="mono"
              rows={3}
              aria-label="Connection string"
              value={form.uri}
              onChange={(e) => set('uri', e.target.value)}
              placeholder="mongodb://host:27017/?replicaSet=rs0 or mongodb+srv://cluster.example.net"
              spellCheck={false}
            />
          </Field>
          <div className="form-actions left">
            <button type="button" className="small" onClick={importURI}>
              Fill in the fields from this string
            </button>
          </div>
          {parseError && <div className="error-text">{parseError}</div>}
        </>
      )}
      {parsed && parsed.unmapped && parsed.unmapped.length > 0 && s.mode === 'manual' && (
        <div className="info-banner small">
          These connection string options have no field and are not kept in manual mode: <code>{parsed.unmapped.join(', ')}</code>. Set them on the Advanced
          tab or stay in connection string mode.
        </div>
      )}
      <label className="check">
        <input type="checkbox" checked={form.readOnly} onChange={(e) => set('readOnly', e.target.checked)} />
        Read-only (block inserts, updates, deletes and drops)
      </label>
    </div>
  )
}

// ---- 2. Authentication ---------------------------------------------------------

export function AuthTab({ form, set, stored, caps }: TabProps) {
  const mechs = caps.authMechanisms ?? []
  const m = form.authMechanism
  const note = mechs.find((c) => c.id === m)?.note
  const needsUser = !['MONGODB-X509', 'MONGODB-OIDC'].includes(m)
  return (
    <div className="tab-panel-body">
      <Field label="Authentication method" hint={note}>
        <select value={m} onChange={(e) => set('authMechanism', e.target.value)} aria-label="Authentication method">
          {mechs.length === 0 && <option value="">Default (SCRAM negotiated)</option>}
          {mechs.map((c) => (
            <option key={c.id || 'default'} value={c.id} disabled={!c.supported}>
              {c.label}
              {c.supported ? '' : ' — not supported'}
            </option>
          ))}
        </select>
      </Field>
      {needsUser && (
        <div className="field-row">
          <Field label={m === 'MONGODB-AWS' ? 'Access key ID (optional)' : 'Username'}>
            <input value={form.username} onChange={(e) => set('username', e.target.value)} autoComplete="off" spellCheck={false} aria-label="Username" />
          </Field>
          <SecretField label={m === 'MONGODB-AWS' ? 'Secret access key' : 'Password'} kind="password" form={form} set={set} stored={stored} />
        </div>
      )}
      {m === 'MONGODB-X509' && (
        <Field label="Certificate subject (optional)" hint="Leave empty to use the subject of the client certificate.">
          <input value={form.username} onChange={(e) => set('username', e.target.value)} spellCheck={false} aria-label="Certificate subject" />
        </Field>
      )}
      <div className="field-row">
        <Field label="Authentication database" hint={['PLAIN', 'MONGODB-X509', 'MONGODB-AWS', 'MONGODB-OIDC'].includes(m) ? 'Uses $external by default.' : 'Usually admin.'}>
          <input value={form.authSource} onChange={(e) => set('authSource', e.target.value)} placeholder={['PLAIN', 'MONGODB-X509', 'MONGODB-AWS', 'MONGODB-OIDC'].includes(m) ? '$external' : 'admin'} spellCheck={false} aria-label="Authentication database" />
        </Field>
      </div>
      {(m === 'MONGODB-OIDC' || m === 'MONGODB-AWS') && (
        <Field label="Mechanism properties" hint={m === 'MONGODB-OIDC' ? 'e.g. ENVIRONMENT:azure,TOKEN_RESOURCE:api://my-app' : 'Optional, e.g. AWS_CREDENTIAL_PROVIDER properties.'}>
          <input className="mono" value={form.authProperties} onChange={(e) => set('authProperties', e.target.value)} spellCheck={false} aria-label="Mechanism properties" />
        </Field>
      )}
      {m === 'MONGODB-AWS' && <SecretField label="Session token" kind="awsSessionToken" form={form} set={set} stored={stored} placeholder="only for temporary credentials" />}
      <Unsupported items={mechs} />
    </div>
  )
}

// ---- 3. SSL / TLS --------------------------------------------------------------

export function TLSTab({ form, set, patch, stored }: TabProps) {
  const t = form.tls
  return (
    <div className="tab-panel-body">
      <label className="check">
        <input type="checkbox" checked={t.enabled} onChange={(e) => patch('tls', { enabled: e.target.checked })} />
        Use TLS / SSL
      </label>
      <fieldset disabled={!t.enabled} className="plain">
        <FileField label="CA certificate file" kind="cert" value={t.caFile} onChange={(v) => patch('tls', { caFile: v })} hint="PEM. Leave empty to trust the operating system's certificate authorities." />
        <FileField label="Client certificate and key (PEM)" kind="cert" value={t.certKeyFile} onChange={(v) => patch('tls', { certKeyFile: v })} hint="One PEM file containing the certificate followed by its private key. Required for X.509 authentication." />
        {t.certKeyFile && <SecretField label="Client key password" kind="tlsKeyPassword" form={form} set={set} stored={stored} placeholder="only if the key is encrypted" />}
        <label className="check">
          <input type="checkbox" checked={t.allowInvalidHostnames} onChange={(e) => patch('tls', { allowInvalidHostnames: e.target.checked })} />
          Allow invalid host names (the certificate chain is still verified)
        </label>
        <label className="check">
          <input type="checkbox" checked={t.allowInvalidCertificates} onChange={(e) => patch('tls', { allowInvalidCertificates: e.target.checked })} />
          Allow invalid certificates <span className="danger-text">— disables verification; for testing only</span>
        </label>
      </fieldset>
      <p className="muted small">Files are checked when you test or connect: missing files, empty CA bundles, expired or not-yet-valid client certificates and encrypted keys without a password are reported before connecting.</p>
    </div>
  )
}

// ---- 4. SSH --------------------------------------------------------------------

export function SSHTab({ form, set, patch, stored, caps }: TabProps) {
  const s = form.ssh
  const methods = caps.sshAuth ?? []
  return (
    <div className="tab-panel-body">
      <label className="check">
        <input type="checkbox" checked={s.enabled} onChange={(e) => patch('ssh', { enabled: e.target.checked, port: s.port || 22, authMethod: s.authMethod || 'password' })} />
        Connect through an SSH tunnel
      </label>
      <fieldset disabled={!s.enabled} className="plain">
        <div className="field-row">
          <Field label="SSH host">
            <input className="mono" value={s.host} onChange={(e) => patch('ssh', { host: e.target.value.trim() })} placeholder="bastion.example.com" aria-label="SSH host" spellCheck={false} />
          </Field>
          <Field label="Port" narrow>
            <NumberInput value={s.port} onChange={(n) => patch('ssh', { port: n })} min={1} max={65535} placeholder="22" label="SSH port" />
          </Field>
          <Field label="SSH username">
            <input value={s.username} onChange={(e) => patch('ssh', { username: e.target.value.trim() })} aria-label="SSH username" autoComplete="off" spellCheck={false} />
          </Field>
        </div>
        <Field label="Authentication">
          <select value={s.authMethod} onChange={(e) => patch('ssh', { authMethod: e.target.value as typeof s.authMethod })} aria-label="SSH authentication">
            {methods.map((c) => (
              <option key={c.id} value={c.id} disabled={!c.supported}>
                {c.label}
                {c.supported ? '' : ' — not supported'}
              </option>
            ))}
          </select>
        </Field>
        {s.authMethod === 'password' && <SecretField label="SSH password" kind="sshPassword" form={form} set={set} stored={stored} />}
        {s.authMethod === 'privateKey' && (
          <>
            <FileField label="Private key file" kind="key" value={s.privateKeyFile} onChange={(v) => patch('ssh', { privateKeyFile: v })} hint="OpenSSH or PEM (RSA, ECDSA, Ed25519)." />
            <SecretField label="Key passphrase" kind="sshPassphrase" form={form} set={set} stored={stored} placeholder="only if the key is protected" />
          </>
        )}
        {s.authMethod === 'agent' && <p className="muted small">{methods.find((m) => m.id === 'agent')?.note}</p>}
        <Field label="Trusted host key" hint="Filled in when you trust the server's key after a test. Keys in ~/.ssh/known_hosts are trusted automatically.">
          <div className="file-row">
            <input className="mono" value={s.hostKey} readOnly placeholder="(not pinned)" aria-label="Trusted host key" />
            {s.hostKey && (
              <button type="button" className="ghost" onClick={() => patch('ssh', { hostKey: '' })}>
                Forget
              </button>
            )}
          </div>
        </Field>
      </fieldset>
      <p className="muted small">MongoDB hosts are resolved and reached from the SSH server, so replica set members and shards discovered by the driver go through the tunnel too.</p>
      <Unsupported items={methods} />
    </div>
  )
}

// ---- 5. Proxy ------------------------------------------------------------------

export function ProxyTab({ form, set, patch, stored, caps }: TabProps) {
  const p = form.proxy
  const types = caps.proxy ?? []
  return (
    <div className="tab-panel-body">
      <Field label="Proxy type">
        <select value={p.type} onChange={(e) => patch('proxy', { type: e.target.value as typeof p.type, port: p.port || (e.target.value === 'socks5' ? 1080 : 8080) })} aria-label="Proxy type">
          {types.length === 0 && <option value="none">None</option>}
          {types.map((c) => (
            <option key={c.id} value={c.id} disabled={!c.supported}>
              {c.label}
              {c.supported ? '' : ' — not supported'}
            </option>
          ))}
        </select>
      </Field>
      {p.type !== 'none' && (
        <>
          <div className="field-row">
            <Field label="Proxy host">
              <input className="mono" value={p.host} onChange={(e) => patch('proxy', { host: e.target.value.trim() })} aria-label="Proxy host" spellCheck={false} />
            </Field>
            <Field label="Port" narrow>
              <NumberInput value={p.port} onChange={(n) => patch('proxy', { port: n })} min={1} max={65535} label="Proxy port" />
            </Field>
          </div>
          <div className="field-row">
            <Field label="Proxy username (optional)">
              <input value={p.username} onChange={(e) => patch('proxy', { username: e.target.value })} aria-label="Proxy username" autoComplete="off" />
            </Field>
            <SecretField label="Proxy password" kind="proxyPassword" form={form} set={set} stored={stored} />
          </div>
          <p className="muted small">{types.find((t) => t.id === p.type)?.note} With an SSH tunnel, the proxy carries the SSH connection.</p>
        </>
      )}
      <Unsupported items={types} />
    </div>
  )
}

// ---- 6. Shell -----------------------------------------------------------

export function ShellTab({ form, patch }: TabProps) {
  const s = form.shell
  return (
    <div className="tab-panel-body">
      <Field label="Default database" hint="Where new shell tabs for this connection start.">
        <input value={s.defaultDb} onChange={(e) => patch('shell', { defaultDb: e.target.value.trim() })} placeholder="test" aria-label="Default database" spellCheck={false} />
      </Field>
      <Field label="Initialization commands" hint="Shell commands, e.g. use shop or db.orders.countDocuments({}). They run in Mango3S's built-in shell interpreter (a mongosh subset: no JavaScript variables, functions or load()).">
        <textarea className="mono" rows={6} value={s.initScript} onChange={(e) => patch('shell', { initScript: e.target.value })} aria-label="Initialization commands" spellCheck={false} />
      </Field>
      <label className="check">
        <input type="checkbox" checked={s.runInitScript} onChange={(e) => patch('shell', { runInitScript: e.target.checked })} />
        Run these commands automatically when a shell tab opens
      </label>
      <p className="muted small">Without automatic running, new shell tabs start with these commands in the editor so you can run them with ⌘/Ctrl+Enter.</p>
    </div>
  )
}

// ---- 7. MongoDB Tools ----------------------------------------------------------

const TOOL_NAMES = ['mongosh', 'mongodump', 'mongorestore', 'mongoexport', 'mongoimport', 'bsondump', 'mongostat', 'mongotop', 'mongofiles']

export function ToolsTab({ form, patch, stored }: TabProps) {
  const t = form.tools
  const [results, setResults] = useState<ToolInfo[] | null>(null)
  const [busy, setBusy] = useState(false)
  const detect = async () => {
    setBusy(true)
    try {
      setResults(await backend.detectTools(t, stored?.id ?? ''))
    } finally {
      setBusy(false)
    }
  }
  const paths = t.paths ?? {}
  return (
    <div className="tab-panel-body">
      <FileField label="Tools folder" kind="folder" value={t.dir} onChange={(v) => patch('tools', { dir: v })} hint="Searched first, then PATH and the usual install locations." />
      <details>
        <summary>Paths for individual tools</summary>
        {TOOL_NAMES.map((n) => (
          <FileField key={n} label={n} kind="tool" value={paths[n] ?? ''} onChange={(v) => patch('tools', { paths: { ...paths, [n]: v } })} />
        ))}
      </details>
      <div className="form-actions left">
        <button type="button" onClick={detect} disabled={busy}>
          {busy && <Spinner small />} Detect installed tools
        </button>
      </div>
      {results && (
        <table className="grid compact tools-table">
          <thead>
            <tr>
              <th>Tool</th>
              <th>Status</th>
              <th>Version</th>
              <th>Path</th>
            </tr>
          </thead>
          <tbody>
            {results.map((r) => (
              <tr key={r.name}>
                <td>
                  <strong>{r.name}</strong>
                  <div className="muted small">{r.purpose}</div>
                </td>
                <td>
                  <span className={`tool-status ${r.status}`}>{r.status}</span>
                  {r.message && <div className="small">{r.message}</div>}
                </td>
                <td className="mono">{r.version || '—'}</td>
                <td className="mono small" title={r.source}>
                  {r.path || '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <p className="muted small">Mango3S's import, export and shell are built in and don't need these tools; the paths are recorded per connection for your own use.</p>
    </div>
  )
}

// ---- 8. Advanced ---------------------------------------------------------------

export function AdvancedTab({ form, set, patch, caps }: TabProps) {
  const a = form.advanced
  const tri = (v: string, on: (v: '' | 'true' | 'false') => void, label: string) => (
    <select value={v} onChange={(e) => on(e.target.value as '' | 'true' | 'false')} aria-label={label}>
      <option value="">default</option>
      <option value="true">on</option>
      <option value="false">off</option>
    </select>
  )
  const toggleComp = (c: string) => patch('advanced', { compressors: a.compressors.includes(c) ? a.compressors.filter((x) => x !== c) : [...a.compressors, c] })
  return (
    <div className="tab-panel-body">
      <h4>Timeouts</h4>
      <div className="field-row">
        <Field label="Connect timeout (s)" narrow>
          <NumberInput value={form.connectTimeoutSeconds} onChange={(n) => set('connectTimeoutSeconds', n)} min={1} max={300} placeholder="10" label="Connect timeout" />
        </Field>
        <Field label="Server selection (s)" narrow>
          <NumberInput value={a.serverSelectionTimeoutSeconds} onChange={(n) => patch('advanced', { serverSelectionTimeoutSeconds: n })} max={600} placeholder="= connect" label="Server selection timeout" />
        </Field>
        <Field label="Operation timeout (s)" narrow hint="timeoutMS for every operation">
          <NumberInput value={a.operationTimeoutSeconds} onChange={(n) => patch('advanced', { operationTimeoutSeconds: n })} max={3600} label="Operation timeout" />
        </Field>
        <Field label="Heartbeat (s)" narrow>
          <NumberInput value={a.heartbeatSeconds} onChange={(n) => patch('advanced', { heartbeatSeconds: n })} max={3600} label="Heartbeat interval" />
        </Field>
      </div>
      <h4>Client</h4>
      <div className="field-row">
        <Field label="Application name" hint="Shown in server logs and currentOp.">
          <input value={a.appName} onChange={(e) => patch('advanced', { appName: e.target.value })} placeholder="Mango3S" aria-label="Application name" />
        </Field>
        <Field label="Local threshold (ms)" narrow>
          <NumberInput value={a.localThresholdMs} onChange={(n) => patch('advanced', { localThresholdMs: n })} max={60000} label="Local threshold" />
        </Field>
      </div>
      <h4>Read and write</h4>
      <div className="field-row">
        <Field label="Read preference">
          <select value={a.readPreference} onChange={(e) => patch('advanced', { readPreference: e.target.value as typeof a.readPreference })} aria-label="Read preference">
            <option value="">default (primary)</option>
            {['primary', 'primaryPreferred', 'secondary', 'secondaryPreferred', 'nearest'].map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Tag set" hint="dc:ny,rack:1">
          <input className="mono" value={a.readPreferenceTags} onChange={(e) => patch('advanced', { readPreferenceTags: e.target.value })} disabled={!a.readPreference || a.readPreference === 'primary'} aria-label="Read preference tags" />
        </Field>
        <Field label="Max staleness (s)" narrow hint="≥ 90">
          <NumberInput value={a.maxStalenessSeconds} onChange={(n) => patch('advanced', { maxStalenessSeconds: n })} label="Max staleness" />
        </Field>
      </div>
      <div className="field-row">
        <Field label="Read concern">
          <select value={a.readConcern} onChange={(e) => patch('advanced', { readConcern: e.target.value })} aria-label="Read concern">
            <option value="">default</option>
            {['local', 'available', 'majority', 'linearizable', 'snapshot'].map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Write concern w" hint="majority, a number, or a tag">
          <input value={a.writeConcernW} onChange={(e) => patch('advanced', { writeConcernW: e.target.value.trim() })} placeholder="default" aria-label="Write concern w" list="wc-values" />
          <datalist id="wc-values">
            <option value="majority" />
            <option value="1" />
            <option value="0" />
          </datalist>
        </Field>
        <Field label="Journal" narrow>
          {tri(a.writeConcernJournal, (v) => patch('advanced', { writeConcernJournal: v }), 'Journal')}
        </Field>
      </div>
      <h4>Connection pool and retries</h4>
      <div className="field-row">
        <Field label="Max pool size" narrow>
          <NumberInput value={a.maxPoolSize} onChange={(n) => patch('advanced', { maxPoolSize: n })} max={10000} placeholder="100" label="Max pool size" />
        </Field>
        <Field label="Min pool size" narrow>
          <NumberInput value={a.minPoolSize} onChange={(n) => patch('advanced', { minPoolSize: n })} max={10000} placeholder="0" label="Min pool size" />
        </Field>
        <Field label="Max connecting" narrow>
          <NumberInput value={a.maxConnecting} onChange={(n) => patch('advanced', { maxConnecting: n })} max={1000} placeholder="2" label="Max connecting" />
        </Field>
        <Field label="Max idle (s)" narrow>
          <NumberInput value={a.maxIdleTimeSeconds} onChange={(n) => patch('advanced', { maxIdleTimeSeconds: n })} max={86400} label="Max idle time" />
        </Field>
        <Field label="Retry writes" narrow>
          {tri(a.retryWrites, (v) => patch('advanced', { retryWrites: v }), 'Retry writes')}
        </Field>
        <Field label="Retry reads" narrow>
          {tri(a.retryReads, (v) => patch('advanced', { retryReads: v }), 'Retry reads')}
        </Field>
      </div>
      <div className="field">
        <span>Compression</span>
        <div className="check-row">
          {['zstd', 'snappy', 'zlib'].map((c) => (
            <label className="check" key={c}>
              <input type="checkbox" checked={a.compressors.includes(c)} onChange={() => toggleComp(c)} />
              {c}
            </label>
          ))}
        </div>
      </div>
      <Unsupported items={caps.advanced ?? []} />
    </div>
  )
}

// ---- 9. Collection History -----------------------------------------------------

export function HistoryTab({ stored, onReopen }: TabProps & { onReopen: (e: CollectionHistoryEntry) => void }) {
  const [list, setList] = useState<CollectionHistoryEntry[] | null>(null)
  const id = stored?.id
  useEffect(() => {
    if (!id) return
    backend.listCollectionHistory(id).then(setList).catch(() => setList([]))
  }, [id])
  if (!id) return <div className="tab-panel-body muted">Save the connection first; collections you open with it are listed here.</div>
  return (
    <div className="tab-panel-body">
      <div className="form-actions left">
        <span className="muted small">Collections opened with this connection, most recent first.</span>
        <span className="spacer" />
        <button
          type="button"
          className="small ghost danger-text"
          disabled={!list?.length}
          onClick={async () => {
            await backend.clearCollectionHistory(id)
            setList([])
          }}
        >
          Clear history
        </button>
      </div>
      {list === null ? (
        <Spinner />
      ) : list.length === 0 ? (
        <p className="muted">No collections opened yet.</p>
      ) : (
        <ul className="history-list">
          {list.map((e) => (
            <li key={`${e.db}.${e.coll}`}>
              <button type="button" className="history-item" onDoubleClick={() => onReopen(e)} onClick={() => onReopen(e)} title="Reopen">
                <span className="mono">
                  {e.db}.{e.coll}
                </span>
                <span className="muted small">
                  {timeAgo(e.lastOpened)} · opened {e.count}×
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
