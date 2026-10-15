import { useEffect, useState } from 'react'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatBytes } from '../lib/format'
import type { ColumnSpec, ImportOptions, ImportPreview } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Modal, Spinner } from './common'
import { highlight } from './highlight'

export const COLUMN_TYPES = ['auto', 'string', 'int', 'long', 'double', 'decimal', 'bool', 'date', 'objectId', 'json']
export const DELIMITERS = [
  { value: ',', label: 'Comma (,)' },
  { value: ';', label: 'Semicolon (;)' },
  { value: 'tab', label: 'Tab' },
  { value: '|', label: 'Pipe (|)' },
]

export function ImportWizard({ connId, db, coll, onClose }: { connId: string; db: string; coll?: string; onClose: () => void }) {
  const { toast, sessions } = useApp()
  const readOnly = !!sessions.find((s) => s.id === connId)?.readOnly
  const [step, setStep] = useState(0)
  const [opts, setOpts] = useState<ImportOptions>({
    connId,
    db,
    coll: coll ?? '',
    path: '',
    format: '',
    delimiter: ',',
    columns: [],
    keepBlanks: false,
    mode: 'insert',
    dropFirst: false,
    stopOnError: false,
    batchSize: 1000,
  })
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const set = <K extends keyof ImportOptions>(k: K, v: ImportOptions[K]) => setOpts((o) => ({ ...o, [k]: v }))

  // Re-run the preview whenever parsing options change.
  const previewKey = JSON.stringify([opts.path, opts.format, opts.delimiter, opts.columns, opts.keepBlanks])
  useEffect(() => {
    if (!opts.path) return
    let cancelled = false
    setLoading(true)
    setError(null)
    backend
      .previewImport(opts)
      .then((p) => {
        if (cancelled) return
        setPreview(p)
        if (opts.format === '') set('format', p.format)
        if (p.format === 'csv' && opts.columns.length === 0 && p.columns.length > 0) {
          set('columns', p.columns.map((c) => ({ name: c.name, type: 'auto', skip: false })))
        }
      })
      .catch((err) => !cancelled && (setPreview(null), setError(toAppError(err))))
      .finally(() => !cancelled && setLoading(false))
    return () => {
      cancelled = true
    }
  }, [previewKey])

  const choose = async () => {
    try {
      const path = await backend.chooseImportFile()
      if (!path) return
      setOpts((o) => ({ ...o, path, format: '', columns: [] }))
      if (!coll) {
        const base = path.split(/[\\/]/).pop() ?? ''
        set('coll', base.replace(/\.[^.]+$/, '').replace(/[^\w-]/g, '_'))
      }
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const setColumn = (i: number, patch: Partial<ColumnSpec>) =>
    set('columns', opts.columns.map((c, j) => (j === i ? { ...c, ...patch } : c)))

  const start = async () => {
    setLoading(true)
    setError(null)
    try {
      await backend.startImport(opts)
      toast({ kind: 'info', title: 'Import started', message: 'Progress is shown under Jobs in the status bar.' })
      onClose()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setLoading(false)
    }
  }

  const steps = ['Source', 'Preview', 'Target']
  const canNext = step === 0 ? !!opts.path && !!preview : step === 1 ? !!preview && preview.documents.length > 0 : false

  return (
    <Modal
      title="Import documents"
      onClose={onClose}
      wide
      footer={
        <>
          <ol className="stepper">
            {steps.map((s, i) => (
              <li key={s} className={i === step ? 'active' : i < step ? 'done' : ''}>
                {i + 1}. {s}
              </li>
            ))}
          </ol>
          <span className="spacer" />
          {loading && <Spinner small />}
          <button onClick={onClose}>Cancel</button>
          {step > 0 && <button onClick={() => setStep(step - 1)}>Back</button>}
          {step < 2 ? (
            <button className="primary" disabled={!canNext || loading} onClick={() => setStep(step + 1)}>
              Next
            </button>
          ) : (
            <button className="primary" disabled={loading || !opts.db || !opts.coll || readOnly} onClick={start}>
              Start import
            </button>
          )}
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {step === 0 && (
        <div className="wizard-step">
          <div className="field">
            <span>File</span>
            <div className="file-row">
              <input
                className="mono"
                value={opts.path}
                onChange={(e) => setOpts((o) => ({ ...o, path: e.target.value, columns: [] }))}
                placeholder="/path/to/data.json"
                spellCheck={false}
              />
              <button onClick={choose}>Browse…</button>
            </div>
            {preview && (
              <small className="hint">
                {formatBytes(preview.sizeBytes)} · detected {preview.format.toUpperCase()}
              </small>
            )}
          </div>
          <div className="field-row">
            <label className="field">
              <span>Format</span>
              <select value={opts.format} onChange={(e) => setOpts((o) => ({ ...o, format: e.target.value as ImportOptions['format'], columns: [] }))}>
                <option value="">Detect from extension</option>
                <option value="json">JSON (array, NDJSON or mongoexport)</option>
                <option value="csv">CSV / TSV</option>
              </select>
            </label>
            {opts.format === 'csv' && (
              <label className="field">
                <span>Delimiter</span>
                <select value={opts.delimiter} onChange={(e) => setOpts((o) => ({ ...o, delimiter: e.target.value, columns: [] }))}>
                  {DELIMITERS.map((d) => (
                    <option key={d.value} value={d.value}>
                      {d.label}
                    </option>
                  ))}
                </select>
              </label>
            )}
          </div>
          {opts.format === 'csv' && (
            <label className="check">
              <input type="checkbox" checked={opts.keepBlanks} onChange={(e) => set('keepBlanks', e.target.checked)} />
              Import empty cells as empty strings (default: omit the field)
            </label>
          )}
          <p className="muted small">
            JSON files keep every BSON type (Extended JSON). CSV headers like <code>address.city</code> create nested documents; types
            are inferred per cell unless you set them in the next step.
          </p>
        </div>
      )}

      {step === 1 && preview && (
        <div className="wizard-step">
          {preview.error && (
            <div className="error-banner" role="alert">
              <div>
                <strong>Parse error</strong>
                <div className="error-message">{preview.error}</div>
                <div className="muted small">Documents before this point parsed correctly. With "stop on error" off, CSV rows that fail are skipped and reported.</div>
              </div>
            </div>
          )}
          {opts.format === 'csv' && opts.columns.length > 0 && (
            <table className="grid compact columns-table">
              <thead>
                <tr>
                  <th>Column</th>
                  <th>Detected</th>
                  <th>Import as</th>
                  <th>Skip</th>
                  <th>Samples</th>
                </tr>
              </thead>
              <tbody>
                {opts.columns.map((c, i) => {
                  const p = preview.columns.find((x) => x.name === c.name)
                  return (
                    <tr key={c.name}>
                      <td className="mono">{c.name}</td>
                      <td>{p?.inferred}</td>
                      <td>
                        <select value={c.type} onChange={(e) => setColumn(i, { type: e.target.value })} aria-label={`Type of ${c.name}`}>
                          {COLUMN_TYPES.map((t) => (
                            <option key={t} value={t}>
                              {t}
                            </option>
                          ))}
                        </select>
                      </td>
                      <td>
                        <input type="checkbox" checked={c.skip} onChange={(e) => setColumn(i, { skip: e.target.checked })} aria-label={`Skip ${c.name}`} />
                      </td>
                      <td className="muted">{p?.samples.join(' · ')}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
          <h3>
            First {preview.documents.length} document{preview.documents.length === 1 ? '' : 's'}
          </h3>
          <div className="preview-docs">
            {preview.documents.map((d, i) => (
              <pre key={i} className="doc-text">
                {highlight(d.text)}
              </pre>
            ))}
          </div>
        </div>
      )}

      {step === 2 && (
        <div className="wizard-step">
          <div className="field-row">
            <label className="field">
              <span>Database</span>
              <input value={opts.db} onChange={(e) => set('db', e.target.value)} spellCheck={false} />
            </label>
            <label className="field">
              <span>Collection</span>
              <input value={opts.coll} onChange={(e) => set('coll', e.target.value)} spellCheck={false} />
            </label>
          </div>
          <label className="field">
            <span>Mode</span>
            <select value={opts.mode} onChange={(e) => set('mode', e.target.value as ImportOptions['mode'])}>
              <option value="insert">Insert — documents with an existing _id fail and are reported</option>
              <option value="upsert">Upsert — replace documents with the same _id, insert the rest</option>
            </select>
          </label>
          <label className="check">
            <input type="checkbox" checked={opts.stopOnError} onChange={(e) => set('stopOnError', e.target.checked)} />
            Stop at the first error
          </label>
          <label className="check">
            <input type="checkbox" checked={opts.dropFirst} onChange={(e) => set('dropFirst', e.target.checked)} />
            Drop the collection before importing
          </label>
          {opts.dropFirst && (
            <div className="error-banner">
              <div>
                <strong>Destructive</strong>
                <div className="error-message">
                  All existing documents and indexes in {opts.db}.{opts.coll} will be deleted.
                </div>
              </div>
            </div>
          )}
          {readOnly && <div className="error-text">This connection is read-only; imports are blocked.</div>}
          <p className="muted small">Documents are written in batches of {opts.batchSize}. The import runs in the background and can be canceled.</p>
        </div>
      )}
    </Modal>
  )
}
