import { useEffect, useState } from 'react'
import { backend, newOpId } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatCount, formatDuration } from '../lib/format'
import type { ExplainPlan, ExplainRequest, ExplainStage } from '../lib/types'
import { ErrorBanner, Modal, Segmented, Spinner } from './common'
import { highlight } from './highlight'

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

/** Runs explain(executionStats) and visualizes the plan with diagnostics. */
export function ExplainDialog({ request, onClose }: { request: ExplainRequest; onClose: () => void }) {
  const [plan, setPlan] = useState<ExplainPlan | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  const [view, setView] = useState<'visual' | 'raw'>('visual')
  const [opId] = useState(() => newOpId('explain'))

  useEffect(() => {
    let cancelled = false
    backend
      .explain(opId, request)
      .then((p) => !cancelled && setPlan(p))
      .catch((err) => !cancelled && setError(toAppError(err)))
    return () => {
      cancelled = true
      backend.cancel(opId).catch(() => undefined)
    }
  }, [opId, request])

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
            <Segmented
              label="Explain view"
              value={view}
              onChange={setView}
              options={[
                { value: 'visual', label: 'Visual' },
                { value: 'raw', label: 'Raw' },
              ]}
            />
          </div>
          {view === 'raw' ? (
            <pre className="doc-text explain-raw">{highlight(plan.raw)}</pre>
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
                  <p className="muted small">Read bottom-up: leaf stages produce documents for their parents.</p>
                  <ul className="plan-tree">
                    <StageNode stage={plan.root} />
                  </ul>
                </>
              )}
            </>
          )}
        </div>
      )}
    </Modal>
  )
}
