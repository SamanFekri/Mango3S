import { useEffect, useState } from 'react'
import { ErrorBanner, Spinner } from '../../components/common'
import { PasswordInput } from '../../components/PasswordInput'
import { backend } from '../../lib/backend'
import { toAppError, type AppError } from '../../lib/errors'
import type { DbRole, DbUser, UserEffective } from '../../lib/types'
import { useApp } from '../../state/AppContext'
import { RolePicker } from './RolePicker'
import { generatePassword, groupPrivileges, roleKey } from './roles'

/** Asks to confirm a user change, listing the server-side warnings (losing
 * your own admin rights, removing the last admin). */
async function confirmWarnings(confirm: ReturnType<typeof useApp>['confirm'], warnings: string[], label: string, typeToConfirm?: string) {
  if (warnings.length === 0 && !typeToConfirm) return true
  return confirm({
    title: warnings.length ? 'Are you sure?' : label,
    message: (
      <>
        {warnings.map((w) => (
          <p key={w}>{w}</p>
        ))}
        {typeToConfirm && <p>This cannot be undone.</p>}
      </>
    ),
    confirmLabel: label,
    danger: true,
    typeToConfirm,
  })
}

/** One user: its roles, what it can do, its password, and dropping it. */
export function UserPanel({ connId, readOnly, user, customRoles, onChanged }: { connId: string; readOnly: boolean; user: DbUser; customRoles: string[]; onChanged: () => void }) {
  const { confirm, toast } = useApp()
  const [roles, setRoles] = useState<DbRole[]>(user.roles)
  const [effective, setEffective] = useState<UserEffective | 'loading' | null>(null)
  const [check, setCheck] = useState({ db: user.db === 'admin' ? '' : user.db, coll: '', action: 'find' })
  const [checked, setChecked] = useState<{ q: string; missing: string[] } | null>(null)
  const [pwd, setPwd] = useState('')
  const [pwd2, setPwd2] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    setRoles(user.roles)
    setEffective(null)
    setChecked(null)
    setPwd('')
    setPwd2('')
    setError(null)
  }, [user])

  const dirty = roles.map(roleKey).sort().join() !== user.roles.map(roleKey).sort().join()
  const guard = async (f: () => Promise<void>) => {
    setBusy(true)
    setError(null)
    try {
      await f()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  const saveRoles = () =>
    guard(async () => {
      const warnings = await backend.checkUserChange({ connId, db: user.db, user: user.user, drop: false, roles })
      if (!(await confirmWarnings(confirm, warnings, 'Change roles'))) return
      await backend.setUserRoles(connId, user.db, user.user, roles)
      toast({ kind: 'success', title: `Roles of ${user.user} changed` })
      onChanged()
    })

  const loadEffective = async () => {
    setEffective('loading')
    try {
      setEffective(await backend.userEffective(connId, user.db, user.user))
    } catch (err) {
      setEffective(null)
      setError(toAppError(err))
    }
  }

  const runCheck = () =>
    guard(async () => {
      const missing = await backend.userCan(connId, user.db, user.user, check.db, check.coll, [check.action.trim()])
      setChecked({ q: `${check.action} on ${check.db || 'any database'}${check.coll ? `.${check.coll}` : ''}`, missing })
    })

  const changePassword = () =>
    guard(async () => {
      await backend.setUserPassword(connId, user.db, user.user, pwd)
      setPwd('')
      setPwd2('')
      toast({ kind: 'success', title: `Password of ${user.user} changed`, message: user.self ? 'Edit this connection to use the new password next time.' : undefined })
    })

  const drop = () =>
    guard(async () => {
      const warnings = await backend.checkUserChange({ connId, db: user.db, user: user.user, drop: true, roles: [] })
      if (!(await confirmWarnings(confirm, warnings, 'Drop user', user.user))) return
      await backend.dropUser(connId, user.db, user.user, user.user)
      toast({ kind: 'success', title: `Dropped ${user.user}@${user.db}` })
      onChanged()
    })

  return (
    <aside className="users-detail" aria-label={`User ${user.user}`}>
      <h3>
        {user.user}
        <span className="muted">@{user.db}</span> {user.self && <span className="badge">you</span>}
      </h3>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />

      <section>
        <h4>Roles</h4>
        <RolePicker db={user.db} value={roles} onChange={setRoles} custom={customRoles} disabled={readOnly} />
        {!readOnly && (
          <div className="field-row">
            <button className="primary small" disabled={!dirty || busy} onClick={() => void saveRoles()}>
              Save roles
            </button>
            {dirty && (
              <button className="small ghost" onClick={() => setRoles(user.roles)}>
                Undo
              </button>
            )}
          </div>
        )}
      </section>

      <section>
        <h4>What can this user do?</h4>
        {effective === null && (
          <button className="small" onClick={() => void loadEffective()}>
            Show all privileges
          </button>
        )}
        {effective === 'loading' && <Spinner small />}
        {effective && effective !== 'loading' && (
          <>
            <p className="small muted">Roles, inherited ones included:</p>
            <div className="chip-list">
              {effective.roles.map((r) => (
                <span key={roleKey(r)} className="chip mono">
                  {roleKey(r)}
                </span>
              ))}
            </div>
            <table className="grid compact small" aria-label="Privileges">
              <tbody>
                {groupPrivileges(effective.privileges).map((p) => (
                  <tr key={p.label}>
                    <td>{p.label}</td>
                    <td className="mono">{p.actions.join(', ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
        <div className="field-row">
          <label className="field">
            <span>Action</span>
            <input className="mono" value={check.action} onChange={(e) => setCheck({ ...check, action: e.target.value })} aria-label="Action" />
          </label>
          <label className="field">
            <span>Database</span>
            <input className="mono" value={check.db} onChange={(e) => setCheck({ ...check, db: e.target.value })} aria-label="Database to check" />
          </label>
          <label className="field">
            <span>Collection</span>
            <input className="mono" value={check.coll} onChange={(e) => setCheck({ ...check, coll: e.target.value })} aria-label="Collection to check" />
          </label>
          <button className="small" disabled={!check.action.trim() || busy} onClick={() => void runCheck()}>
            Check
          </button>
        </div>
        {checked && (
          <p className={checked.missing.length ? 'error-text' : 'success-text'} role="status">
            {checked.missing.length ? `✗ Not allowed: ${checked.q}` : `✓ Allowed: ${checked.q}`}
          </p>
        )}
      </section>

      {!readOnly && (
        <section>
          <h4>Password</h4>
          <div className="field-row">
            <PasswordInput value={pwd} onChange={(e) => setPwd(e.target.value)} placeholder="New password" aria-label="New password" autoComplete="new-password" />
            <PasswordInput value={pwd2} onChange={(e) => setPwd2(e.target.value)} placeholder="Repeat" aria-label="Repeat password" autoComplete="new-password" />
            <button
              className="small ghost"
              onClick={() => {
                const p = generatePassword()
                setPwd(p)
                setPwd2(p)
                void navigator.clipboard?.writeText(p)
                toast({ kind: 'info', title: 'Password generated and copied' })
              }}
            >
              Generate
            </button>
            <button className="small" disabled={!pwd || pwd !== pwd2 || busy} onClick={() => void changePassword()}>
              Change password
            </button>
          </div>
          {pwd2 && pwd !== pwd2 && <p className="error-text small">The passwords differ.</p>}
        </section>
      )}

      {!readOnly && (
        <section>
          <button className="small danger" disabled={busy} onClick={() => void drop()}>
            Drop user…
          </button>
        </section>
      )}
    </aside>
  )
}
