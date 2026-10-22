import { useCallback, useEffect, useMemo, useState } from 'react'
import { Chart, formatNumber, type ChartSeries } from '../components/Chart'
import { ErrorBanner, Spinner } from '../components/common'
import { backend, onMonitorSample } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatBytes, formatCount } from '../lib/format'
import type { Tab } from '../lib/tabs'
import type { MonitorSample, OpsFilter, RunningOp, ServerOverview } from '../lib/types'
import { useApp } from '../state/AppContext'

const KEEP_MS = 15 * 60 * 1000

function formatUptime(s: number): string {
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  return d > 0 ? `${d}d ${h}h` : h > 0 ? `${h}h ${m}m` : `${m}m`
}

/** Live server monitor for one connection: rates from serverStatus, replica set
 * members and lag, and the running operations (with killOp). Sampling runs only
 * while this tab is open. */
export function MonitorView({ tab }: { tab: Tab }) {
  const connId = tab.connId!
  const { sessions, confirm, toast } = useApp()
  const readOnly = sessions.find((s) => s.id === connId)?.readOnly ?? true
  const [interval, setIntervalSecs] = useState(2)
  const [samples, setSamples] = useState<MonitorSample[]>([])
  const [paused, setPaused] = useState(false)
  const [overview, setOverview] = useState<ServerOverview | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  const [ops, setOps] = useState<RunningOp[] | null>(null)
  const [opsError, setOpsError] = useState<AppError | null>(null)
  const [filter, setFilter] = useState<OpsFilter>({ minSeconds: 0, ns: '', client: '', all: false })
  const [autoOps, setAutoOps] = useState(true)
  const [openOp, setOpenOp] = useState<string | null>(null)

  // Start sampling; samples arrive as events. Stopped when the tab closes.
  useEffect(() => {
    let live = true
    setError(null)
    backend
      .startMonitor(connId, interval)
      .then((snap) => live && setSamples(snap.samples))
      .catch((err) => live && setError(toAppError(err)))
    const off = onMonitorSample((e) => {
      if (e.connId !== connId) return
      setSamples((cur) => {
        const next = [...cur, e.sample]
        const cutoff = Date.parse(e.sample.time) - KEEP_MS
        return next.filter((s) => Date.parse(s.time) >= cutoff)
      })
    })
    return () => {
      live = false
      off()
    }
  }, [connId, interval])
  useEffect(() => () => void backend.stopMonitor(connId).catch(() => undefined), [connId])

  const loadOverview = useCallback(() => {
    backend
      .getServerOverview(connId)
      .then(setOverview)
      .catch((err) => setError(toAppError(err)))
  }, [connId])
  useEffect(() => {
    loadOverview()
    const t = window.setInterval(loadOverview, 10000)
    return () => window.clearInterval(t)
  }, [loadOverview])

  const loadOps = useCallback(() => {
    backend
      .listCurrentOps(connId, filter)
      .then((o) => {
        setOps(o)
        setOpsError(null)
      })
      .catch((err) => setOpsError(toAppError(err)))
  }, [connId, filter])
  useEffect(() => {
    loadOps()
    if (!autoOps) return
    const t = window.setInterval(loadOps, Math.max(2, interval) * 1000)
    return () => window.clearInterval(t)
  }, [loadOps, autoOps, interval])

  const [frozen, setFrozen] = useState<MonitorSample[]>([])
  useEffect(() => {
    if (!paused) setFrozen(samples)
  }, [samples, paused])
  const shown = paused ? frozen : samples
  const rated = useMemo(() => shown.filter((s) => !s.first), [shown])
  const series = (pick: (s: MonitorSample) => number, name: string, color?: string): ChartSeries => ({
    name,
    color,
    points: rated.map((s) => ({ x: Date.parse(s.time), y: pick(s) })),
  })
  const gauge = (pick: (s: MonitorSample) => number, name: string, color?: string): ChartSeries => ({
    name,
    color,
    points: shown.map((s) => ({ x: Date.parse(s.time), y: pick(s) })),
  })
  const last = shown[shown.length - 1]

  const kill = async (op: RunningOp) => {
    const ok = await confirm({
      title: 'Kill operation',
      message: (
        <>
          <p>
            Stop operation <code>{op.opid}</code> ({op.op} on <code>{op.ns || '—'}</code>, running {op.seconds.toFixed(1)} s from {op.client || 'unknown'})?
            The client gets an error.
          </p>
          {op.command && <pre className="doc-text small">{op.command}</pre>}
        </>
      ),
      confirmLabel: 'Kill operation',
      danger: true,
      typeToConfirm: op.opid,
    })
    if (!ok) return
    try {
      await backend.killOperation(connId, op.opid, op.opid)
      toast({ kind: 'success', title: `Killed operation ${op.opid}` })
      loadOps()
    } catch (err) {
      setOpsError(toAppError(err))
    }
  }

  const rs = overview?.replication
  return (
    <div className="view monitor-view">
      <div className="view-header">
        <div className="breadcrumbs">
          <span>{tab.connName}</span>
          <span className="muted">· server monitor</span>
        </div>
        {overview && (
          <span className="muted small">
            {overview.host} · {overview.process} {overview.version} · up {formatUptime(overview.uptime)}
            {overview.storageEngine && ` · ${overview.storageEngine}`}
          </span>
        )}
        <span className="spacer" />
        <label className="inline">
          Every
          <select value={interval} onChange={(e) => setIntervalSecs(Number(e.target.value))} aria-label="Sample interval">
            {[1, 2, 5].map((n) => (
              <option key={n} value={n}>
                {n} s
              </option>
            ))}
          </select>
        </label>
        <button className="small" onClick={() => setPaused(!paused)}>
          {paused ? 'Resume' : 'Pause'}
        </button>
      </div>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {!error && shown.length < 2 && (
        <div className="empty-state">
          <Spinner /> Collecting the first samples…
        </div>
      )}
      {shown.length >= 2 && (
        <div className="monitor-charts">
          <Chart
            title="Operations per second"
            kind="line"
            series={[
              series((s) => s.query, 'query'),
              series((s) => s.insert, 'insert'),
              series((s) => s.update, 'update'),
              series((s) => s.delete, 'delete'),
              series((s) => s.getmore, 'getmore'),
              series((s) => s.command, 'command'),
            ]}
          />
          <Chart
            title="Network per second"
            kind="area"
            formatY={(n) => formatBytes(n)}
            series={[series((s) => s.netIn, 'in'), series((s) => s.netOut, 'out')]}
          />
          <Chart title="Connections" kind="line" series={[gauge((s) => s.connections, 'open')]} />
          <Chart
            title="Queued and active clients"
            kind="line"
            series={[
              gauge((s) => s.queueReaders, 'queued readers', '#ef4444'),
              gauge((s) => s.queueWriters, 'queued writers', '#f97316'),
              gauge((s) => s.activeReaders, 'active readers', '#3b82f6'),
              gauge((s) => s.activeWriters, 'active writers', '#22a06b'),
            ]}
          />
          {last && last.cacheMax > 0 && (
            <Chart
              title={`WiredTiger cache (${formatBytes(last.cacheMax)} configured)`}
              kind="area"
              formatY={(n) => formatBytes(n)}
              series={[gauge((s) => s.cacheBytes, 'used'), gauge((s) => s.cacheDirty, 'dirty', '#ef4444')]}
            />
          )}
        </div>
      )}

      {rs && (
        <section className="monitor-section">
          <h3>
            Replica set {rs.set && <span className="mono">{rs.set}</span>}
            {rs.oplogHours !== undefined && (
              <span className="muted small">
                {' '}
                · oplog window {rs.oplogHours < 1 ? `${Math.round(rs.oplogHours * 60)} min` : `${rs.oplogHours.toFixed(1)} h`}
                {rs.oplogSizeMb !== undefined && ` (${formatCount(Math.round(rs.oplogSizeMb))} MB)`}
              </span>
            )}
          </h3>
          {rs.problem && <p className="warning-text small">{rs.problem}</p>}
          {rs.members && rs.members.length > 0 && (
            <table className="grid compact">
              <thead>
                <tr>
                  <th>Member</th>
                  <th>State</th>
                  <th>Health</th>
                  <th>Lag</th>
                  <th>Ping</th>
                  <th>Syncs from</th>
                </tr>
              </thead>
              <tbody>
                {rs.members.map((m) => (
                  <tr key={m.name}>
                    <td className="mono">
                      {m.name}
                      {m.self && <span className="muted small"> (this server)</span>}
                    </td>
                    <td>{m.state}</td>
                    <td className={m.health ? '' : 'danger-text'}>{m.health ? 'up' : 'down'}</td>
                    <td className={m.lagSeconds !== undefined && m.lagSeconds > 10 ? 'warning-text' : ''}>
                      {m.lagSeconds === undefined ? '—' : `${m.lagSeconds.toFixed(m.lagSeconds < 10 ? 1 : 0)} s`}
                    </td>
                    <td>{m.pingMs === undefined ? '—' : `${m.pingMs} ms`}</td>
                    <td className="mono small">{m.syncSource || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      )}

      <section className="monitor-section">
        <h3>Running operations</h3>
        <div className="field-row">
          <label className="field narrow">
            <span>Running at least (s)</span>
            <input type="number" min={0} step={0.5} value={filter.minSeconds} onChange={(e) => setFilter({ ...filter, minSeconds: Number(e.target.value) })} aria-label="Minimum seconds" />
          </label>
          <label className="field">
            <span>Namespace contains</span>
            <input value={filter.ns} onChange={(e) => setFilter({ ...filter, ns: e.target.value })} placeholder="db.collection" aria-label="Namespace filter" />
          </label>
          <label className="field">
            <span>Client or app contains</span>
            <input value={filter.client} onChange={(e) => setFilter({ ...filter, client: e.target.value })} aria-label="Client filter" />
          </label>
          <label className="check">
            <input type="checkbox" checked={filter.all} onChange={(e) => setFilter({ ...filter, all: e.target.checked })} />
            Include idle and system
          </label>
          <label className="check">
            <input type="checkbox" checked={autoOps} onChange={(e) => setAutoOps(e.target.checked)} />
            Refresh automatically
          </label>
          {!autoOps && <button onClick={loadOps}>Refresh</button>}
        </div>
        <ErrorBanner error={opsError} onDismiss={() => setOpsError(null)} />
        {ops && ops.length === 0 && <p className="muted small">Nothing is running that matches.</p>}
        {ops && ops.length > 0 && (
          <table className="grid compact monitor-ops">
            <thead>
              <tr>
                <th>ID</th>
                <th>Op</th>
                <th>Namespace</th>
                <th>Running</th>
                <th>Client</th>
                <th>Plan</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {ops.map((o) => (
                <tr key={o.opid} className={openOp === o.opid ? 'selected' : ''} onClick={() => setOpenOp(openOp === o.opid ? null : o.opid)} title="Show the command">
                  <td className="mono small">{o.opid}</td>
                  <td>
                    {o.op}
                    {o.waitingForLock && <span className="warning-text small"> waiting for lock</span>}
                  </td>
                  <td className="mono small">{o.ns || '—'}</td>
                  <td className={o.seconds > 10 ? 'warning-text' : ''}>{o.seconds < 10 ? o.seconds.toFixed(2) : formatNumber(o.seconds)} s</td>
                  <td className="small">
                    {o.client}
                    {o.appName && <span className="muted"> · {o.appName}</span>}
                  </td>
                  <td className={`mono small ${o.planSummary.startsWith('COLLSCAN') ? 'warning-text' : ''}`}>{o.planSummary || '—'}</td>
                  <td>
                    {!readOnly && (
                      <button
                        className="small ghost danger-text"
                        onClick={(e) => {
                          e.stopPropagation()
                          void kill(o)
                        }}
                        aria-label={`Kill operation ${o.opid}`}
                      >
                        Kill
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {openOp && ops?.find((o) => o.opid === openOp)?.command && (
          <pre className="doc-text small" aria-label="Operation command">
            {ops.find((o) => o.opid === openOp)!.command}
          </pre>
        )}
      </section>
    </div>
  )
}
