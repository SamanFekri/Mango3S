import { useEffect, useState } from 'react'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { Profile, ProfileInput, TestResult } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Modal, Spinner } from './common'

const MECHANISMS = ['', 'SCRAM-SHA-256', 'SCRAM-SHA-1', 'MONGODB-X509', 'PLAIN', 'MONGODB-AWS']

function blank(): ProfileInput {
  return {
    id: '',
    name: '',
    uri: 'mongodb://localhost:27017',
    username: '',
    password: '',
    clearPassword: false,
    authSource: '',
    authMechanism: '',
    connectTimeoutSeconds: 10,
    readOnly: false,
  }
}

function fromProfile(p: Profile): ProfileInput {
  return {
    id: p.id,
    name: p.name,
    uri: p.uri,
    username: p.username ?? '',
    password: '',
    clearPassword: false,
    authSource: p.authSource ?? '',
    authMechanism: p.authMechanism ?? '',
    connectTimeoutSeconds: p.connectTimeoutSeconds,
    readOnly: p.readOnly,
  }
}

export function ConnectionManager() {
  const { profiles, reloadProfiles, openConnectionManager, connect, sessions, toast, confirm, disconnect } = useApp()
  const [selected, setSelected] = useState<string | null>(profiles[0]?.id ?? null)
  const [form, setForm] = useState<ProfileInput>(() => (profiles[0] ? fromProfile(profiles[0]) : blank()))
  const [busy, setBusy] = useState<'' | 'test' | 'save' | 'connect'>('')
  const [error, setError] = useState<AppError | null>(null)
  const [test, setTest] = useState<TestResult | null>(null)
  const [dirty, setDirty] = useState(false)

  const current = profiles.find((p) => p.id === selected)
  const close = () => openConnectionManager(false)

  useEffect(() => {
    setError(null)
    setTest(null)
  }, [selected])

  const select = (p: Profile | null) => {
    setSelected(p?.id ?? null)
    setForm(p ? fromProfile(p) : blank())
    setDirty(false)
  }

  const set = <K extends keyof ProfileInput>(k: K, v: ProfileInput[K]) => {
    setForm((f) => ({ ...f, [k]: v }))
    setDirty(true)
    setTest(null)
  }

  const run = async (kind: 'test' | 'save' | 'connect') => {
    setBusy(kind)
    setError(null)
    setTest(null)
    try {
      if (kind === 'test') {
        setTest(await backend.testConnection(form))
        return
      }
      let id = form.id
      if (dirty || !id) {
        const saved = await backend.saveProfile(form)
        await reloadProfiles()
        setSelected(saved.id)
        setForm(fromProfile(saved))
        setDirty(false)
        id = saved.id
        if (sessions.some((s) => s.id === saved.id)) {
          toast({ kind: 'info', title: 'Saved', message: 'Reconnect to apply changes to the open connection.' })
        } else if (kind === 'save') {
          toast({ kind: 'success', title: `Saved "${saved.name}"` })
        }
      }
      if (kind === 'connect') {
        const info = await connect(id)
        if (info) close()
      }
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy('')
    }
  }

  const remove = async () => {
    if (!current) return
    const ok = await confirm({
      title: 'Delete connection',
      message: `Delete the saved connection "${current.name}" and its stored password?`,
      confirmLabel: 'Delete',
      danger: true,
    })
    if (!ok) return
    try {
      if (sessions.some((s) => s.id === current.id)) await disconnect(current.id)
      await backend.deleteProfile(current.id)
      await reloadProfiles()
      select(null)
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const uriHasPassword = /^mongodb(\+srv)?:\/\/[^/@]*:[^/@]*@/.test(form.uri)

  return (
    <Modal title="Connections" onClose={close} wide>
      <div className="conn-manager">
        <aside className="conn-list">
          <button className="primary block" onClick={() => select(null)}>
            + New connection
          </button>
          <ul>
            {profiles.map((p) => (
              <li key={p.id}>
                <button className={p.id === selected ? 'conn-item active' : 'conn-item'} onClick={() => select(p)}>
                  <span className={sessions.some((s) => s.id === p.id) ? 'dot on' : 'dot'} />
                  <span className="conn-name">{p.name}</span>
                  {p.readOnly && <span className="badge">RO</span>}
                </button>
              </li>
            ))}
            {profiles.length === 0 && <li className="muted small pad">No saved connections yet.</li>}
          </ul>
        </aside>

        <form
          className="conn-form"
          onSubmit={(e) => {
            e.preventDefault()
            run('connect')
          }}
        >
          <h3>{form.id ? 'Edit connection' : 'New connection'}</h3>
          <label className="field">
            <span>Name</span>
            <input value={form.name} onChange={(e) => set('name', e.target.value)} placeholder="Local dev" autoFocus required />
          </label>
          <label className="field">
            <span>Connection string</span>
            <input
              className="mono"
              value={form.uri}
              onChange={(e) => set('uri', e.target.value)}
              placeholder="mongodb://host:27017 or mongodb+srv://cluster.example.net"
              spellCheck={false}
              required
            />
            {uriHasPassword && (
              <small className="hint">
                The password in this string will be moved to the {`OS credential store`} and removed from the saved URI.
              </small>
            )}
          </label>
          <div className="field-row">
            <label className="field">
              <span>Username</span>
              <input value={form.username} onChange={(e) => set('username', e.target.value)} autoComplete="off" spellCheck={false} />
            </label>
            <label className="field">
              <span>Password</span>
              <input
                type="password"
                value={form.password}
                onChange={(e) => set('password', e.target.value)}
                placeholder={current?.hasPassword && !form.clearPassword ? '•••••• (saved)' : ''}
                autoComplete="new-password"
              />
            </label>
          </div>
          {current?.hasPassword && (
            <label className="check">
              <input type="checkbox" checked={form.clearPassword} onChange={(e) => set('clearPassword', e.target.checked)} />
              Remove saved password
            </label>
          )}
          <div className="field-row">
            <label className="field">
              <span>Auth database</span>
              <input value={form.authSource} onChange={(e) => set('authSource', e.target.value)} placeholder="admin" spellCheck={false} />
            </label>
            <label className="field">
              <span>Auth mechanism</span>
              <select value={form.authMechanism} onChange={(e) => set('authMechanism', e.target.value)}>
                {MECHANISMS.map((m) => (
                  <option key={m} value={m}>
                    {m || 'Default (negotiate)'}
                  </option>
                ))}
              </select>
            </label>
            <label className="field narrow">
              <span>Timeout (s)</span>
              <input
                type="number"
                min={1}
                max={120}
                value={form.connectTimeoutSeconds}
                onChange={(e) => set('connectTimeoutSeconds', Number(e.target.value))}
              />
            </label>
          </div>
          <label className="check">
            <input type="checkbox" checked={form.readOnly} onChange={(e) => set('readOnly', e.target.checked)} />
            Read-only (block inserts, updates, deletes and drops)
          </label>
          <p className="muted small">
            TLS, replica set, direct connection and other options can be set in the connection string, e.g.{' '}
            <code>?tls=true&amp;replicaSet=rs0</code>.
          </p>

          <ErrorBanner error={error} onDismiss={() => setError(null)} />
          {test && (
            <div className="success-banner" role="status">
              <strong>Connection OK</strong> — MongoDB {test.serverVersion} · {test.topology} · {test.latencyMs} ms
            </div>
          )}

          <div className="form-actions">
            {current && (
              <button type="button" className="danger ghost" onClick={remove} disabled={!!busy}>
                Delete
              </button>
            )}
            <span className="spacer" />
            <button type="button" onClick={() => run('test')} disabled={!!busy}>
              {busy === 'test' ? <Spinner small /> : null} Test
            </button>
            <button type="button" onClick={() => run('save')} disabled={!!busy || (!dirty && !!form.id)}>
              {busy === 'save' ? <Spinner small /> : null} Save
            </button>
            <button type="submit" className="primary" disabled={!!busy}>
              {busy === 'connect' ? <Spinner small /> : null} {dirty || !form.id ? 'Save & connect' : 'Connect'}
            </button>
          </div>
        </form>
      </div>
    </Modal>
  )
}
