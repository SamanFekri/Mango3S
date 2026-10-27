import { useState } from 'react'
import type { DbRole } from '../../lib/types'
import { ADMIN_ROLES, DB_ROLES, hasRole, parseRole, roleKey, sameRole } from './roles'

/** Roles as checkboxes (the built-in roles of `db`, and the deployment-wide
 * ones when db is admin), plus chips for any other role with a field to add
 * one as "role@db". */
export function RolePicker({
  db,
  value,
  onChange,
  custom = [],
  disabled,
}: {
  db: string
  value: DbRole[]
  onChange: (roles: DbRole[]) => void
  /** Custom roles of db, offered as checkboxes too. */
  custom?: string[]
  disabled?: boolean
}) {
  const [other, setOther] = useState('')
  const toggle = (r: DbRole, on: boolean) => onChange(on ? [...value, r] : value.filter((x) => !sameRole(x, r)))
  const boxes = [...DB_ROLES, ...custom, ...(db === 'admin' ? ADMIN_ROLES : [])].map((role) => ({ role, db }))
  const rest = value.filter((r) => !hasRole(boxes, r))
  const add = () => {
    const r = parseRole(other, db)
    if (r && !hasRole(value, r)) onChange([...value, r])
    setOther('')
  }
  return (
    <div className="role-picker">
      <div className="role-boxes">
        {boxes.map((r) => (
          <label key={roleKey(r)} className="check small">
            <input type="checkbox" checked={hasRole(value, r)} disabled={disabled} onChange={(e) => toggle(r, e.target.checked)} />
            <span className="mono">{r.role}</span>
          </label>
        ))}
      </div>
      <div className="chip-list">
        {rest.map((r) => (
          <span key={roleKey(r)} className="chip mono">
            {roleKey(r)}
            {!disabled && (
              <button className="icon-btn" aria-label={`Remove ${roleKey(r)}`} onClick={() => toggle(r, false)}>
                ×
              </button>
            )}
          </span>
        ))}
      </div>
      {!disabled && (
        <div className="file-row">
          <input
            className="mono"
            value={other}
            onChange={(e) => setOther(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && add()}
            placeholder={`role@database, e.g. read@${db === 'admin' ? 'shop' : db}`}
            aria-label="Add a role"
            spellCheck={false}
          />
          <button className="small" onClick={add} disabled={!other.trim()}>
            Add role
          </button>
        </div>
      )}
    </div>
  )
}
