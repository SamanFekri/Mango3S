import { useState } from 'react'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { CompareOptions, SchemaDiff, SchemaResult } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Spinner } from './common'

const LABELS: Record<string, string> = {
  createIndex: 'Create index',
  replaceIndex: 'Rebuild index',
  dropIndex: 'Drop index',
  validator: 'Set validation rules',
}

/** Indexes and validation rules of two collections: what differs, and copying
 * the chosen changes to the target (Compare & sync › Collections). */
export function SchemaSync({ options }: { options: CompareOptions }) {
  const { toast } = useApp()
  const [diff, setDiff] = useState<SchemaDiff | null>(null)
  const [chosen, setChosen] = useState<Set<string>>(new Set())
  const [confirm, setConfirm] = useState('')
  const [result, setResult] = useState<SchemaResult | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const ready = !!options.source.coll && !!options.target.coll && !!options.target.db
  const target = `${options.target.db}.${options.target.coll}`

  const load = async () => {
    setBusy(true)
    setError(null)
    setResult(null)
    try {
      const d = await backend.compareSchema(options)
      setDiff(d)
      // Additive changes are chosen by default; drops and rebuilds are opt-in.
      setChosen(new Set(d.changes.filter((c) => c.kind === 'createIndex' || c.kind === 'validator').map((c) => c.id)))
      setConfirm('')
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  const destructive = !!diff?.changes.some((c) => chosen.has(c.id) && (c.kind === 'dropIndex' || c.kind === 'replaceIndex'))
  const apply = async () => {
    if (!diff) return
    setBusy(true)
    setError(null)
    try {
      const r = await backend.syncSchema(options, [...chosen], diff.fingerprint, confirm)
      setResult(r)
      toast({ kind: r.failed.length ? 'error' : 'success', title: `Applied ${r.applied.length} change${r.applied.length === 1 ? '' : 's'} to ${target}` })
      setDiff(await backend.compareSchema(options))
      setChosen(new Set())
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel schema-sync">
      <h3>Indexes and validation</h3>
      <div className="form-actions">
        <button onClick={() => void load()} disabled={!ready || busy}>
          {busy && <Spinner small />} Compare indexes &amp; validation
        </button>
        <span className="muted small">Copies index definitions and validation rules from the source to the target, after you review them.</span>
      </div>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {diff && diff.changes.length === 0 && <p className="success-text">Indexes and validation rules are the same.</p>}
      {diff && diff.changes.length > 0 && (
        <>
          <table className="grid compact" aria-label="Index and validation differences">
            <thead>
              <tr>
                <th />
                <th>Change on the target</th>
                <th>Source</th>
                <th>Target</th>
              </tr>
            </thead>
            <tbody>
              {diff.changes.map((c) => (
                <tr key={c.id}>
                  <td>
                    <input
                      type="checkbox"
                      aria-label={`${LABELS[c.kind]} ${c.name ?? ''}`.trim()}
                      checked={chosen.has(c.id)}
                      onChange={(e) =>
                        setChosen((s) => {
                          const n = new Set(s)
                          if (e.target.checked) n.add(c.id)
                          else n.delete(c.id)
                          return n
                        })
                      }
                    />
                  </td>
                  <td className={c.kind === 'dropIndex' || c.kind === 'replaceIndex' ? 'danger-text' : ''}>
                    {LABELS[c.kind]} {c.name && <code>{c.name}</code>}
                  </td>
                  <td className="mono small">
                    <pre className="schema-spec">{c.source}</pre>
                  </td>
                  <td className="mono small">
                    <pre className="schema-spec">{c.target}</pre>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {destructive && (
            <label className="field">
              <span>
                Dropping or rebuilding indexes can slow queries until they are back. Type <code>{target}</code> to confirm.
              </span>
              <input className="mono" value={confirm} onChange={(e) => setConfirm(e.target.value)} aria-label="Confirm target" />
            </label>
          )}
          <div className="form-actions">
            <button className="primary" disabled={busy || chosen.size === 0 || (destructive && confirm !== target)} onClick={() => void apply()}>
              Apply {chosen.size} change{chosen.size === 1 ? '' : 's'} to the target
            </button>
          </div>
        </>
      )}
      {result && result.failed.length > 0 && (
        <ul className="error-text small">
          {result.failed.map((f) => (
            <li key={f}>{f}</li>
          ))}
        </ul>
      )}
    </section>
  )
}
