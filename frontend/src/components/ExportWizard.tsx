import { useEffect, useMemo, useState } from 'react'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { ExportOptions, QueryRequest, Stage } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Modal, Segmented, Spinner } from './common'
import { DELIMITERS } from './ImportWizard'

/** Export wizard; the source query defaults to the collection tab's current
 * query. With `pipeline`, it exports the aggregation's results instead. */
export function ExportWizard({
  query,
  pipeline,
  allowDiskUse,
  onClose,
}: {
  query: QueryRequest
  pipeline?: Stage[]
  allowDiskUse?: boolean
  onClose: () => void
}) {
  const { toast } = useApp()
  const [opts, setOpts] = useState<ExportOptions>({
    connId: query.connId,
    db: query.db,
    coll: query.coll,
    filter: query.filter,
    projection: query.projection,
    sort: query.sort,
    skip: query.skip,
    limit: query.limit,
    path: '',
    format: 'json',
    jsonMode: 'array',
    canonical: true,
    pretty: false,
    delimiter: ',',
    noHeader: false,
    fields: [],
    pipeline,
    allowDiskUse,
  })
  const [fields, setFields] = useState<string[]>([])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [fieldFilter, setFieldFilter] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const set = <K extends keyof ExportOptions>(k: K, v: ExportOptions[K]) => setOpts((o) => ({ ...o, [k]: v }))

  useEffect(() => {
    if (pipeline) return
    backend
      .sampleFields(query.connId, query.db, query.coll)
      .then((f) => setFields(f))
      .catch(() => undefined)
  }, [query.connId, query.db, query.coll, pipeline])

  const ext = opts.format === 'xlsx' ? 'xlsx' : opts.format === 'csv' ? (opts.delimiter === 'tab' ? 'tsv' : 'csv') : opts.jsonMode === 'lines' ? 'ndjson' : 'json'
  const visible = useMemo(() => fields.filter((f) => f.toLowerCase().includes(fieldFilter.toLowerCase())), [fields, fieldFilter])

  const choose = async () => {
    try {
      const path = await backend.chooseExportFile(`${query.coll}${pipeline ? '-results' : ''}.${ext}`)
      if (path) set('path', path)
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const toggle = (f: string) =>
    setSelected((s) => {
      const n = new Set(s)
      if (n.has(f)) n.delete(f)
      else n.add(f)
      return n
    })

  const start = async () => {
    setBusy(true)
    setError(null)
    try {
      // Keep the field order of the sampled list for CSV columns.
      const chosen = fields.filter((f) => selected.has(f))
      await backend.startExport({ ...opts, fields: chosen, projection: chosen.length > 0 ? '' : opts.projection })
      toast({ kind: 'info', title: 'Export started', message: 'Progress is shown under Jobs in the status bar.' })
      onClose()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={pipeline ? `Export pipeline results: ${query.db}.${query.coll}` : `Export ${query.db}.${query.coll}`}
      onClose={onClose}
      wide
      footer={
        <>
          <span className="spacer" />
          {busy && <Spinner small />}
          <button onClick={onClose}>Cancel</button>
          <button className="primary" disabled={busy || !opts.path} onClick={start}>
            Start export
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <div className={pipeline ? '' : 'export-grid'}>
        <section>
          <h3>Documents</h3>
          {pipeline ? (
            <p className="muted">
              The results of the pipeline ({pipeline.filter((st) => st.enabled).length} enabled stages): every document, not just the shown page. To pick fields or
              limit them, add <code>$project</code> or <code>$limit</code> stages.
            </p>
          ) : (
            <>
              <label className="field">
                <span>Filter</span>
                <input className="mono" value={opts.filter} onChange={(e) => set('filter', e.target.value)} placeholder="{}" spellCheck={false} />
              </label>
              <div className="field-row">
                <label className="field">
                  <span>Sort</span>
                  <input className="mono" value={opts.sort} onChange={(e) => set('sort', e.target.value)} placeholder="{ field: 1 }" spellCheck={false} />
                </label>
                <label className="field narrow">
                  <span>Skip</span>
                  <input type="number" min={0} value={opts.skip} onChange={(e) => set('skip', Math.max(0, Number(e.target.value) || 0))} />
                </label>
                <label className="field narrow">
                  <span>Limit</span>
                  <input type="number" min={0} value={opts.limit} onChange={(e) => set('limit', Math.max(0, Number(e.target.value) || 0))} title="0 = all" />
                </label>
              </div>
            </>
          )}

          <h3>Format</h3>
          <Segmented
            label="Export format"
            value={opts.format}
            onChange={(f) => set('format', f)}
            options={[
              { value: 'json', label: 'JSON' },
              { value: 'csv', label: 'CSV' },
              { value: 'xlsx', label: 'Excel' },
            ]}
          />
          {opts.format === 'json' ? (
            <div className="format-options">
              <label className="field">
                <span>Layout</span>
                <select value={opts.jsonMode} onChange={(e) => set('jsonMode', e.target.value as ExportOptions['jsonMode'])}>
                  <option value="array">JSON array</option>
                  <option value="lines">One document per line (NDJSON, mongoimport-compatible)</option>
                </select>
              </label>
              <label className="check">
                <input type="checkbox" checked={opts.canonical} onChange={(e) => set('canonical', e.target.checked)} />
                Canonical Extended JSON (keeps int64 / double / decimal exactly)
              </label>
              {opts.jsonMode === 'array' && (
                <label className="check">
                  <input type="checkbox" checked={opts.pretty} onChange={(e) => set('pretty', e.target.checked)} />
                  Pretty-print
                </label>
              )}
            </div>
          ) : opts.format === 'xlsx' ? (
            <div className="format-options">
              <label className="check">
                <input type="checkbox" checked={!opts.noHeader} onChange={(e) => set('noHeader', !e.target.checked)} />
                Header row
              </label>
              <p className="muted small">
                One sheet named after the collection. Numbers, true/false and dates become typed cells (dates in UTC); nested fields become dotted
                columns, arrays and sub-documents JSON text. A sheet holds at most 1,048,575 rows — use JSON for backups.
              </p>
            </div>
          ) : (
            <div className="format-options">
              <div className="field-row">
                <label className="field">
                  <span>Delimiter</span>
                  <select value={opts.delimiter} onChange={(e) => set('delimiter', e.target.value)}>
                    {DELIMITERS.map((d) => (
                      <option key={d.value} value={d.value}>
                        {d.label}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
              <label className="check">
                <input type="checkbox" checked={!opts.noHeader} onChange={(e) => set('noHeader', !e.target.checked)} />
                Header row
              </label>
              <p className="muted small">
                Nested fields become dotted columns; arrays are written as JSON. CSV cannot keep BSON types exactly — use JSON for
                backups.
              </p>
            </div>
          )}

          <h3>Destination</h3>
          <div className="file-row">
            <input className="mono" value={opts.path} onChange={(e) => set('path', e.target.value)} placeholder={`/path/to/${query.coll}.${ext}`} spellCheck={false} />
            <button onClick={choose}>Browse…</button>
          </div>
        </section>

        {!pipeline && (
          <section className="field-picker">
            <h3>
              Fields <span className="muted small">({selected.size === 0 ? 'all' : `${selected.size} selected`})</span>
            </h3>
            <input placeholder="Filter fields" value={fieldFilter} onChange={(e) => setFieldFilter(e.target.value)} aria-label="Filter fields" />
            <div className="field-picker-actions">
              <button className="small ghost" onClick={() => setSelected(new Set(visible))}>
                Select shown
              </button>
              <button className="small ghost" onClick={() => setSelected(new Set())}>
                Clear (= all fields)
              </button>
            </div>
            <ul className="field-list">
              {visible.map((f) => (
                <li key={f}>
                  <label className="check">
                    <input type="checkbox" checked={selected.has(f)} onChange={() => toggle(f)} />
                    <span className="mono" style={{ paddingLeft: `${(f.split('.').length - 1) * 12}px` }}>
                      {f}
                    </span>
                  </label>
                </li>
              ))}
              {fields.length === 0 && <li className="muted small">Sampling fields…</li>}
            </ul>
            {opts.projection && selected.size === 0 && (
              <p className="muted small">
                The tab's projection <code>{opts.projection}</code> applies.
              </p>
            )}
          </section>
        )}
      </div>
    </Modal>
  )
}
