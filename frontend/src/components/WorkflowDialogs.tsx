import { useCallback, useEffect, useState } from 'react'
import { backend, newOpId } from '../lib/backend'
import { formatBytes, formatCount } from '../lib/format'
import { toAppError, type AppError } from '../lib/errors'
import type { IndexAdvice, IndexFinding, ProfileEntry, Profiling, QueryShape, SessionInfo, SQLDialect } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Modal, Segmented, Spinner } from './common'
import { emptyIndex } from './ManageDialogs'
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

const STATUS_LABEL: Record<QueryShape['status'], string> = {
  missing: 'no index',
  partial: 'partly indexed',
  covered: 'indexed',
  hidden: 'index hidden',
  none: 'no suggestion',
}

/**
 * The index advisor: slow query shapes from the profiler with a suggested
 * index for each, plus indexes that look unused or redundant.
 */
export function IndexAdvisorDialog({ conn, db, coll, onClose }: { conn: SessionInfo; db: string; coll?: string; onClose: () => void }) {
  const { confirm, toast } = useApp()
  const [filterColl, setFilterColl] = useState(coll ?? '')
  const [minMillis, setMinMillis] = useState(0)
  const [advice, setAdvice] = useState<IndexAdvice | null>(null)
  const [open, setOpen] = useState<number | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  const analyze = useCallback(async () => {
    setBusy(true)
    setError(null)
    try {
      setAdvice(await backend.analyzeIndexes(newOpId('advisor'), conn.id, db, { coll: filterColl.trim(), minMillis, limit: 5000 }))
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }, [conn.id, db, filterColl, minMillis])
  useEffect(() => {
    void analyze()
    // Analyze once on open; later runs are explicit.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const act = async (fn: () => Promise<unknown>, done: string) => {
    setBusy(true)
    setError(null)
    try {
      await fn()
      toast({ kind: 'success', title: done })
      await analyze()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  const create = async (s: QueryShape) => {
    if (!s.suggested) return
    const ok = await confirm({
      title: 'Create suggested index',
      message: (
        <>
          <code>
            db.getCollection({JSON.stringify(s.coll)}).createIndex({s.suggested})
          </code>
          <p className="muted small">Building an index reads the whole collection and adds work to every write. On a large collection, create it at a quiet time.</p>
        </>
      ),
      confirmLabel: 'Create index',
    })
    if (ok) await act(() => backend.createIndex(newOpId('index'), conn.id, db, s.coll, { ...emptyIndex(), keys: s.suggested! }), `Created index ${s.name}`)
  }
  const drop = async (f: IndexFinding) => {
    const ok = await confirm({
      title: 'Drop index',
      message: (
        <>
          Drop <code>{f.name}</code> on {db}.{f.coll}? {f.reason} Hiding it first is safer: queries stop using it, and you can unhide it instantly if something slows down.
        </>
      ),
      confirmLabel: 'Drop index',
      danger: true,
      typeToConfirm: f.name,
    })
    if (ok) await act(() => backend.dropIndex(conn.id, db, f.coll, f.name), `Dropped ${f.name}`)
  }
  const findings = (title: string, list: IndexFinding[], empty: string) => (
    <>
      <h4>{title}</h4>
      {list.length === 0 ? (
        <p className="muted small">{empty}</p>
      ) : (
        <table className="grid compact">
          <tbody>
            {list.map((f) => (
              <tr key={`${f.coll}.${f.name}`}>
                <td className="mono">
                  {f.coll}.{f.name}
                  {f.hidden && <span className="muted small"> (hidden)</span>}
                </td>
                <td className="mono small">{f.keys}</td>
                <td className="small">{f.reason}</td>
                <td>{formatBytes(f.size)}</td>
                <td className="row-buttons">
                  {!conn.readOnly && (
                    <>
                      <button
                        className="small ghost"
                        disabled={busy}
                        onClick={() => act(() => backend.modifyIndex(conn.id, db, f.coll, f.name, { hidden: !f.hidden }), `${f.hidden ? 'Unhid' : 'Hid'} ${f.name}`)}
                      >
                        {f.hidden ? 'Unhide' : 'Hide'}
                      </button>
                      <button className="small ghost danger-text" disabled={busy} onClick={() => drop(f)} aria-label={`Drop ${f.coll}.${f.name}`}>
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
    </>
  )
  return (
    <Modal title={`Index advisor: ${db}`} onClose={onClose} wide footer={<button onClick={onClose}>Close</button>}>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <div className="field-row">
        <label className="field">
          <span>Collection</span>
          <input value={filterColl} onChange={(e) => setFilterColl(e.target.value)} placeholder="all collections" aria-label="Collection" />
        </label>
        <label className="field narrow">
          <span>Slower than (ms)</span>
          <input type="number" min={0} value={minMillis} onChange={(e) => setMinMillis(Number(e.target.value))} aria-label="Slower than" />
        </label>
        <button onClick={() => void analyze()} disabled={busy}>
          {busy ? <Spinner small /> : 'Analyze'}
        </button>
      </div>
      {advice && (
        <>
          <p className="muted small">
            {advice.analyzed > 0
              ? `${formatCount(advice.analyzed)} profiled operations${advice.since ? ` since ${new Date(advice.since).toLocaleString()}` : ''}. `
              : ''}
            Suggestions put fields matched exactly first, then the sort, then ranges. They are based on recorded queries only: check them against your application.
          </p>
          {advice.notes.map((n) => (
            <p key={n} className="warning-text small">
              {n}
            </p>
          ))}
          {advice.profilingLevel === 0 && !conn.readOnly && (
            <button disabled={busy} onClick={() => act(() => backend.setProfiling(conn.id, db, { level: 1, slowms: 0, sampleRate: 0 }), 'Profiler on (slow operations)')}>
              Turn on the profiler (slow operations)
            </button>
          )}
          <h4>Query shapes, slowest total first</h4>
          {advice.shapes.length === 0 ? (
            <p className="muted small">No profiled queries to analyze.</p>
          ) : (
            <div className="backup-entries">
              <table className="grid compact">
                <thead>
                  <tr>
                    <th>Collection</th>
                    <th>Shape</th>
                    <th>Runs</th>
                    <th>Total / avg ms</th>
                    <th>Examined / returned</th>
                    <th>Status</th>
                    <th>Suggested index</th>
                  </tr>
                </thead>
                <tbody>
                  {advice.shapes.map((s, i) => (
                    <tr key={i} className={open === i ? 'selected' : ''} onClick={() => setOpen(open === i ? null : i)} title="Show details">
                      <td className="mono">{s.coll}</td>
                      <td className="mono small">
                        {s.op !== 'query' && <span className="muted">{s.op} </span>}
                        {s.shape}
                      </td>
                      <td>{formatCount(s.count)}</td>
                      <td>
                        {formatCount(s.totalMillis)} / {formatCount(s.avgMillis)}
                      </td>
                      <td>
                        {formatCount(s.docsExamined)} / {formatCount(s.returned)}
                      </td>
                      <td className={s.status === 'missing' || s.status === 'partial' ? 'warning-text' : ''}>{STATUS_LABEL[s.status]}</td>
                      <td className="mono small">
                        {s.status === 'missing' || s.status === 'partial' ? s.suggested : s.servedBy}
                        {!conn.readOnly && (s.status === 'missing' || s.status === 'partial') && (
                          <button
                            className="small"
                            disabled={busy}
                            onClick={(e) => {
                              e.stopPropagation()
                              void create(s)
                            }}
                          >
                            Create
                          </button>
                        )}
                        {!conn.readOnly && s.status === 'hidden' && s.servedBy && (
                          <button
                            className="small"
                            disabled={busy}
                            onClick={(e) => {
                              e.stopPropagation()
                              void act(() => backend.modifyIndex(conn.id, db, s.coll, s.servedBy!, { hidden: false }), `Unhid ${s.servedBy}`)
                            }}
                          >
                            Unhide
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {open !== null && advice.shapes[open] && (
            <div aria-label="Shape details">
              <p className="small">
                {advice.shapes[open].reason} Plans seen: {advice.shapes[open].plans.join(', ') || 'unknown'}.
              </p>
              <pre className="job-log">{advice.shapes[open].example}</pre>
            </div>
          )}
          {findings('Unused indexes', advice.unused, 'Every index has been used since the server started.')}
          {findings('Redundant indexes', advice.redundant, 'No index is just the start of another one.')}
        </>
      )}
    </Modal>
  )
}
