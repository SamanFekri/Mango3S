import { useCallback, useEffect, useMemo, useState } from 'react'
import { ErrorBanner, Spinner } from '../components/common'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatCount, formatDuration } from '../lib/format'
import type { Tab } from '../lib/tabs'
import type { SchemaField, SchemaReport, TypeStat } from '../lib/types'
import { useApp } from '../state/AppContext'

const TYPE_COLORS: Record<string, string> = {
  string: 'var(--tok-str)',
  int32: 'var(--tok-num)',
  int64: 'var(--tok-num)',
  double: 'var(--tok-num)',
  decimal: 'var(--tok-num)',
  objectId: 'var(--tok-fn)',
  date: 'var(--tok-kw)',
  boolean: 'var(--tok-key)',
  document: 'var(--accent)',
  array: 'var(--accent-hover)',
  null: 'var(--text-faint)',
}

export function TypeBar({ types }: { types: TypeStat[] }) {
  return (
    <div className="type-bar" aria-label={types.map((t) => `${t.type} ${t.percent}%`).join(', ')}>
      {types.map((t) => (
        <span
          key={t.type}
          className="type-seg"
          style={{ width: `${Math.max(t.percent, 2)}%`, background: TYPE_COLORS[t.type] ?? 'var(--border-strong)' }}
          title={`${t.type}: ${formatCount(t.count)} (${t.percent}%)`}
        />
      ))}
    </div>
  )
}

function typeLabel(types: TypeStat[]): string {
  return types.map((t) => (types.length > 1 ? `${t.type} ${t.percent}%` : t.type)).join(' · ')
}

function FieldRow({ f, expanded, onToggle, hasChildren }: { f: SchemaField; expanded: boolean; onToggle: () => void; hasChildren: boolean }) {
  const range = f.min !== undefined ? `${f.min} … ${f.max}` : f.minLength !== undefined ? `len ${f.minLength}–${f.maxLength} (avg ${f.avgLength})` : ''
  return (
    <tr className={f.presence < 100 ? 'sparse' : ''}>
      <td style={{ paddingLeft: `${8 + f.depth * 18}px` }} className="schema-name">
        {hasChildren ? (
          <button className="caret-btn" onClick={onToggle} aria-label={expanded ? 'Collapse' : 'Expand'} aria-expanded={expanded}>
            <span className={expanded ? 'caret open' : 'caret'} />
          </button>
        ) : (
          <span className="caret-spacer" />
        )}
        <span className="mono" title={f.path}>
          {f.name}
        </span>
        {f.unique && <span className="badge">unique</span>}
      </td>
      <td>
        <TypeBar types={f.types} />
        <div className="small muted">{typeLabel(f.types)}</div>
        {f.elementTypes && <div className="small muted">elements: {typeLabel(f.elementTypes)}</div>}
      </td>
      <td className="num">
        <div className="presence">
          <div className="presence-bar" style={{ width: `${f.presence}%` }} />
        </div>
        <span className="small">{f.presence}%</span>
      </td>
      <td className="num small">
        {f.distinctCapped ? `${formatCount(f.distinct)}+` : formatCount(f.distinct)}
      </td>
      <td className="mono small">{range}</td>
      <td className="mono small samples" title={f.samples.join('\n')}>
        {f.samples.slice(0, 3).join(', ')}
      </td>
    </tr>
  )
}

export function SchemaView({ tab }: { tab: Tab }) {
  const { toastError } = useApp()
  const connId = tab.connId!
  const db = tab.db!
  const coll = tab.coll!
  const opId = `schema-${tab.id}`
  const [sampleSize, setSampleSize] = useState(1000)
  const [filter, setFilter] = useState('')
  const [report, setReport] = useState<SchemaReport | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const [search, setSearch] = useState('')

  const run = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setReport(await backend.analyzeSchema(opId, { connId, db, coll, sampleSize, filter }))
    } catch (err) {
      const e = toAppError(err)
      if (e.code !== 'CANCELED') setError(e)
    } finally {
      setLoading(false)
    }
  }, [opId, connId, db, coll, sampleSize, filter])

  useEffect(() => {
    run()
    return () => {
      backend.cancel(opId).catch(() => undefined)
    }
    // Analyze once when the tab opens; later runs are explicit.
  }, [])

  const parents = useMemo(() => {
    const set = new Set<string>()
    report?.fields.forEach((f) => {
      const i = f.path.lastIndexOf('.')
      if (i > 0) set.add(f.path.slice(0, i))
    })
    return set
  }, [report])

  const visible = useMemo(() => {
    if (!report) return []
    const q = search.trim().toLowerCase()
    return report.fields.filter((f) => {
      if (q) return f.path.toLowerCase().includes(q)
      for (const c of collapsed) if (f.path.startsWith(c + '.')) return false
      return true
    })
  }, [report, collapsed, search])

  const copyAsJSON = async () => {
    if (!report) return
    try {
      await navigator.clipboard?.writeText(JSON.stringify(report, null, 2))
    } catch (err) {
      toastError(err)
    }
  }

  return (
    <div className="view schema-view">
      <div className="view-header">
        <div className="breadcrumbs">
          <span>{tab.connName}</span>
          <span className="sep">/</span>
          <span>{db}</span>
          <span className="sep">/</span>
          <strong>{coll}</strong>
          <span className="muted">· schema</span>
        </div>
        <span className="spacer" />
        <label className="inline">
          Sample
          <select value={sampleSize} onChange={(e) => setSampleSize(Number(e.target.value))}>
            {[100, 1000, 5000, 10000, 50000].map((n) => (
              <option key={n} value={n}>
                {formatCount(n)} docs
              </option>
            ))}
          </select>
        </label>
        <input className="mono schema-filter" placeholder="filter, e.g. { status: 'A' }" value={filter} onChange={(e) => setFilter(e.target.value)} />
        {loading ? (
          <button className="small" onClick={() => backend.cancel(opId)}>
            <Spinner small /> Cancel
          </button>
        ) : (
          <button className="primary" onClick={run}>
            Analyze
          </button>
        )}
      </div>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {report && (
        <>
          <div className="schema-summary">
            <span>
              <strong>{formatCount(report.sampled)}</strong> documents sampled of ~{formatCount(report.estimatedDocs)}
            </span>
            <span>
              <strong>{report.fields.length}</strong> field paths
            </span>
            <span className="muted">{formatDuration(report.durationMs)}</span>
            <span className="spacer" />
            <input placeholder="Find field" value={search} onChange={(e) => setSearch(e.target.value)} aria-label="Find field" />
            <button className="small ghost" onClick={() => setCollapsed(new Set(parents))}>
              Collapse all
            </button>
            <button className="small ghost" onClick={() => setCollapsed(new Set())}>
              Expand all
            </button>
            <button className="small ghost" onClick={copyAsJSON}>
              Copy report
            </button>
          </div>
          {report.sampled === 0 ? (
            <div className="empty-state">No documents matched; nothing to analyze.</div>
          ) : (
            <div className="table-wrap">
              <table className="grid schema-table">
                <thead>
                  <tr>
                    <th>Field</th>
                    <th>Types</th>
                    <th>Present</th>
                    <th>Distinct</th>
                    <th>Range / length</th>
                    <th>Samples</th>
                  </tr>
                </thead>
                <tbody>
                  {visible.map((f) => (
                    <FieldRow
                      key={f.path}
                      f={f}
                      hasChildren={parents.has(f.path)}
                      expanded={!collapsed.has(f.path)}
                      onToggle={() =>
                        setCollapsed((c) => {
                          const n = new Set(c)
                          if (n.has(f.path)) n.delete(f.path)
                          else n.add(f.path)
                          return n
                        })
                      }
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <p className="muted small">
            Statistics describe a random sample ($sample). Fields inside arrays of documents are listed under the array (e.g.{' '}
            <code>items.sku</code>); "Present" counts documents that contain the field at least once.
          </p>
        </>
      )}
    </div>
  )
}
