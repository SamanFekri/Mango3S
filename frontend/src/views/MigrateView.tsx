import { Fragment, useCallback, useEffect, useState } from 'react'
import { ErrorBanner, Spinner } from '../components/common'
import { JobStatus, Stat, useJobDone } from '../components/p2common'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatCount } from '../lib/format'
import type { Tab } from '../lib/tabs'
import type { EmbedSpec, MigrationRun, SQLSource, SQLTable, TableMapping } from '../lib/types'
import { useApp } from '../state/AppContext'

const DSN_HINTS: Record<SQLSource['driver'], string> = {
  postgres: 'postgres://reader:password@host:5432/shop',
  mysql: 'reader:password@tcp(host:3306)/shop',
  sqlite: '/path/to/database.sqlite',
}

function collectionName(table: string): string {
  return table.replace(/^.*\./, '').replace(/[^\w-]/g, '_')
}

export function MigrateView({ tab }: { tab: Tab }) {
  const { sessions, confirm, prompt, toast, toastError } = useApp()
  const [source, setSource] = useState<SQLSource>({ driver: 'postgres', dsn: '' })
  const [tables, setTables] = useState<SQLTable[] | null>(null)
  const [mappings, setMappings] = useState<Record<string, TableMapping & { selected: boolean }>>({})
  const [embeds, setEmbeds] = useState<EmbedSpec[]>([])
  const [types, setTypes] = useState<string[]>([])
  const [connId, setConnId] = useState(tab.connId ?? sessions[0]?.id ?? '')
  const [db, setDb] = useState(tab.db ?? '')
  const [onRowError, setOnRowError] = useState<'fail' | 'skip'>('fail')
  const [expanded, setExpanded] = useState<string | null>(null)
  const [runs, setRuns] = useState<MigrationRun[]>([])
  const [jobId, setJobId] = useState<string | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  const [busy, setBusy] = useState(false)

  const loadRuns = useCallback(() => {
    backend.listMigrations().then(setRuns).catch(() => undefined)
  }, [])
  useEffect(() => {
    loadRuns()
    backend.migrationTypes().then(setTypes).catch(() => undefined)
  }, [loadRuns])
  useJobDone(jobId, loadRuns)

  const introspect = async () => {
    setBusy(true)
    setError(null)
    try {
      const t = await backend.introspectSQL(source)
      setTables(t)
      const m: Record<string, TableMapping & { selected: boolean }> = {}
      for (const tb of t) {
        m[tb.name] = {
          table: tb.name,
          collection: collectionName(tb.name),
          idStrategy: tb.primaryKey.length ? 'pk' : 'objectId',
          columns: tb.columns.map((c) => ({ column: c.name, field: c.name, type: c.suggest })),
          embedOnly: false,
          selected: true,
        }
      }
      setMappings(m)
      setEmbeds([])
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  const setMap = (name: string, patch: Partial<TableMapping & { selected: boolean }>) => setMappings({ ...mappings, [name]: { ...mappings[name], ...patch } })

  const start = async () => {
    setError(null)
    try {
      const job = await backend.startMigration({
        source,
        connId,
        db,
        tables: Object.values(mappings)
          .filter((m) => m.selected)
          .map(({ selected: _s, ...m }) => m),
        embeds,
        onRowError,
        batchSize: 1000,
      })
      setJobId(job.id)
      toast({ kind: 'info', title: 'Migration started', message: 'The source is read in one read-only transaction.' })
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const rollback = async (run: MigrationRun) => {
    const typed = await prompt({ title: 'Roll back migration', label: `Type the database name "${run.db}" to drop the ${run.createdCollections.length} collection(s) this run created`, confirmLabel: 'Roll back' })
    if (typed === null) return
    if (!(await confirm({ title: 'Roll back', message: `Drop ${run.createdCollections.join(', ') || 'nothing'} from ${run.db}?`, confirmLabel: 'Drop', danger: true }))) return
    try {
      await backend.rollbackMigration(run.id, typed)
      loadRuns()
      toast({ kind: 'success', title: 'Rolled back' })
    } catch (err) {
      toastError(err, 'Rollback failed')
    }
  }

  const candidates = (tables ?? []).flatMap((t) => t.foreignKeys.filter((fk) => fk.columns.length === 1).map((fk) => ({ child: t.name, parent: fk.refTable, column: fk.columns[0] })))

  return (
    <div className="view migrate-view">
      <div className="view-header">
        <h1>Migrate from SQL</h1>
      </div>
      <section className="panel">
        <div className="field-row">
          <label className="field narrow">
            <span>Database</span>
            <select value={source.driver} onChange={(e) => setSource({ ...source, driver: e.target.value as SQLSource['driver'] })}>
              <option value="postgres">PostgreSQL</option>
              <option value="mysql">MySQL / MariaDB</option>
              <option value="sqlite">SQLite</option>
            </select>
          </label>
          <label className="field">
            <span>Connection string (used for this run only; never saved or logged)</span>
            <input type="password" className="mono" value={source.dsn} placeholder={DSN_HINTS[source.driver]} onChange={(e) => setSource({ ...source, dsn: e.target.value })} autoComplete="off" />
          </label>
          <button onClick={introspect} disabled={busy || !source.dsn}>
            {busy && <Spinner small />} Read schema
          </button>
        </div>
        <p className="muted small">Least privilege: a user with SELECT only is enough. The source is read inside one read-only (snapshot) transaction.</p>
      </section>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />

      {tables && (
        <section className="panel">
          <h3>Tables → collections</h3>
          <table className="grid">
            <thead>
              <tr>
                <th />
                <th>Table</th>
                <th>Rows</th>
                <th>Collection</th>
                <th>_id</th>
                <th>Embed only</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {tables.map((t) => {
                const m = mappings[t.name]
                return (
                  <Fragment key={t.name}>
                    <tr>
                      <td>
                        <input type="checkbox" checked={m.selected} onChange={(e) => setMap(t.name, { selected: e.target.checked })} aria-label={`Migrate ${t.name}`} />
                      </td>
                      <td className="mono">{t.name}</td>
                      <td>{formatCount(t.rows)}</td>
                      <td>
                        <input value={m.collection} onChange={(e) => setMap(t.name, { collection: e.target.value })} disabled={m.embedOnly} />
                      </td>
                      <td>
                        <select value={m.idStrategy} onChange={(e) => setMap(t.name, { idStrategy: e.target.value as TableMapping['idStrategy'] })}>
                          <option value="pk" disabled={!t.primaryKey.length}>
                            primary key ({t.primaryKey.join(', ') || 'none'})
                          </option>
                          <option value="objectId">new ObjectId</option>
                        </select>
                      </td>
                      <td>
                        <input type="checkbox" checked={m.embedOnly} onChange={(e) => setMap(t.name, { embedOnly: e.target.checked })} aria-label={`Embed ${t.name} only`} />
                      </td>
                      <td>
                        <button className="small ghost" onClick={() => setExpanded(expanded === t.name ? null : t.name)}>
                          {expanded === t.name ? 'Hide columns' : 'Columns'}
                        </button>
                      </td>
                    </tr>
                    {expanded === t.name && (
                      <tr>
                        <td colSpan={7}>
                          <table className="grid compact">
                            <thead>
                              <tr>
                                <th>Column</th>
                                <th>SQL type</th>
                                <th>Field (dots nest)</th>
                                <th>BSON type</th>
                              </tr>
                            </thead>
                            <tbody>
                              {m.columns.map((c, i) => (
                                <tr key={c.column}>
                                  <td className="mono">{c.column}</td>
                                  <td className="muted">{t.columns[i]?.dbType}</td>
                                  <td>
                                    <input value={c.field} onChange={(e) => setMap(t.name, { columns: m.columns.map((x, j) => (j === i ? { ...x, field: e.target.value } : x)) })} />
                                  </td>
                                  <td>
                                    <select value={c.type} onChange={(e) => setMap(t.name, { columns: m.columns.map((x, j) => (j === i ? { ...x, type: e.target.value } : x)) })}>
                                      {types.map((ty) => (
                                        <option key={ty}>{ty}</option>
                                      ))}
                                    </select>
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                )
              })}
            </tbody>
          </table>

          {candidates.length > 0 && (
            <>
              <h3>Embed child rows</h3>
              {candidates.map((c) => {
                const on = embeds.some((e) => e.child === c.child && e.parent === c.parent)
                return (
                  <label className="check" key={c.child + c.parent}>
                    <input
                      type="checkbox"
                      checked={on}
                      onChange={(e) =>
                        setEmbeds(e.target.checked ? [...embeds, { child: c.child, parent: c.parent, childColumns: [c.column], as: collectionName(c.child) }] : embeds.filter((x) => !(x.child === c.child && x.parent === c.parent)))
                      }
                    />
                    Embed <code>{c.child}</code> rows into <code>{c.parent}</code> documents as an array (via {c.column})
                  </label>
                )
              })}
            </>
          )}

          <div className="field-row">
            <label className="field">
              <span>Target connection</span>
              <select value={connId} onChange={(e) => setConnId(e.target.value)}>
                {sessions.map((s) => (
                  <option key={s.id} value={s.id} disabled={s.readOnly}>
                    {s.name}
                    {s.readOnly ? ' (read-only)' : ''}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span>Target database</span>
              <input value={db} onChange={(e) => setDb(e.target.value)} spellCheck={false} />
            </label>
            <label className="field narrow">
              <span>Row errors</span>
              <select value={onRowError} onChange={(e) => setOnRowError(e.target.value as 'fail' | 'skip')}>
                <option value="fail">stop</option>
                <option value="skip">skip row</option>
              </select>
            </label>
          </div>
          <div className="form-actions">
            <span className="muted small">Collections must be new or empty. Every run is validated (row counts, sampled rows) and can be rolled back.</span>
            <span className="spacer" />
            <button className="primary" onClick={start} disabled={!connId || !db}>
              Start migration
            </button>
          </div>
        </section>
      )}

      <JobStatus jobId={jobId} />

      {runs.length > 0 && (
        <section className="panel">
          <h3>Migration runs</h3>
          {runs.map((r) => (
            <details key={r.id} className="run" open={r.id === runs[0].id}>
              <summary>
                <span className={`job-state ${r.state === 'succeeded' ? 'succeeded' : r.state === 'failed' ? 'failed' : ''}`}>{r.state}</span> {r.source} → {r.db}{' '}
                <span className="muted small">{new Date(r.created).toLocaleString()}</span>
                {r.state !== 'rolledBack' && r.state !== 'running' && (
                  <button className="small ghost danger-text" onClick={() => rollback(r)}>
                    Roll back
                  </button>
                )}
              </summary>
              <p className="small">{r.summary}</p>
              {r.report && (
                <>
                  <div className="metrics">
                    <Stat label="Validation" value={r.report.valid ? 'passed' : 'failed'} tone={r.report.valid ? 'good' : 'bad'} />
                  </div>
                  <table className="grid compact">
                    <thead>
                      <tr>
                        <th>Table</th>
                        <th>Rows</th>
                        <th>Inserted</th>
                        <th>Skipped</th>
                        <th>Counts</th>
                        <th>Sampled</th>
                        <th>Mismatches</th>
                      </tr>
                    </thead>
                    <tbody>
                      {r.report.tables.map((t) => (
                        <tr key={t.table}>
                          <td className="mono">
                            {t.table} → {t.collection}
                          </td>
                          <td>{formatCount(t.rows)}</td>
                          <td>{formatCount(t.inserted)}</td>
                          <td>{t.skipped}</td>
                          <td>{t.countOk ? '✓' : '✗'}</td>
                          <td>{t.sampleChecked}</td>
                          <td className="small">{t.sampleMismatches.join('; ') || '—'}</td>
                        </tr>
                      ))}
                      {r.report.embeds.map((e) => (
                        <tr key={e.child + e.parent}>
                          <td className="mono">
                            {e.child} ⇢ {e.parent}.{e.as}
                          </td>
                          <td>{e.childRows}</td>
                          <td>{e.embedded}</td>
                          <td>{e.orphans} orphan(s)</td>
                          <td>{e.countOk ? '✓' : '✗'}</td>
                          <td />
                          <td />
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </>
              )}
            </details>
          ))}
        </section>
      )}
    </div>
  )
}
