import { useState } from 'react'
import { ErrorBanner, Segmented, Spinner } from '../components/common'
import { ConfirmInput, EndpointPicker, JobStatus, Stat, useJobDone } from '../components/p2common'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatCount, formatDuration } from '../lib/format'
import type { Tab } from '../lib/tabs'
import type { CompareResult, DBCompareResult, DocDiff, Endpoint, SyncOptions, SyncOutcome, SyncPreview } from '../lib/types'
import { useApp } from '../state/AppContext'
import { SchemaSync } from '../components/SchemaSync'

function DiffList({ samples }: { samples: DocDiff[] }) {
  if (samples.length === 0) return null
  const label = { changed: 'changed', onlySource: 'only in source', onlyTarget: 'only in target' }
  return (
    <div className="diff-list">
      {samples.map((s) => (
        <details key={s.id + s.kind} className={`diff diff-${s.kind}`}>
          <summary>
            <code>{s.id}</code> <span className={`kind-badge kind-${s.kind}`}>{label[s.kind]}</span>
            {s.fields.length > 0 && <span className="muted small"> {s.fields.length} field(s)</span>}
          </summary>
          {s.fields.length > 0 && (
            <table className="grid compact">
              <thead>
                <tr>
                  <th>Field</th>
                  <th>Change</th>
                  <th>Source</th>
                  <th>Target</th>
                </tr>
              </thead>
              <tbody>
                {s.fields.map((f) => (
                  <tr key={f.path}>
                    <td className="mono">{f.path}</td>
                    <td>{f.change}</td>
                    <td className="mono">{f.source}</td>
                    <td className="mono">{f.target}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </details>
      ))}
    </div>
  )
}

export function CompareView({ tab }: { tab: Tab }) {
  const { toast } = useApp()
  const [mode, setMode] = useState<'collection' | 'database'>(tab.coll ? 'collection' : 'database')
  const [source, setSource] = useState<Endpoint>({ connId: tab.connId ?? '', db: tab.db ?? '', coll: tab.coll ?? '' })
  const [target, setTarget] = useState<Endpoint>({ connId: tab.connId ?? '', db: '', coll: tab.coll ?? '' })
  const [filter, setFilter] = useState('')
  const [ignore, setIgnore] = useState('')
  const [ignoreOrder, setIgnoreOrder] = useState(true)
  const [matchKey, setMatchKey] = useState('')
  const [deep, setDeep] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [compareJob, setCompareJob] = useState<string | null>(null)
  const [result, setResult] = useState<CompareResult | null>(null)
  const [dbResult, setDbResult] = useState<DBCompareResult | null>(null)
  const [syncOpts, setSyncOpts] = useState<SyncOptions>({ insert: true, replace: true, delete: false })
  const [preview, setPreview] = useState<SyncPreview | null>(null)
  const [confirm, setConfirm] = useState('')
  const [syncJob, setSyncJob] = useState<string | null>(null)
  const [outcome, setOutcome] = useState<SyncOutcome | null>(null)
  const [busy, setBusy] = useState(false)

  const cmpJob = useJobDone(compareJob, async (j) => {
    if (j.state !== 'succeeded') return
    try {
      if (mode === 'collection') setResult(await backend.getCompareResult(j.id))
      else setDbResult(await backend.getDatabaseCompareResult(j.id))
    } catch (err) {
      setError(toAppError(err))
    }
  })
  useJobDone(syncJob, async (j) => {
    try {
      setOutcome(await backend.getSyncOutcome(j.id))
    } catch {
      // failed before producing an outcome; the job row shows the error
    }
  })

  const keyFields = matchKey
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  const collOptions = {
    source,
    target,
    filter,
    ignoreFields: ignore
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean),
    ignoreFieldOrder: ignoreOrder,
    sampleSize: 50,
    matchKey: keyFields,
  }

  const run = async () => {
    setError(null)
    setResult(null)
    setDbResult(null)
    setPreview(null)
    setOutcome(null)
    setSyncJob(null)
    setBusy(true)
    try {
      const job =
        mode === 'collection'
          ? await backend.startCompare(collOptions)
          : await backend.startCompareDatabases({ sourceConnId: source.connId, sourceDb: source.db, targetConnId: target.connId, targetDb: target.db, deep })
      setCompareJob(job.id)
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  const previewSync = async () => {
    if (!compareJob) return
    setError(null)
    setOutcome(null)
    try {
      setPreview(await backend.previewSync(compareJob, syncOpts))
      setConfirm('')
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const apply = async () => {
    if (!preview) return
    setError(null)
    try {
      const job = await backend.startSync(preview.planId, confirm)
      setSyncJob(job.id)
      setPreview(null)
      toast({ kind: 'info', title: 'Sync started', message: 'A backup of every changed document is written first.' })
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const running = cmpJob && (cmpJob.state === 'running' || cmpJob.state === 'queued')

  return (
    <div className="view compare-view">
      <div className="view-header">
        <h1>Compare &amp; sync</h1>
        <span className="spacer" />
        <Segmented
          label="Compare what"
          value={mode}
          onChange={(m) => {
            setMode(m)
            setResult(null)
            setDbResult(null)
          }}
          options={[
            { value: 'collection', label: 'Collections' },
            { value: 'database', label: 'Databases' },
          ]}
        />
      </div>
      <div className="endpoints">
        <EndpointPicker label="Source" value={source} onChange={setSource} collection={mode === 'collection'} />
        <EndpointPicker label="Target" value={target} onChange={setTarget} collection={mode === 'collection'} />
      </div>
      {mode === 'collection' ? (
        <div className="field-row">
          <label className="field">
            <span>Filter (both sides)</span>
            <input className="mono" value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="{}" />
          </label>
          <label className="field">
            <span>Ignore fields (comma-separated paths)</span>
            <input className="mono" value={ignore} onChange={(e) => setIgnore(e.target.value)} placeholder="updatedAt, meta.etag" />
          </label>
          <label className="field">
            <span>Match documents by (empty = _id)</span>
            <input className="mono" value={matchKey} onChange={(e) => setMatchKey(e.target.value)} placeholder="email  or  tenant, orderNo" aria-label="Match documents by" />
          </label>
          <label className="check">
            <input type="checkbox" checked={ignoreOrder} onChange={(e) => setIgnoreOrder(e.target.checked)} />
            Ignore field order
          </label>
        </div>
      ) : (
        <label className="check">
          <input type="checkbox" checked={deep} onChange={(e) => setDeep(e.target.checked)} />
          Compare documents of collections present on both sides (slower)
        </label>
      )}
      <div className="form-actions">
        <button className="primary" onClick={run} disabled={busy || !!running}>
          {busy || running ? <Spinner small /> : null} Compare
        </button>
        <span className="muted small">
          Documents are matched by {keyFields.length ? keyFields.join(' + ') + ' (must be unique on both sides; the target keeps its _ids)' : '_id'}. Results are
          repeatable: equal inputs give the same fingerprint.
        </span>
      </div>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <JobStatus jobId={compareJob} />

      {result && (
        <section className="panel">
          <div className="metrics">
            <Stat label="Identical" value={formatCount(result.identical)} tone="good" />
            <Stat label="Changed" value={formatCount(result.changed)} tone={result.changed ? 'warn' : undefined} />
            <Stat label="Only in source" value={formatCount(result.onlySource)} tone={result.onlySource ? 'warn' : undefined} />
            <Stat label="Only in target" value={formatCount(result.onlyTarget)} tone={result.onlyTarget ? 'warn' : undefined} />
            <Stat label="Fingerprint" value={result.fingerprint} />
          </div>
          <p className="muted small">
            {formatCount(result.sourceCount)} source and {formatCount(result.targetCount)} target documents compared in {formatDuration(result.durationMs)}.
            {result.samples.length > 0 && ` First ${result.samples.length} differences (sorted by ${keyFields.length ? keyFields.join(' + ') : '_id'}):`}
          </p>
          <DiffList samples={result.samples} />

          {(result.changed > 0 || result.onlySource > 0 || result.onlyTarget > 0) && (
            <div className="sync-box">
              <h3>Synchronize target from source</h3>
              <div className="field-row">
                <label className="check">
                  <input type="checkbox" checked={syncOpts.insert} onChange={(e) => setSyncOpts({ ...syncOpts, insert: e.target.checked })} />
                  Insert missing ({formatCount(result.onlySource)})
                </label>
                <label className="check">
                  <input type="checkbox" checked={syncOpts.replace} onChange={(e) => setSyncOpts({ ...syncOpts, replace: e.target.checked })} />
                  Replace changed ({formatCount(result.changed)})
                </label>
                <label className="check">
                  <input type="checkbox" checked={syncOpts.delete} onChange={(e) => setSyncOpts({ ...syncOpts, delete: e.target.checked })} />
                  Delete extra in target ({formatCount(result.onlyTarget)})
                </label>
                <button onClick={previewSync}>Preview sync</button>
              </div>
              {preview && (
                <div className="preview-box">
                  <p>
                    Dry run: <strong>{preview.inserts}</strong> insert(s), <strong>{preview.replaces}</strong> replace(s),{' '}
                    <strong className={preview.deletes ? 'danger-text' : ''}>{preview.deletes}</strong> delete(s) on <code>{preview.target}</code>. Every
                    document is re-checked when applied; anything that changed since this preview is skipped and reported. Before-images go to a backup you can
                    restore from the Library.
                  </p>
                  <DiffList samples={preview.samples.slice(0, 10)} />
                  <ConfirmInput text={preview.confirmText} value={confirm} onChange={setConfirm} action={`Deletes ${preview.deletes} document(s)`} />
                  <div className="form-actions">
                    <button onClick={() => setPreview(null)}>Discard</button>
                    <button className="primary" disabled={!!preview.confirmText && confirm !== preview.confirmText} onClick={apply}>
                      Apply exactly this plan
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}
        </section>
      )}

      {mode === 'collection' && <SchemaSync options={collOptions} />}

      <JobStatus jobId={syncJob} />
      {outcome && (
        <section className="panel">
          <h3>Sync result</h3>
          <div className="metrics">
            <Stat label="Inserted" value={`${outcome.applied.inserts} / ${outcome.planned.inserts}`} />
            <Stat label="Replaced" value={`${outcome.applied.replaces} / ${outcome.planned.replaces}`} />
            <Stat label="Deleted" value={`${outcome.applied.deletes} / ${outcome.planned.deletes}`} />
            <Stat label="Matches preview" value={outcome.matchesPreview ? 'yes' : 'no'} tone={outcome.matchesPreview ? 'good' : 'warn'} />
            <Stat label="Verified" value={outcome.verifiedEqual ? 'in sync' : 'differences left'} tone={outcome.verifiedEqual ? 'good' : 'warn'} />
          </div>
          {!outcome.matchesPreview && (
            <p className="muted small">
              Skipped because they changed after the preview: {outcome.skippedIds.join(', ')}
            </p>
          )}
          <p className="muted small">
            Backup: <code>{outcome.backupPath}</code>
          </p>
        </section>
      )}

      {dbResult && (
        <section className="panel">
          <table className="grid">
            <thead>
              <tr>
                <th>Collection</th>
                <th>Source</th>
                <th>Target</th>
                <th>Indexes</th>
                {dbResult.options.deep && <th>Documents</th>}
              </tr>
            </thead>
            <tbody>
              {dbResult.collections.map((c) => (
                <tr key={c.name}>
                  <td className="mono">{c.name}</td>
                  <td>{c.inSource ? formatCount(c.sourceCount) : <span className="danger-text">missing</span>}</td>
                  <td>{c.inTarget ? formatCount(c.targetCount) : <span className="danger-text">missing</span>}</td>
                  <td className="small">{c.indexDiffs.length ? c.indexDiffs.join('; ') : c.inSource && c.inTarget ? 'same' : ''}</td>
                  {dbResult.options.deep && (
                    <td className="small">
                      {c.compared ? (c.changed + c.onlySource + c.onlyTarget === 0 ? 'identical' : `${c.changed} changed, ${c.onlySource} / ${c.onlyTarget} only`) : ''}
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
    </div>
  )
}
