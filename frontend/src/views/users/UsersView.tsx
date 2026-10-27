import { useCallback, useEffect, useState } from 'react'
import { ErrorBanner, Modal, Segmented, Spinner } from '../../components/common'
import { PasswordInput } from '../../components/PasswordInput'
import { backend } from '../../lib/backend'
import { toAppError, type AppError } from '../../lib/errors'
import type { Tab } from '../../lib/tabs'
import type { DbRole, DbUser, RoleInfo } from '../../lib/types'
import { useApp } from '../../state/AppContext'
import { RoleEditor } from './RolesPanel'
import { RolePicker } from './RolePicker'
import { UserPanel } from './UserPanel'
import { generatePassword, roleKey } from './roles'

/** Users and roles of a connection (Explorer › connection › Users and roles…). */
export function UsersView({ tab }: { tab: Tab }) {
  const connId = tab.connId!
  const { sessions } = useApp()
  const readOnly = !!sessions.find((s) => s.id === connId)?.readOnly
  const [section, setSection] = useState<'users' | 'roles'>('users')
  const [dbs, setDbs] = useState<string[]>(['admin'])
  const [db, setDb] = useState('') // users: '' = every database; roles need one
  const [users, setUsers] = useState<DbUser[] | null>(null)
  const [roles, setRoles] = useState<RoleInfo[] | null>(null)
  const [builtin, setBuiltin] = useState(false)
  const [selected, setSelected] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const roleDb = db || 'admin'

  useEffect(() => {
    backend
      .listDatabases(connId)
      .then((list) => setDbs([...new Set(['admin', ...list.map((d) => d.name)])].sort()))
      .catch(() => undefined)
  }, [connId])

  const load = useCallback(async () => {
    setError(null)
    try {
      if (section === 'users') setUsers(await backend.listUsers(connId, db))
      else setRoles(await backend.listRoles(connId, roleDb, builtin))
    } catch (err) {
      setError(toAppError(err))
      if (section === 'users') setUsers([])
      else setRoles([])
    }
  }, [connId, db, roleDb, section, builtin])

  useEffect(() => {
    setSelected(null)
    void load()
  }, [load])

  // Custom roles of the selected user's database, offered as checkboxes.
  const [customOf, setCustomOf] = useState<Record<string, string[]>>({})
  const user = users?.find((u) => `${u.user}@${u.db}` === selected) ?? null
  useEffect(() => {
    if (!user || customOf[user.db]) return
    backend
      .listRoles(connId, user.db, false)
      .then((r) => setCustomOf((c) => ({ ...c, [user.db]: r.map((x) => x.role) })))
      .catch(() => setCustomOf((c) => ({ ...c, [user.db]: [] })))
  }, [user, connId, customOf])

  const role = roles?.find((r) => r.role === selected) ?? null

  return (
    <div className="view users-view">
      <div className="view-header">
        <div className="breadcrumbs">
          <span>{tab.connName}</span>
          <span className="muted">· users and roles</span>
          {readOnly && <span className="badge">read-only</span>}
        </div>
        <span className="spacer" />
        <Segmented
          label="Show"
          value={section}
          onChange={(s) => {
            setSection(s)
            setCreating(false)
          }}
          options={[
            { value: 'users', label: 'Users' },
            { value: 'roles', label: 'Roles' },
          ]}
        />
        <label className="inline">
          Database
          <select value={section === 'roles' ? roleDb : db} onChange={(e) => setDb(e.target.value)} aria-label="Database">
            {section === 'users' && <option value="">All databases</option>}
            {dbs.map((d) => (
              <option key={d} value={d}>
                {d}
              </option>
            ))}
          </select>
        </label>
        {section === 'roles' && (
          <label className="check small">
            <input type="checkbox" checked={builtin} onChange={(e) => setBuiltin(e.target.checked)} />
            Built-in roles
          </label>
        )}
        <button className="small ghost" onClick={() => void load()}>
          Refresh
        </button>
        {!readOnly && (
          <button
            className="small primary"
            onClick={() => {
              setSelected(null)
              setCreating(true)
            }}
          >
            {section === 'users' ? 'New user…' : 'New role…'}
          </button>
        )}
      </div>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />

      <div className="users-body">
        <div className="table-wrap">
          {section === 'users' ? (
            users === null ? (
              <Spinner />
            ) : users.length === 0 ? (
              <div className="empty-state">{db ? `No users are defined in ${db}.` : 'No users yet. The server may run without authentication.'}</div>
            ) : (
              <table className="grid compact" aria-label="Users">
                <thead>
                  <tr>
                    <th>User</th>
                    <th>Database</th>
                    <th>Roles</th>
                  </tr>
                </thead>
                <tbody>
                  {users.map((u) => {
                    const k = `${u.user}@${u.db}`
                    return (
                      <tr key={k} className={selected === k ? 'selected clickable' : 'clickable'} onClick={() => (setSelected(k), setCreating(false))}>
                        <td>
                          {u.user} {u.self && <span className="badge">you</span>}
                        </td>
                        <td>{u.db}</td>
                        <td className="mono small">{u.roles.map(roleKey).join(', ')}</td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )
          ) : roles === null ? (
            <Spinner />
          ) : roles.length === 0 ? (
            <div className="empty-state">No custom roles in {roleDb}. Tick “Built-in roles” to see MongoDB's own.</div>
          ) : (
            <table className="grid compact" aria-label="Roles">
              <thead>
                <tr>
                  <th>Role</th>
                  <th>Inherits</th>
                  <th>Privileges</th>
                </tr>
              </thead>
              <tbody>
                {roles.map((r) => (
                  <tr key={r.role} className={selected === r.role ? 'selected clickable' : 'clickable'} onClick={() => (setSelected(r.role), setCreating(false))}>
                    <td>
                      {r.role} {r.isBuiltin && <span className="badge">built-in</span>}
                    </td>
                    <td className="mono small">{r.roles.map(roleKey).join(', ')}</td>
                    <td className="small">{r.privileges.length}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {section === 'users' && user && !creating && (
          <UserPanel connId={connId} readOnly={readOnly} user={user} customRoles={customOf[user.db] ?? []} onChanged={() => void load()} />
        )}
        {section === 'roles' && (creating || role) && (
          <RoleEditor
            connId={connId}
            db={roleDb}
            role={creating ? null : role}
            readOnly={readOnly}
            onSaved={(name) => {
              setCreating(false)
              void load().then(() => setSelected(name))
            }}
          />
        )}
      </div>

      {section === 'users' && creating && (
        <NewUserDialog
          connId={connId}
          dbs={dbs}
          initialDb={db || 'admin'}
          onClose={() => setCreating(false)}
          onCreated={(k) => {
            setCreating(false)
            void load().then(() => setSelected(k))
          }}
        />
      )}
    </div>
  )
}

function NewUserDialog({ connId, dbs, initialDb, onClose, onCreated }: { connId: string; dbs: string[]; initialDb: string; onClose: () => void; onCreated: (key: string) => void }) {
  const { toast } = useApp()
  const [db, setDb] = useState(initialDb)
  const [name, setName] = useState('')
  const [pwd, setPwd] = useState('')
  const [pwd2, setPwd2] = useState('')
  const [roles, setRoles] = useState<DbRole[]>(initialDb === 'admin' ? [] : [{ role: 'read', db: initialDb }])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  const create = async () => {
    setBusy(true)
    setError(null)
    try {
      await backend.createUser({ connId, db, user: name.trim(), password: pwd, roles })
      toast({ kind: 'success', title: `Created ${name.trim()}@${db}` })
      onCreated(`${name.trim()}@${db}`)
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title="New user"
      onClose={onClose}
      wide
      footer={
        <>
          <button onClick={onClose}>Cancel</button>
          <button className="primary" disabled={busy || !name.trim() || !pwd || pwd !== pwd2} onClick={() => void create()}>
            {busy && <Spinner small />} Create user
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <div className="field-row">
        <label className="field">
          <span>User name</span>
          <input value={name} onChange={(e) => setName(e.target.value)} autoFocus aria-label="User name" autoComplete="off" spellCheck={false} />
        </label>
        <label className="field">
          <span>Authentication database</span>
          <select value={db} onChange={(e) => setDb(e.target.value)} aria-label="Authentication database">
            {dbs.map((d) => (
              <option key={d} value={d}>
                {d}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="field-row">
        <label className="field">
          <span>Password</span>
          <PasswordInput value={pwd} onChange={(e) => setPwd(e.target.value)} aria-label="Password" autoComplete="new-password" />
        </label>
        <label className="field">
          <span>Repeat password</span>
          <PasswordInput value={pwd2} onChange={(e) => setPwd2(e.target.value)} aria-label="Repeat password" autoComplete="new-password" />
        </label>
        <button
          className="small ghost"
          onClick={() => {
            const p = generatePassword()
            setPwd(p)
            setPwd2(p)
            void navigator.clipboard?.writeText(p)
            toast({ kind: 'info', title: 'Password generated and copied', message: 'Keep it somewhere safe: Mango3S does not store it.' })
          }}
        >
          Generate
        </button>
      </div>
      {pwd2 && pwd !== pwd2 && <p className="error-text small">The passwords differ.</p>}
      <h4>Roles</h4>
      <RolePicker db={db} value={roles} onChange={setRoles} />
    </Modal>
  )
}
