import { useCallback, useEffect, useState } from 'react'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { SearchIndexList, SessionInfo } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Spinner } from './common'

export const SEARCH_TEMPLATES = {
  search: '{\n  mappings: { dynamic: true }\n}',
  vectorSearch: '{\n  fields: [\n    { type: "vector", path: "embedding", numDimensions: 1536, similarity: "cosine" }\n  ]\n}',
}
type SearchType = keyof typeof SEARCH_TEMPLATES

/** Atlas Search and Vector Search indexes of a collection (Collection info ›
 * Indexes). They build in the background, so the list refreshes until every
 * index is ready. */
export function SearchIndexes({ conn, db, coll }: { conn: SessionInfo; db: string; coll: string }) {
  const { confirm, toast } = useApp()
  const [list, setList] = useState<SearchIndexList | null>(null)
  const [form, setForm] = useState<{ name: string; type: SearchType; definition: string; editing: boolean } | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  const load = useCallback(() => {
    backend
      .listSearchIndexes(conn.id, db, coll)
      .then(setList)
      .catch((err) => setError(toAppError(err)))
  }, [conn.id, db, coll])
  useEffect(load, [load])

  const building = !!list?.indexes.some((i) => i.status !== 'READY' && i.status !== 'FAILED')
  useEffect(() => {
    if (!building) return
    const id = window.setInterval(load, 3000)
    return () => window.clearInterval(id)
  }, [building, load])

  if (!list) return <Spinner small />
  if (!list.supported) {
    return (
      <section className="search-indexes">
        <h4>Search indexes</h4>
        <p className="muted small">{list.reason}</p>
      </section>
    )
  }

  const save = async () => {
    if (!form) return
    setBusy(true)
    setError(null)
    try {
      if (form.editing) await backend.updateSearchIndex(conn.id, db, coll, form.name, form.definition)
      else await backend.createSearchIndex(conn.id, db, coll, { name: form.name, type: form.type, definition: form.definition })
      toast({ kind: 'success', title: form.editing ? `Updating ${form.name}` : 'Search index created', message: 'It builds in the background; the status updates here.' })
      setForm(null)
      load()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  const drop = async (name: string) => {
    const ok = await confirm({ title: 'Drop search index', message: <>Drop the search index <code>{name}</code>? Queries that use it stop working.</>, confirmLabel: 'Drop', danger: true, typeToConfirm: name })
    if (!ok) return
    try {
      await backend.dropSearchIndex(conn.id, db, coll, name)
      toast({ kind: 'success', title: `Dropping ${name}` })
      load()
    } catch (err) {
      setError(toAppError(err))
    }
  }

  return (
    <section className="search-indexes">
      <h4>Search indexes {building && <Spinner small />}</h4>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {list.indexes.length === 0 ? (
        <p className="muted small">No Atlas Search or Vector Search indexes yet.</p>
      ) : (
        <table className="grid compact" aria-label="Search indexes">
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {list.indexes.map((ix) => (
              <tr key={ix.name}>
                <td className="mono">{ix.name}</td>
                <td>{ix.type === 'vectorSearch' ? 'Vector Search' : 'Search'}</td>
                <td title={ix.message}>
                  <span className={ix.status === 'FAILED' ? 'danger-text' : ix.queryable ? 'success-text' : 'muted'}>{ix.status.toLowerCase()}</span>
                  {ix.queryable && ix.status !== 'READY' && <span className="muted small"> (old version queryable)</span>}
                </td>
                <td>
                  {!conn.readOnly && (
                    <>
                      <button className="small ghost" onClick={() => setForm({ name: ix.name, type: ix.type as SearchType, definition: ix.definition, editing: true })}>
                        Edit
                      </button>
                      <button className="small ghost danger" onClick={() => void drop(ix.name)}>
                        Drop
                      </button>
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {!conn.readOnly && !form && (
        <button className="small" onClick={() => setForm({ name: list.indexes.length ? '' : 'default', type: 'search', definition: SEARCH_TEMPLATES.search, editing: false })}>
          New search index…
        </button>
      )}
      {form && (
        <div className="search-index-form">
          <div className="field-row">
            <label className="field">
              <span>Name</span>
              <input className="mono" value={form.name} disabled={form.editing} onChange={(e) => setForm({ ...form, name: e.target.value })} aria-label="Search index name" />
            </label>
            <label className="field">
              <span>Type</span>
              <select
                value={form.type}
                disabled={form.editing}
                aria-label="Search index type"
                onChange={(e) => {
                  const type = e.target.value as SearchType
                  setForm({ ...form, type, definition: form.definition === SEARCH_TEMPLATES[form.type] ? SEARCH_TEMPLATES[type] : form.definition })
                }}
              >
                <option value="search">Atlas Search (full text, $search)</option>
                <option value="vectorSearch">Vector Search ($vectorSearch)</option>
              </select>
            </label>
          </div>
          <label className="field">
            <span>Definition</span>
            <textarea className="mono" rows={8} value={form.definition} onChange={(e) => setForm({ ...form, definition: e.target.value })} aria-label="Search index definition" spellCheck={false} />
          </label>
          <div className="field-row">
            <button className="primary small" disabled={busy || (!form.editing && !form.name.trim())} onClick={() => void save()}>
              {form.editing ? 'Save definition' : 'Create'}
            </button>
            <button className="small" onClick={() => setForm(null)}>
              Cancel
            </button>
          </div>
        </div>
      )}
    </section>
  )
}
