import { useCallback, useEffect, useState } from 'react'
import { backend } from '../lib/backend'
import { formatCount } from '../lib/format'
import { toAppError, type AppError } from '../lib/errors'
import type { ProfileEntry, Profiling, SessionInfo, SQLDialect } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Modal, Segmented, Spinner } from './common'
import { JobStatus } from './p2common'

const LEVELS: { value: '0' | '1' | '2'; label: string; title: string }[] = [
  { value: '0', label: 'Off', title: 'No profiling' },
  { value: '1', label: 'Slow operations', title: 'Record operations slower than the threshold' },
  { value: '2', label: 'All operations', title: 'Record every operation (adds load; use briefly)' },
]

/** The database profiler: level, threshold and the recorded operations. */
export function ProfilerDialog({ conn, db, onClose }: { conn: SessionInfo; db: string; onClose: () => void }) {
  const { confirm, toast } = useApp()
  const [p, setP] = useState<Profiling | null>(null)
  const [entries, setEntries] = useState<ProfileEntry[] | null>(null)
  const [coll, setColl] = useState('')
  const [op, setOp] = useState('')
  const [minMillis, setMinMillis] = useState(0)
  const [open, setOpen] = useState<number | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  const load = useCallback(async () => {
    setError(null)
    try {
      const [lvl, list] = await Promise.all([backend.getProfiling(conn.id, db), backend.listProfile(conn.id, db, { coll: coll.trim(), op, minMillis, limit: 200 })])
      setP(lvl)
      setEntries(list)
    } catch (err) {
      setError(toAppError(err))
    }
  }, [conn.id, db, coll, op, minMillis])
  useEffect(() => {
    void load()
  }, [load])

  const apply = async (next: Profiling) => {
    if (next.level === 2) {
      const ok = await confirm({
        title: 'Profile all operations',
        message: 'Recording every operation adds load to the server and fills system.profile quickly. Turn it off when you are done.',
        confirmLabel: 'Profile everything',
      })
      if (!ok) return
    }
    setBusy(true)
    try {
      await backend.setProfiling(conn.id, db, next)
      toast({ kind: 'success', title: next.level === 0 ? 'Profiler off' : `Profiler level ${next.level}` })
      await load()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  const clear = async () => {
    const ok = await confirm({ title: 'Clear profiler data', message: `Delete the recorded operations of ${db} (system.profile)?`, confirmLabel: 'Clear', danger: true })
    if (!ok) return
    try {
      await backend.clearProfile(conn.id, db)
      await load()
    } catch (err) {
      setError(toAppError(err))
    }
  }
  return (
    <Modal title={`Query profiler: ${db}`} onClose={onClose} wide footer={<button onClick={onClose}>Close</button>}>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {!p ? (
        !error && <Spinner />
      ) : (
        <>
          <div className="field-row">
            <div className="field">
              <span>Profiling</span>
              <Segmented label="Profiling level" value={String(p.level) as '0' | '1' | '2'} onChange={(v) => !conn.readOnly && !busy && apply({ ...p, level: Number(v) as Profiling['level'] })} options={LEVELS} />
            </div>
            <label className="field narrow">
              <span>Slow (ms)</span>
              <input type="number" min={0} value={p.slowms} onChange={(e) => setP({ ...p, slowms: Number(e.target.value) })} aria-label="Slow threshold" disabled={conn.readOnly} />
            </label>
            <label className="field narrow">
              <span>Sample rate</span>
              <input type="number" min={0} max={1} step={0.1} value={p.sampleRate} onChange={(e) => setP({ ...p, sampleRate: Number(e.target.value) })} aria-label="Sample rate" disabled={conn.readOnly} />
            </label>
            {!conn.readOnly && (
              <button onClick={() => apply(p)} disabled={busy}>
                Apply
              </button>
            )}
          </div>
          <p className="muted small">The slow threshold and sample rate apply to the whole server (and its log). Profiling runs on mongod; through mongos, connect to a shard.</p>
        </>
      )}
      <div className="field-row">
        <label className="field">
          <span>Collection</span>
          <input value={coll} onChange={(e) => setColl(e.target.value)} placeholder="all" aria-label="Collection filter" />
        </label>
        <label className="field narrow">
          <span>Operation</span>
          <select value={op} onChange={(e) => setOp(e.target.value)} aria-label="Operation filter">
            <option value="">all</option>
            {['query', 'getmore', 'insert', 'update', 'remove', 'command'].map((o) => (
              <option key={o}>{o}</option>
            ))}
          </select>
        </label>
        <label className="field narrow">
          <span>At least (ms)</span>
          <input type="number" min={0} value={minMillis} onChange={(e) => setMinMillis(Number(e.target.value))} aria-label="Minimum duration" />
        </label>
        <button onClick={() => void load()}>Refresh</button>
        {!conn.readOnly && (
          <button className="ghost danger-text" onClick={clear}>
            Clear
          </button>
        )}
      </div>
      {entries && entries.length === 0 && <p className="muted">No recorded operations{p?.level === 0 ? ' (profiling is off)' : ''}.</p>}
      {entries && entries.length > 0 && (
        <div className="backup-entries">
          <table className="grid compact">
            <thead>
              <tr>
                <th>Time</th>
                <th>Op</th>
                <th>Namespace</th>
                <th>ms</th>
                <th>Plan</th>
                <th>Keys / docs examined</th>
                <th>Returned</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e, i) => (
                <tr key={i} className={open === i ? 'selected' : ''} onClick={() => setOpen(open === i ? null : i)} title="Show the command">
                  <td className="small">{new Date(e.ts).toLocaleTimeString()}</td>
                  <td>{e.op}</td>
                  <td className="mono">{e.ns}</td>
                  <td>{formatCount(e.millis)}</td>
                  <td className={e.planSummary.includes('COLLSCAN') ? 'warning-text' : ''}>{e.planSummary}</td>
                  <td>
                    {formatCount(e.keysExamined)} / {formatCount(e.docsExamined)}
                  </td>
                  <td>{formatCount(e.nreturned)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {open !== null && entries?.[open] && (
        <pre className="job-log" aria-label="Profiled command">
          {entries[open].command}
          {entries[open].appName && `\n\napp: ${entries[open].appName}`}
          {entries[open].user && `  user: ${entries[open].user}`}
          {entries[open].client && `  client: ${entries[open].client}`}
        </pre>
      )}
    </Modal>
  )
}

/** Export a collection as a SQL script (CREATE TABLE + INSERT). */
export function SqlExportDialog({ conn, db, coll, onClose }: { conn: SessionInfo; db: string; coll: string; onClose: () => void }) {
  const [dialect, setDialect] = useState<SQLDialect>('postgres')
  const [path, setPath] = useState('')
  const [table, setTable] = useState(coll.replace(/\./g, '_'))
  const [flatten, setFlatten] = useState(true)
  const [dropTable, setDropTable] = useState(false)
  const [sampleSize, setSampleSize] = useState(1000)
  const [filter, setFilter] = useState('')
  const [jobId, setJobId] = useState<string | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  const run = async () => {
    setError(null)
    try {
      const j = await backend.startSQLExport({ connId: conn.id, db, coll, path, options: { dialect, table: table.trim(), filter: filter.trim(), sampleSize, batchRows: 200, flatten, dropTable } })
      setJobId(j.id)
    } catch (err) {
      setError(toAppError(err))
    }
  }
  return (
    <Modal
      title={`Export ${db}.${coll} to SQL`}
      onClose={onClose}
      wide
      footer={
        <>
          <button onClick={onClose}>{jobId ? 'Close' : 'Cancel'}</button>
          {!jobId && (
            <button className="primary" disabled={!path || !table.trim()} onClick={run}>
              Export
            </button>
          )}
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <Segmented
        label="SQL dialect"
        value={dialect}
        onChange={setDialect}
        options={[
          { value: 'postgres', label: 'PostgreSQL' },
          { value: 'mysql', label: 'MySQL' },
          { value: 'sqlite', label: 'SQLite' },
        ]}
      />
      <div className="field-row">
        <label className="field">
          <span>SQL file</span>
          <div className="file-row">
            <input value={path} readOnly placeholder="Choose where to save the .sql file" aria-label="SQL file" />
            <button onClick={async () => setPath((await backend.chooseSaveFile('Save SQL script', `${table || coll}.${dialect}.sql`)) || path)}>Save as…</button>
          </div>
        </label>
        <label className="field narrow">
          <span>Table</span>
          <input value={table} onChange={(e) => setTable(e.target.value)} aria-label="Table name" />
        </label>
      </div>
      <label className="field">
        <span>Only documents matching (optional, Extended JSON)</span>
        <input className="mono" value={filter} onChange={(e) => setFilter(e.target.value)} placeholder='{ "status": "active" }' aria-label="Filter" />
      </label>
      <div className="check-row">
        <label className="check">
          <input type="checkbox" checked={flatten} onChange={(e) => setFlatten(e.target.checked)} /> Nested fields as columns (address.city → address_city)
        </label>
        <label className="check">
          <input type="checkbox" checked={dropTable} onChange={(e) => setDropTable(e.target.checked)} /> Start with DROP TABLE IF EXISTS
        </label>
        <label className="check">
          Columns from the first
          <input type="number" min={1} max={100000} value={sampleSize} onChange={(e) => setSampleSize(Number(e.target.value))} aria-label="Sample size" /> documents
        </label>
      </div>
      <p className="muted small">
        Types map to native SQL types (ObjectId → CHAR(24), dates → TIMESTAMP, decimals → exact NUMERIC). Arrays and mixed-type fields are stored as JSON. Fields the sample didn't
        include, and values that don't fit their column, are kept as JSON in an <code>_extra</code> column, so nothing is dropped. Indexes and validation rules are not converted.
      </p>
      {jobId && <JobStatus jobId={jobId} />}
    </Modal>
  )
}
