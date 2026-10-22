import { useEffect, useMemo, useRef, useState } from 'react'

/** A small dependency-free SVG chart used by the server monitor and the
 * Chart result view: line, area, bar, pie and scatter, over time, number or
 * category axes. */

export type ChartKind = 'line' | 'area' | 'bar' | 'pie' | 'scatter'
export type XKind = 'time' | 'number' | 'category'

export interface ChartPoint {
  /** Milliseconds for time axes, a number, or a category label. */
  x: number | string
  y: number
}

export interface ChartSeries {
  name: string
  points: ChartPoint[]
  color?: string
}

/** Categorical colours that read on both light and dark backgrounds. */
export const PALETTE = ['#3b82f6', '#f08c2a', '#22a06b', '#d946ef', '#ef4444', '#14b8a6', '#ca8a04', '#8b5cf6', '#64748b', '#0ea5e9']

const M = { top: 10, right: 12, bottom: 26, left: 52 }

function niceStep(range: number, ticks: number): number {
  const raw = range / Math.max(1, ticks)
  const mag = Math.pow(10, Math.floor(Math.log10(raw || 1)))
  const norm = raw / mag
  return (norm < 1.5 ? 1 : norm < 3 ? 2 : norm < 7 ? 5 : 10) * mag
}

/** "Nice" y-axis ticks covering [min, max]. */
export function yTicks(min: number, max: number, count = 4): number[] {
  if (min === max) {
    max = min === 0 ? 1 : min + Math.abs(min)
  }
  const step = niceStep(max - min, count)
  const lo = Math.floor(min / step) * step
  const out: number[] = []
  // Always end at or above the maximum, so no value is drawn off the axis.
  for (let v = lo; out.length < 50; v += step) {
    out.push(Number(v.toFixed(10)))
    if (v >= max) break
  }
  return out
}

export function formatNumber(n: number): string {
  const a = Math.abs(n)
  if (a >= 1e9) return `${(n / 1e9).toFixed(a >= 1e10 ? 0 : 1)}G`
  if (a >= 1e6) return `${(n / 1e6).toFixed(a >= 1e7 ? 0 : 1)}M`
  if (a >= 1e3) return `${(n / 1e3).toFixed(a >= 1e4 ? 0 : 1)}k`
  if (a > 0 && a < 1) return n.toFixed(2)
  return String(Math.round(n * 10) / 10)
}

function formatTime(ms: number, spanMs: number): string {
  const d = new Date(ms)
  if (spanMs > 2 * 86400000) return d.toLocaleDateString()
  if (spanMs > 3600000) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function useWidth(fallback: number) {
  const ref = useRef<HTMLDivElement>(null)
  const [w, setW] = useState(fallback)
  useEffect(() => {
    const el = ref.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(() => setW(Math.max(160, el.clientWidth)))
    ro.observe(el)
    setW(Math.max(160, el.clientWidth || fallback))
    return () => ro.disconnect()
  }, [fallback])
  return [ref, w] as const
}

export function Chart({
  kind,
  series,
  xKind = 'time',
  height = 160,
  formatY = formatNumber,
  title,
  yMin,
}: {
  kind: ChartKind
  series: ChartSeries[]
  xKind?: XKind
  height?: number
  formatY?: (n: number) => string
  title: string
  /** Lowest value of the y axis (default: 0, or the data's minimum if negative). */
  yMin?: number
}) {
  const [ref, width] = useWidth(480)
  const [hover, setHover] = useState<number | null>(null)
  const colored = series.map((s, i) => ({ ...s, color: s.color ?? PALETTE[i % PALETTE.length]! }))

  const iw = Math.max(10, width - M.left - M.right)
  const ih = Math.max(10, height - M.top - M.bottom)

  // X domain
  const categories = useMemo(() => {
    if (xKind !== 'category' && kind !== 'bar') return []
    const seen: string[] = []
    for (const s of colored) for (const p of s.points) if (!seen.includes(String(p.x))) seen.push(String(p.x))
    return seen
  }, [colored, xKind, kind])
  if (kind === 'pie') return <Pie title={title} series={colored} height={height} formatY={formatY} />
  const categorical = categories.length > 0
  const xs = colored.flatMap((s) => s.points.map((p) => Number(p.x))).filter((n) => Number.isFinite(n))
  const xMin = xs.length ? Math.min(...xs) : 0
  const xMax = xs.length ? Math.max(...xs) : 1
  const ys = colored.flatMap((s) => s.points.map((p) => p.y)).filter((n) => Number.isFinite(n))
  const dataMin = ys.length ? Math.min(...ys) : 0
  const dataMax = ys.length ? Math.max(...ys) : 1
  const ticks = yTicks(yMin ?? Math.min(0, dataMin), dataMax)
  const lo = ticks[0]!
  const hi = ticks[ticks.length - 1]!
  const sy = (v: number) => M.top + ih - ((v - lo) / (hi - lo || 1)) * ih
  const band = categorical ? iw / categories.length : 0
  const sx = (x: number | string) =>
    categorical ? M.left + categories.indexOf(String(x)) * band + band / 2 : M.left + ((Number(x) - xMin) / (xMax - xMin || 1)) * iw

  // Hover: nearest x position.
  const xPositions = categorical ? categories.map((c) => sx(c)) : [...new Set(xs)].sort((a, b) => a - b).map((x) => sx(x))
  const xValues = categorical ? categories : [...new Set(xs)].sort((a, b) => a - b)
  const onMove = (e: React.MouseEvent<SVGSVGElement>) => {
    const r = e.currentTarget.getBoundingClientRect()
    const px = e.clientX - r.left
    let best = -1
    let bestD = Infinity
    xPositions.forEach((p, i) => {
      const d = Math.abs(p - px)
      if (d < bestD) {
        bestD = d
        best = i
      }
    })
    setHover(best >= 0 ? best : null)
  }
  const hx = hover !== null ? xValues[hover] : undefined
  const xLabel = (x: number | string) => (categorical ? String(x) : xKind === 'time' ? formatTime(Number(x), xMax - xMin) : formatNumber(Number(x)))

  const nTimeTicks = Math.min(6, Math.max(2, Math.floor(iw / 90)))
  const xTicks: (number | string)[] = categorical
    ? categories.filter((_c: string, i: number) => i % Math.max(1, Math.ceil(categories.length / Math.max(1, Math.floor(iw / 70)))) === 0)
    : Array.from({ length: nTimeTicks }, (_v: unknown, i: number) => xMin + ((xMax - xMin) * i) / (nTimeTicks - 1 || 1))

  return (
    <figure className="chart" ref={ref}>
      <figcaption className="chart-title">{title}</figcaption>
      <svg width={width} height={height} role="img" aria-label={title} onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
        {ticks.map((t) => (
          <g key={t}>
            <line className="chart-grid" x1={M.left} x2={M.left + iw} y1={sy(t)} y2={sy(t)} />
            <text className="chart-axis" x={M.left - 6} y={sy(t) + 4} textAnchor="end">
              {formatY(t)}
            </text>
          </g>
        ))}
        {xTicks.map((t, i) => (
          <text key={i} className="chart-axis" x={sx(t)} y={height - 8} textAnchor={i === 0 && !categorical ? 'start' : i === xTicks.length - 1 && !categorical ? 'end' : 'middle'}>
            {xLabel(t)}
          </text>
        ))}
        {kind === 'bar' &&
          colored.map((s, si) =>
            s.points.map((p, pi) => {
              const w = (band * 0.8) / colored.length
              const x = sx(p.x) - (band * 0.8) / 2 + si * w
              const y0 = sy(Math.max(lo, 0))
              const y1 = sy(p.y)
              return <rect key={`${si}-${pi}`} x={x} y={Math.min(y0, y1)} width={Math.max(1, w - 1)} height={Math.abs(y0 - y1)} fill={s.color} rx={1.5} />
            }),
          )}
        {(kind === 'line' || kind === 'area') &&
          colored.map((s, si) => {
            const pts = s.points.filter((p) => Number.isFinite(p.y)).map((p) => `${sx(p.x).toFixed(1)},${sy(p.y).toFixed(1)}`)
            if (pts.length === 0) return null
            const base = sy(Math.max(lo, 0)).toFixed(1)
            return (
              <g key={si}>
                {kind === 'area' && (
                  <polygon
                    points={`${pts[0]!.split(',')[0]},${base} ${pts.join(' ')} ${pts[pts.length - 1]!.split(',')[0]},${base}`}
                    fill={s.color}
                    opacity={0.16}
                  />
                )}
                <polyline points={pts.join(' ')} fill="none" stroke={s.color} strokeWidth={1.8} strokeLinejoin="round" />
              </g>
            )
          })}
        {kind === 'scatter' &&
          colored.map((s, si) => s.points.map((p, pi) => <circle key={`${si}-${pi}`} cx={sx(p.x)} cy={sy(p.y)} r={3} fill={s.color} opacity={0.8} />))}
        {hx !== undefined && kind !== 'scatter' && <line className="chart-hover" x1={sx(hx)} x2={sx(hx)} y1={M.top} y2={M.top + ih} />}
      </svg>
      {hx !== undefined && kind !== 'scatter' && (
        <div className="chart-readout small" role="status">
          <span className="muted">{xLabel(hx)}</span>
          {colored.map((s) => {
            const p = s.points.find((q) => String(q.x) === String(hx))
            return p ? (
              <span key={s.name}>
                <i style={{ background: s.color }} /> {s.name} {formatY(p.y)}
              </span>
            ) : null
          })}
        </div>
      )}
      {colored.length > 1 && (
        <div className="chart-legend small">
          {colored.map((s) => (
            <span key={s.name}>
              <i style={{ background: s.color }} /> {s.name}
            </span>
          ))}
        </div>
      )}
    </figure>
  )
}

function Pie({ title, series, height, formatY }: { title: string; series: (ChartSeries & { color: string })[]; height: number; formatY: (n: number) => string }) {
  // A pie shows the first series; each point is a slice.
  const pts = (series[0]?.points ?? []).filter((p) => p.y > 0)
  const total = pts.reduce((n, p) => n + p.y, 0)
  const r = height / 2 - 8
  const cx = r + 8
  const cy = height / 2
  let angle = -Math.PI / 2
  return (
    <figure className="chart">
      <figcaption className="chart-title">{title}</figcaption>
      <div className="chart-pie">
        <svg width={cx * 2} height={height} role="img" aria-label={title}>
          {pts.length === 1 ? (
            <circle cx={cx} cy={cy} r={r} fill={PALETTE[0]} />
          ) : (
            pts.map((p, i) => {
              const a0 = angle
              const a1 = angle + (p.y / total) * Math.PI * 2
              angle = a1
              const large = a1 - a0 > Math.PI ? 1 : 0
              const d = `M${cx},${cy} L${cx + r * Math.cos(a0)},${cy + r * Math.sin(a0)} A${r},${r} 0 ${large} 1 ${cx + r * Math.cos(a1)},${cy + r * Math.sin(a1)} Z`
              return <path key={i} d={d} fill={PALETTE[i % PALETTE.length]} stroke="var(--surface)" strokeWidth={1} />
            })
          )}
        </svg>
        <div className="chart-legend small vertical">
          {pts.map((p, i) => (
            <span key={i}>
              <i style={{ background: PALETTE[i % PALETTE.length] }} /> {String(p.x)} — {formatY(p.y)} ({((p.y / total) * 100).toFixed(1)}%)
            </span>
          ))}
        </div>
      </div>
    </figure>
  )
}
