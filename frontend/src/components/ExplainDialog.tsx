import { useEffect, useState } from 'react'
import { backend, newOpId } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatCount, formatDuration } from '../lib/format'
import type { ExplainPlan, ExplainRequest, ExplainStage } from '../lib/types'
import { ErrorBanner, Modal, Segmented, Spinner } from './common'
import { highlight } from './highlight'
import { PlanTree, stageNames } from './PlanTree'

const STAGE_HELP: Record<string, string> = {
  COLLSCAN: 'Reads every document in the collection',
  IXSCAN: 'Walks an index',
  FETCH: 'Loads full documents for index matches',
  SORT: 'Sorts in memory',
  PROJECTION_COVERED: 'Projection answered from the index',
  PROJECTION_SIMPLE: 'Selects fields',
  PROJECTION_DEFAULT: 'Computes the projection',
  LIMIT: 'Stops after n documents',
  SKIP: 'Skips n documents',
  IDHACK: 'Direct _id lookup',
  EXPRESS_IXSCAN: 'Fast single-document index lookup',
  GROUP: 'Groups documents',
  OR: 'Unions index scans',
  SHARDING_FILTER: 'Drops orphaned documents',
}

function stageClass(name: string): string {
  if (name === 'COLLSCAN') return 'stage-bad'
  if (name === 'SORT') return 'stage-warn'
  if (name.includes('IXSCAN') || name === 'IDHACK' || name === 'PROJECTION_COVERED') return 'stage-good'
  return ''
}

function Metric({ label, value }: { label: string; value?: number | string }) {
  return (
    <div className="metric">
      <div className="metric-value">{value === undefined || value === null ? '—' : typeof value === 'number' ? formatCount(value) : value}</div>
      <div className="metric-label">{label}</div>
    </div>
  )
}

function StageNode({ stage }: { stage: ExplainStage }) {
  const stats = [
    stage.nReturned !== undefined && `${formatCount(stage.nReturned)} returned`,
    stage.docsExamined !== undefined && `${formatCount(stage.docsExamined)} docs examined`,
    stage.keysExamined !== undefined && `${formatCount(stage.keysExamined)} keys examined`,
    stage.timeMs !== undefined && `~${stage.timeMs} ms`,
  ].filter(Boolean)
  return (
    <li>
      <div className={`plan-stage ${stageClass(stage.name)}`}>
        <div className="plan-stage-head">
          <strong>{stage.name}</strong>
          {STAGE_HELP[stage.name] && <span className="muted small">{STAGE_HELP[stage.name]}</span>}
        </div>
        {stats.length > 0 && <div className="plan-stats small">{stats.join(' · ')}</div>}
        {stage.details.length > 0 && (
          <dl className="plan-details">
            {stage.details.map((d) => (
              <div key={d.key}>
                <dt>{d.key}</dt>
                <dd className="mono">{d.value}</dd>
              </div>
            ))}
          </dl>
        )}
      </div>
      {stage.children.length > 0 && (
        <ul className="plan-children">
          {stage.children.map((c, i) => (
            <StageNode key={i} stage={c} />
          ))}
        </ul>
      )}
    </li>
  )
}

type ExplainView = 'tree' | 'list' | 'raw' | 'compare'

/** Runs explain(executionStats) and visualizes the plan with diagnostics.
 * compareWith (e.g. the plan before an index was created) opens the Compare
 * view; "Run again to compare" keeps the current plan as the earlier one. */
export function ExplainDialog({
  request,
  onClose,
  compareWith,
  compareLabels = ['Before', 'After'],
}: {
  request: ExplainRequest
  onClose: () => void
  compareWith?: ExplainPlan
  compareLabels?: [string, string]
}) {
  const [plan, setPlan] = useState<ExplainPlan | null>(null)
  const [before, setBefore] = useState<ExplainPlan | null>(compareWith ?? null)
  const [labels, setLabels] = useState<[string, string]>(compareLabels)
  const [error, setError] = useState<AppError | null>(null)
  const [view, setView] = useState<ExplainView>(compareWith ? 'compare' : 'tree')
  const [opId] = useState(() => newOpId('explain'))
  const [run, setRun] = useState(0)

  useEffect(() => {
    let cancelled = false
    setPlan(null)
    backend
      .explain(opId, request)
      .then((p) => !cancelled && setPlan(p))
      .catch((err) => !cancelled && setError(toAppError(err)))
    return () => {
      cancelled = true
      backend.cancel(opId).catch(() => undefined)
    }
  }, [opId, request, run])

  const runAgain = () => {
    if (!plan) return
    setBefore(plan)
    setLabels(['Earlier run', 'This run'])
    setView('compare')
    setRun((n) => n + 1)
  }

  const s = plan?.summary
  return (
    <Modal title="Explain plan" onClose={onClose} wide>
      <ErrorBanner error={error} />
      {!plan && !error && (
        <div className="empty-state">
          <Spinner /> Running the query with execution statistics…
        </div>
      )}
      {plan && s && (
        <div className="explain">
          <div className="explain-head">
            <span className="muted small">
              {plan.namespace} · {s.engine === 'sbe' ? 'slot-based engine' : s.engine === 'classic' ? 'classic engine' : 'engine unknown'}
            </span>
            <span className="spacer" />
            <button className="small" onClick={runAgain} title="Keep this plan and explain again, e.g. after creating an index">
              Run again to compare
            </button>
            <Segmented
              label="Explain view"
              value={view}
              onChange={setView}
              options={[
                { value: 'tree' as ExplainView, label: 'Tree' },
                { value: 'list' as ExplainView, label: 'List' },
                ...(before ? [{ value: 'compare' as ExplainView, label: 'Compare' }] : []),
                { value: 'raw' as ExplainView, label: 'Raw' },
              ]}
            />
          </div>
          {view === 'raw' ? (
            <pre className="doc-text explain-raw">{highlight(plan.raw)}</pre>
          ) : view === 'compare' && before ? (
            <PlanComparison before={before} after={plan} labels={labels} />
          ) : (
            <>
              <div className="metrics">
                <Metric label="Execution time" value={s.executionTimeMs !== undefined ? formatDuration(s.executionTimeMs) : undefined} />
                <Metric label="Returned" value={s.nReturned} />
                <Metric label="Docs examined" value={s.docsExamined} />
                <Metric label="Keys examined" value={s.keysExamined} />
                <Metric label="Index" value={s.indexesUsed.length ? s.indexesUsed.join(', ') : s.collScan ? 'none (COLLSCAN)' : '—'} />
              </div>
              <ul className="diagnostics">
                {plan.diagnostics.map((d, i) => (
                  <li key={i} className={`diag diag-${d.level}`}>
                    <strong>{d.title}</strong>
                    <span>{d.detail}</span>
                  </li>
                ))}
              </ul>
              {plan.pipeline.length > 0 && (
                <>
                  <h3>Pipeline stages after the query</h3>
                  <ol className="plan-pipeline">
                    {plan.pipeline.map((st, i) => (
                      <li key={i} className="plan-stage">
                        <div className="plan-stage-head">
                          <strong className="mono">{st.name}</strong>
                          {st.nReturned !== undefined && <span className="small">{formatCount(st.nReturned)} out</span>}
                          {st.timeMs !== undefined && <span className="muted small">~{st.timeMs} ms</span>}
                        </div>
                        {st.details.map((d) => (
                          <div key={d.key} className="mono small muted">
                            {d.value}
                          </div>
                        ))}
                      </li>
                    ))}
                  </ol>
                </>
              )}
              {plan.root && (
                <>
                  <h3>Winning plan {plan.pipeline.length > 0 && '(query layer)'}</h3>
                  <p className="muted small">Read bottom-up: leaf stages produce documents for their parents.{view === 'tree' && ' Click a stage for its details.'}</p>
                  {view === 'tree' ? (
                    <PlanTree root={plan.root} />
                  ) : (
                    <ul className="plan-tree">
                      <StageNode stage={plan.root} />
                    </ul>
                  )}
                </>
              )}
            </>
          )}
        </div>
      )}
    </Modal>
  )
}

type Num = number | undefined

function change(a: Num, b: Num, lowerIsBetter = true): { text: string; tone: string } {
  if (a === undefined || b === undefined) return { text: '', tone: '' }
  if (a === b) return { text: 'same', tone: '' }
  const pct = a === 0 ? null : Math.round(((b - a) / a) * 100)
  const better = lowerIsBetter ? b < a : b > a
  return { text: pct === null ? (b > a ? 'up' : 'down') : `${pct > 0 ? '+' : ''}${pct}%`, tone: better ? 'better' : 'worse' }
}

/** Two plans side by side: the numbers with their change, what changed in the
 * stages, and both trees. */
export function PlanComparison({ before, after, labels }: { before: ExplainPlan; after: ExplainPlan; labels: [string, string] }) {
  const a = before.summary
  const b = after.summary
  const rows: { label: string; a: Num; b: Num; fmt?: (n: number) => string; lowerIsBetter?: boolean }[] = [
    { label: 'Execution time', a: a.executionTimeMs, b: b.executionTimeMs, fmt: formatDuration },
    { label: 'Docs examined', a: a.docsExamined, b: b.docsExamined },
    { label: 'Keys examined', a: a.keysExamined, b: b.keysExamined },
    { label: 'Returned', a: a.nReturned, b: b.nReturned, lowerIsBetter: false },
  ]
  const fmt = (n: Num, f?: (n: number) => string) => (n === undefined ? '—' : f ? f(n) : formatCount(n))
  const onlyA = stageNames(before.root).filter((n) => !stageNames(after.root).includes(n))
  const onlyB = stageNames(after.root).filter((n) => !stageNames(before.root).includes(n))
  const yesNo = (v: boolean) => (v ? 'yes' : 'no')
  return (
    <div className="plan-compare">
      <table className="grid plan-compare-table">
        <thead>
          <tr>
            <th />
            <th>{labels[0]}</th>
            <th>{labels[1]}</th>
            <th>Change</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => {
            const c = change(r.a, r.b, r.lowerIsBetter ?? true)
            return (
              <tr key={r.label}>
                <td>{r.label}</td>
                <td className="mono">{fmt(r.a, r.fmt)}</td>
                <td className="mono">{fmt(r.b, r.fmt)}</td>
                <td className={`mono change-${c.tone}`}>{c.text}</td>
              </tr>
            )
          })}
          <tr>
            <td>Index used</td>
            <td className="mono">{a.indexesUsed.join(', ') || (a.collScan ? 'none (COLLSCAN)' : '—')}</td>
            <td className="mono">{b.indexesUsed.join(', ') || (b.collScan ? 'none (COLLSCAN)' : '—')}</td>
            <td className={a.collScan && !b.collScan ? 'change-better' : !a.collScan && b.collScan ? 'change-worse' : ''}>
              {a.collScan && !b.collScan ? 'no more collection scan' : !a.collScan && b.collScan ? 'now scans the collection' : ''}
            </td>
          </tr>
          <tr>
            <td>In-memory sort</td>
            <td>{yesNo(a.inMemorySort)}</td>
            <td>{yesNo(b.inMemorySort)}</td>
            <td className={a.inMemorySort && !b.inMemorySort ? 'change-better' : !a.inMemorySort && b.inMemorySort ? 'change-worse' : ''} />
          </tr>
        </tbody>
      </table>
      {(onlyA.length > 0 || onlyB.length > 0) && (
        <p className="small">
          Stages: {onlyA.length > 0 && <span className="mono change-worse">−{[...new Set(onlyA)].join(', −')} </span>}
          {onlyB.length > 0 && <span className="mono change-better">+{[...new Set(onlyB)].join(', +')}</span>}
        </p>
      )}
      <div className="plan-compare-trees">
        <div>
          <h3>{labels[0]}</h3>
          {before.root ? <PlanTree root={before.root} label={`${labels[0]} plan`} /> : <p className="muted small">No query-layer plan.</p>}
        </div>
        <div>
          <h3>{labels[1]}</h3>
          {after.root ? <PlanTree root={after.root} label={`${labels[1]} plan`} /> : <p className="muted small">No query-layer plan.</p>}
        </div>
      </div>
    </div>
  )
}
