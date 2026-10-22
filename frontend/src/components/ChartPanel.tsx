import { useEffect, useMemo, useState } from 'react'
import { Chart, type ChartKind, type ChartSeries } from './Chart'
import { ErrorBanner, Spinner } from './common'
import { backend, newOpId } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { ChartData, ChartSpec } from '../lib/types'
import { useApp } from '../state/AppContext'

const KINDS: { value: ChartSpec['kind']; label: string }[] = [
  { value: 'bar', label: 'Bar' },
  { value: 'line', label: 'Line' },
  { value: 'area', label: 'Area' },
  { value: 'pie', label: 'Pie' },
  { value: 'scatter', label: 'Scatter' },
  { value: 'timeseries', label: 'Time series' },
]

export const emptyChart = (): ChartSpec => ({ kind: 'bar', x: '', y: '', agg: 'count', dateUnit: '', series: '', limit: 0 })

/** Collection › Chart view: groups every document matching the current filter
 * on the server and draws the result. Charts can be saved to the library. */
export function ChartPanel({
  connId,
  db,
  coll,
  filter,
  fields,
  initial,
}: {
  connId: string
  db: string
  coll: string
  /** The query's filter, mongosh syntax. */
  filter: string
  fields: string[]
  initial?: ChartSpec
}) {
  const { prompt, toast, toastError } = useApp()
  const [spec, setSpec] = useState<ChartSpec>(initial ?? emptyChart())
  const [data, setData] = useState<ChartData | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [showPipeline, setShowPipeline] = useState(false)
  const set = (patch: Partial<ChartSpec>) => setSpec((s) => ({ ...s, ...patch }))

  const timeSeries = spec.kind === 'timeseries'
  const needsY = spec.kind === 'scatter' || spec.agg !== 'count'
  const ready = !!spec.x && (!needsY || !!spec.y) && (!timeSeries || !!spec.dateUnit)

  useEffect(() => {
    if (!ready) return
    let live = true
    const id = window.setTimeout(async () => {
      setLoading(true)
      setError(null)
      try {
        const d = await backend.runChart(newOpId('chart'), { connId, db, coll, filter, spec })
        if (live) setData(d)
      } catch (err) {
        if (live) setError(toAppError(err))
      } finally {
        if (live) setLoading(false)
      }
    }, 250)
    return () => {
      live = false
      window.clearTimeout(id)
    }
  }, [connId, db, coll, filter, spec, ready])

  const series: ChartSeries[] = useMemo(() => {
    if (!data) return []
    const by = new Map<string, ChartSeries>()
    for (const p of data.points) {
      const name = p.series ?? (spec.kind === 'scatter' ? `${spec.y} by ${spec.x}` : spec.agg === 'count' ? 'documents' : `${spec.agg}(${spec.y})`)
      if (!by.has(name)) by.set(name, { name, points: [] })
      by.get(name)!.points.push({ x: p.x, y: p.y })
    }
    return [...by.values()]
  }, [data, spec])

  const save = async () => {
    const name = await prompt({ title: 'Save chart', label: 'Name', confirmLabel: 'Save' })
    if (!name) return
    try {
      await backend.saveItem({ kind: 'chart', name, db, coll, query: { filter, projection: '', sort: '', skip: 0, limit: 0 }, chart: spec })
      toast({ kind: 'success', title: `Saved "${name}"`, message: 'It is in the Library.' })
    } catch (err) {
      toastError(err, 'Save failed')
    }
  }

  const fieldSelect = (label: string, value: string, onChange: (v: string) => void, optional = false) => (
    <label className="field">
      <span>{label}</span>
      <select value={value} onChange={(e) => onChange(e.target.value)} aria-label={label}>
        <option value="">{optional ? '(none)' : 'Choose a field'}</option>
        {fields.map((f) => (
          <option key={f} value={f}>
            {f}
          </option>
        ))}
      </select>
    </label>
  )

  const chartKind: ChartKind = spec.kind === 'timeseries' ? 'line' : spec.kind
  return (
    <div className="chart-panel">
      <div className="field-row chart-controls">
        <label className="field">
          <span>Chart</span>
          <select value={spec.kind} onChange={(e) => set({ kind: e.target.value as ChartSpec['kind'], dateUnit: e.target.value === 'timeseries' ? spec.dateUnit || 'day' : spec.dateUnit })} aria-label="Chart type">
            {KINDS.map((k) => (
              <option key={k.value} value={k.value}>
                {k.label}
              </option>
            ))}
          </select>
        </label>
        {fieldSelect(spec.kind === 'pie' ? 'Slices' : 'X axis', spec.x, (x) => set({ x }))}
        {spec.kind !== 'scatter' && (
          <label className="field">
            <span>Value</span>
            <select value={spec.agg} onChange={(e) => set({ agg: e.target.value as ChartSpec['agg'] })} aria-label="Aggregation">
              <option value="count">Count of documents</option>
              <option value="sum">Sum of</option>
              <option value="avg">Average of</option>
              <option value="min">Minimum of</option>
              <option value="max">Maximum of</option>
            </select>
          </label>
        )}
        {needsY && fieldSelect(spec.kind === 'scatter' ? 'Y axis' : 'Field', spec.y, (y) => set({ y }))}
        <label className="field">
          <span>Group dates by</span>
          <select value={spec.dateUnit} onChange={(e) => set({ dateUnit: e.target.value as ChartSpec['dateUnit'] })} aria-label="Group dates by" disabled={spec.kind === 'scatter'}>
            {!timeSeries && <option value="">(not a date)</option>}
            <option value="day">Day</option>
            <option value="week">Week</option>
            <option value="month">Month</option>
            <option value="year">Year</option>
          </select>
        </label>
        {spec.kind !== 'pie' && fieldSelect('Split by', spec.series, (series) => set({ series }), true)}
      </div>
      <div className="chart-toolbar">
        {loading && <Spinner small />}
        {data?.truncated && <span className="warning-text small">Only the first {data.points.length} groups or points are shown.</span>}
        <span className="spacer" />
        {data && (
          <button className="small ghost" onClick={() => setShowPipeline(!showPipeline)}>
            {showPipeline ? 'Hide pipeline' : 'Show pipeline'}
          </button>
        )}
        <button className="small" onClick={() => void save()} disabled={!ready}>
          Save chart…
        </button>
      </div>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {!ready && <div className="empty-state">Choose what to chart. It covers every document that matches the filter, grouped on the server.</div>}
      {ready && data && data.points.length === 0 && !loading && <div className="empty-state">No documents match.</div>}
      {ready && data && data.points.length > 0 && (
        <Chart
          title={`${coll}: ${spec.kind === 'scatter' ? `${spec.y} by ${spec.x}` : `${spec.agg === 'count' ? 'documents' : `${spec.agg} of ${spec.y}`} by ${spec.x}`}`}
          kind={chartKind}
          xKind={data.xKind}
          height={320}
          series={series}
        />
      )}
      {showPipeline && data && <pre className="doc-text small">{data.pipeline}</pre>}
    </div>
  )
}
