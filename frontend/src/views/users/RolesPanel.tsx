import { useEffect, useState } from 'react'
import { ErrorBanner } from '../../components/common'
import { backend } from '../../lib/backend'
import { toAppError, type AppError } from '../../lib/errors'
import type { DbRole, Privilege, RoleInfo } from '../../lib/types'
import { useApp } from '../../state/AppContext'
import { RolePicker } from './RolePicker'
import { CLUSTER_ACTIONS, COLLECTION_ACTIONS, groupPrivileges } from './roles'

type ResourceKind = 'collection' | 'database' | 'cluster'
interface Row {
  kind: ResourceKind
  db: string
  coll: string
  actions: string[]
}

const toRow = (p: Privilege): Row => ({
  kind: p.resource.cluster ? 'cluster' : p.resource.collection ? 'collection' : 'database',
  db: p.resource.db ?? '',
  coll: p.resource.collection ?? '',
  actions: p.actions,
})
const toPrivilege = (r: Row): Privilege => ({
  resource: r.kind === 'cluster' ? { cluster: true } : { db: r.db, collection: r.kind === 'collection' ? r.coll : '' },
  actions: r.actions,
})

/** A custom role's editor: a privilege matrix (one row per resource, a
 * checkbox per action) and the roles it inherits. A built-in role is shown
 * read-only. `role` null creates a new role. */
export function RoleEditor({ connId, db, role, readOnly, onSaved }: { connId: string; db: string; role: RoleInfo | null; readOnly: boolean; onSaved: (name: string | null) => void }) {
  const { confirm, toast } = useApp()
  const [name, setName] = useState(role?.role ?? '')
  const [rows, setRows] = useState<Row[]>(role ? role.privileges.map(toRow) : [{ kind: 'collection', db, coll: '', actions: ['find'] }])
  const [inherits, setInherits] = useState<DbRole[]>(role?.roles ?? [])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    setName(role?.role ?? '')
    setRows(role ? role.privileges.map(toRow) : [{ kind: 'collection', db, coll: '', actions: ['find'] }])
    setInherits(role?.roles ?? [])
    setError(null)
  }, [role, db])

  if (role?.isBuiltin) {
    return (
      <aside className="users-detail" aria-label={`Role ${role.role}`}>
        <h3>
          {role.role}
          <span className="muted">@{role.db}</span> <span className="badge">built-in</span>
        </h3>
        <table className="grid compact small" aria-label="Privileges">
          <tbody>
            {groupPrivileges(role.privileges).map((p) => (
              <tr key={p.label}>
                <td>{p.label}</td>
                <td className="mono">{p.actions.join(', ')}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </aside>
    )
  }

  const edit = (i: number, patch: Partial<Row>) => setRows((rs) => rs.map((r, j) => (j === i ? { ...r, ...patch } : r)))
  const toggle = (i: number, a: string, on: boolean) => edit(i, { actions: on ? [...rows[i].actions, a] : rows[i].actions.filter((x) => x !== a) })
  const locked = readOnly || busy

  const save = async () => {
    setBusy(true)
    setError(null)
    try {
      await backend.saveRole({ connId, db, role: name.trim(), privileges: rows.map(toPrivilege), roles: inherits }, !role)
      toast({ kind: 'success', title: role ? `Role ${name} saved` : `Role ${name} created` })
      onSaved(name.trim())
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  const drop = async () => {
    if (!role) return
    const ok = await confirm({ title: 'Drop role', message: <>Drop the role <code>{role.role}</code>? Users that hold it lose its privileges.</>, confirmLabel: 'Drop role', danger: true, typeToConfirm: role.role })
    if (!ok) return
    try {
      await backend.dropRole(connId, db, role.role, role.role)
      toast({ kind: 'success', title: `Dropped role ${role.role}` })
      onSaved(null)
    } catch (err) {
      setError(toAppError(err))
    }
  }

  return (
    <aside className="users-detail" aria-label={role ? `Role ${role.role}` : 'New role'}>
      {role ? (
        <h3>
          {role.role}
          <span className="muted">@{db}</span>
        </h3>
      ) : (
        <label className="field">
          <span>New role in {db}</span>
          <input className="mono" value={name} onChange={(e) => setName(e.target.value)} placeholder="reporter" aria-label="Role name" autoFocus />
        </label>
      )}
      <ErrorBanner error={error} onDismiss={() => setError(null)} />

      <h4>Privileges</h4>
      <table className="grid compact privilege-matrix" aria-label="Privilege matrix">
        <tbody>
          {rows.map((r, i) => {
            const offered = r.kind === 'cluster' ? CLUSTER_ACTIONS : COLLECTION_ACTIONS
            const extra = r.actions.filter((a) => !offered.includes(a))
            return (
              <tr key={i}>
                <td>
                  <select value={r.kind} disabled={locked} onChange={(e) => edit(i, { kind: e.target.value as ResourceKind, actions: [] })} aria-label="Resource">
                    <option value="collection">Collection</option>
                    <option value="database">Every collection of a database</option>
                    <option value="cluster">Cluster</option>
                  </select>
                  {r.kind !== 'cluster' && (
                    <input className="mono" value={r.db} disabled={locked} onChange={(e) => edit(i, { db: e.target.value })} placeholder="any database" aria-label="Database" />
                  )}
                  {r.kind === 'collection' && (
                    <input className="mono" value={r.coll} disabled={locked} onChange={(e) => edit(i, { coll: e.target.value })} placeholder="collection" aria-label="Collection" />
                  )}
                </td>
                <td className="matrix-actions">
                  {offered.map((a) => (
                    <label key={a} className="check small">
                      <input type="checkbox" checked={r.actions.includes(a)} disabled={locked} onChange={(e) => toggle(i, a, e.target.checked)} />
                      <span className="mono">{a}</span>
                    </label>
                  ))}
                  <input
                    className="mono small"
                    value={extra.join(', ')}
                    disabled={locked}
                    onChange={(e) =>
                      edit(i, {
                        actions: [
                          ...r.actions.filter((a) => offered.includes(a)),
                          ...e.target.value
                            .split(',')
                            .map((a) => a.trim())
                            .filter(Boolean),
                        ],
                      })
                    }
                    placeholder="other actions, comma-separated"
                    aria-label="Other actions"
                  />
                </td>
                <td>
                  {!locked && (
                    <button className="icon-btn" aria-label="Remove privilege" onClick={() => setRows((rs) => rs.filter((_, j) => j !== i))}>
                      ×
                    </button>
                  )}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
      {!locked && (
        <button className="small ghost" onClick={() => setRows((rs) => [...rs, { kind: 'collection', db, coll: '', actions: [] }])}>
          + Add privilege
        </button>
      )}

      <h4>Inherits roles</h4>
      <RolePicker db={db} value={inherits} onChange={setInherits} disabled={locked} />

      {!readOnly && (
        <div className="field-row">
          <button className="primary small" disabled={busy || !name.trim()} onClick={() => void save()}>
            {role ? 'Save role' : 'Create role'}
          </button>
          {role && (
            <button className="small danger" disabled={busy} onClick={() => void drop()}>
              Drop role…
            </button>
          )}
        </div>
      )}
    </aside>
  )
}
