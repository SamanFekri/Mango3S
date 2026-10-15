import { useMemo } from 'react'
import {
  BSON_TYPES,
  BuilderError,
  OPERATORS,
  VALUE_TYPES,
  buildFilter,
  buildProjection,
  buildSort,
  uid,
  type BuilderState,
  type Condition,
} from '../lib/queryBuilder'
import { Segmented } from './common'

export interface BuiltQuery {
  filter: string
  sort: string
  projection: string
}

export function QueryBuilder({
  state,
  onChange,
  fields,
  onApply,
}: {
  state: BuilderState
  onChange: (s: BuilderState) => void
  fields: string[]
  onApply: (q: BuiltQuery, run: boolean) => void
}) {
  const listId = useMemo(() => `fields-${uid()}`, [])
  const preview = useMemo(() => {
    try {
      return {
        query: {
          filter: buildFilter(state),
          sort: buildSort(state.sort),
          projection: buildProjection(state.projection, state.projectionMode),
        },
        error: '',
      }
    } catch (e) {
      return { query: null, error: e instanceof BuilderError ? e.message : String(e) }
    }
  }, [state])

  const setCond = (id: string, patch: Partial<Condition>) =>
    onChange({ ...state, conditions: state.conditions.map((c) => (c.id === id ? { ...c, ...patch } : c)) })

  return (
    <div className="query-builder" aria-label="Visual query builder">
      <datalist id={listId}>
        {fields.map((f) => (
          <option key={f} value={f} />
        ))}
      </datalist>

      <div className="qb-section">
        <div className="qb-head">
          <strong>Filter</strong>
          <Segmented
            label="Combine conditions"
            value={state.combinator}
            onChange={(combinator) => onChange({ ...state, combinator })}
            options={[
              { value: 'and', label: 'Match all (AND)' },
              { value: 'or', label: 'Match any (OR)' },
            ]}
          />
        </div>
        {state.conditions.map((c) => {
          const op = OPERATORS.find((o) => o.value === c.op)!
          return (
            <div className="qb-row" key={c.id}>
              <input
                list={listId}
                placeholder="field"
                value={c.field}
                onChange={(e) => setCond(c.id, { field: e.target.value })}
                aria-label="Field"
                spellCheck={false}
              />
              <select value={c.op} onChange={(e) => setCond(c.id, { op: e.target.value as Condition['op'] })} aria-label="Operator">
                {OPERATORS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
              {c.op === 'exists' ? (
                <select value={c.value === 'false' ? 'false' : 'true'} onChange={(e) => setCond(c.id, { value: e.target.value })} aria-label="Exists">
                  <option value="true">true</option>
                  <option value="false">false</option>
                </select>
              ) : c.op === 'type' ? (
                <select value={c.value} onChange={(e) => setCond(c.id, { value: e.target.value })} aria-label="BSON type">
                  <option value="">choose type…</option>
                  {BSON_TYPES.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </select>
              ) : (
                op.needsValue && (
                  <input
                    placeholder={c.op === 'in' || c.op === 'nin' ? 'a, b, c' : c.op === 'regex' ? '^pattern' : 'value'}
                    value={c.value}
                    onChange={(e) => setCond(c.id, { value: e.target.value })}
                    aria-label="Value"
                    spellCheck={false}
                  />
                )
              )}
              {c.op !== 'exists' && c.op !== 'type' && c.op !== 'regex' && (
                <select value={c.type} onChange={(e) => setCond(c.id, { type: e.target.value as Condition['type'] })} aria-label="Value type">
                  {VALUE_TYPES.map((t) => (
                    <option key={t.value} value={t.value}>
                      {t.label}
                    </option>
                  ))}
                </select>
              )}
              <button
                className="icon-btn"
                aria-label="Remove condition"
                onClick={() => onChange({ ...state, conditions: state.conditions.filter((x) => x.id !== c.id) })}
              >
                ×
              </button>
            </div>
          )
        })}
        <button
          className="small ghost"
          onClick={() => onChange({ ...state, conditions: [...state.conditions, { id: uid(), field: '', op: 'eq', value: '', type: 'auto' }] })}
        >
          + Condition
        </button>
      </div>

      <div className="qb-columns">
        <div className="qb-section">
          <strong>Sort</strong>
          {state.sort.map((r) => (
            <div className="qb-row" key={r.id}>
              <input
                list={listId}
                placeholder="field"
                value={r.field}
                aria-label="Sort field"
                onChange={(e) => onChange({ ...state, sort: state.sort.map((x) => (x.id === r.id ? { ...x, field: e.target.value } : x)) })}
              />
              <select
                value={r.dir}
                aria-label="Sort direction"
                onChange={(e) =>
                  onChange({ ...state, sort: state.sort.map((x) => (x.id === r.id ? { ...x, dir: Number(e.target.value) as 1 | -1 } : x)) })
                }
              >
                <option value={1}>ascending</option>
                <option value={-1}>descending</option>
              </select>
              <button className="icon-btn" aria-label="Remove sort" onClick={() => onChange({ ...state, sort: state.sort.filter((x) => x.id !== r.id) })}>
                ×
              </button>
            </div>
          ))}
          <button className="small ghost" onClick={() => onChange({ ...state, sort: [...state.sort, { id: uid(), field: '', dir: 1 }] })}>
            + Sort field
          </button>
        </div>

        <div className="qb-section">
          <div className="qb-head">
            <strong>Projection</strong>
            <Segmented
              label="Projection mode"
              value={state.projectionMode}
              onChange={(projectionMode) => onChange({ ...state, projectionMode })}
              options={[
                { value: 'include', label: 'Include' },
                { value: 'exclude', label: 'Exclude' },
              ]}
            />
          </div>
          {state.projection.map((f, i) => (
            <div className="qb-row" key={i}>
              <input
                list={listId}
                placeholder="field"
                value={f}
                aria-label="Projection field"
                onChange={(e) => onChange({ ...state, projection: state.projection.map((x, j) => (j === i ? e.target.value : x)) })}
              />
              <button
                className="icon-btn"
                aria-label="Remove projection field"
                onClick={() => onChange({ ...state, projection: state.projection.filter((_, j) => j !== i) })}
              >
                ×
              </button>
            </div>
          ))}
          <button className="small ghost" onClick={() => onChange({ ...state, projection: [...state.projection, ''] })}>
            + Field
          </button>
        </div>
      </div>

      <div className="qb-footer">
        {preview.error ? (
          <span className="error-text">{preview.error}</span>
        ) : (
          <code className="qb-preview" title="Generated filter">
            {preview.query!.filter}
          </code>
        )}
        <span className="spacer" />
        <button disabled={!preview.query} onClick={() => preview.query && onApply(preview.query, false)}>
          Apply to editor
        </button>
        <button className="primary" disabled={!preview.query} onClick={() => preview.query && onApply(preview.query, true)}>
          Apply &amp; run
        </button>
      </div>
    </div>
  )
}
