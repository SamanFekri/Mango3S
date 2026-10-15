import { useEffect, useReducer, useRef, useState } from 'react'
import { CodeDialog } from '../components/CodeDialog'
import { CodeEditor } from '../components/CodeEditor'
import { ExplainDialog } from '../components/ExplainDialog'
import { ErrorBanner, Spinner } from '../components/common'
import { ResultsView, type ViewMode } from '../components/ResultsView'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { STAGE_TEMPLATES, newStage, pipelineReducer, toStages } from '../lib/pipeline'
import type { Tab } from '../lib/tabs'
import type { AggregateRequest, CodeRequest, ExplainRequest, Page } from '../lib/types'
import { useApp } from '../state/AppContext'

export function AggregationView({ tab }: { tab: Tab }) {
  const { settings, sessions, prompt, toast, toastError } = useApp()
  const connId = tab.connId!
  const db = tab.db!
  const coll = tab.coll!
  const readOnly = !!sessions.find((s) => s.id === connId)?.readOnly
  const opId = `agg-${tab.id}`

  const [stages, dispatch] = useReducer(pipelineReducer, undefined, () =>
    tab.init?.stages?.length ? pipelineReducer([], { type: 'replace', stages: tab.init.stages }) : [newStage('$match')],
  )
  const [savedId, setSavedId] = useState(tab.init?.savedId)
  const [dialog, setDialog] = useState<{ kind: 'explain'; req: ExplainRequest } | { kind: 'code'; req: Omit<CodeRequest, 'language'> } | null>(null)
  const [allowDiskUse, setAllowDiskUse] = useState(false)
  const [view, setView] = useState<ViewMode>(settings?.defaultView ?? 'json')
  const [pageSize, setPageSize] = useState(settings?.pageSize ?? 50)
  const [page, setPage] = useState<Page | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [previewing, setPreviewing] = useState<number>(-1)
  const [fields, setFields] = useState<string[]>([])
  const [textMode, setTextMode] = useState<{ text: string; error: AppError | null } | null>(null)
  const lastReq = useRef<AggregateRequest | null>(null)

  useEffect(() => {
    backend.sampleFields(connId, db, coll).then(setFields).catch(() => undefined)
    return () => {
      backend.cancel(opId).catch(() => undefined)
    }
  }, [connId, db, coll, opId])

  const execute = async (req: AggregateRequest) => {
    setLoading(true)
    setError(null)
    try {
      const p = await backend.runAggregation(opId, req)
      lastReq.current = req
      setPage(p)
      setPreviewing(req.upTo)
    } catch (err) {
      const e = toAppError(err)
      if (e.code !== 'CANCELED') setError(e)
    } finally {
      setLoading(false)
    }
  }

  const run = (upTo = -1) =>
    execute({ connId, db, coll, stages: toStages(stages), upTo, page: 0, pageSize, allowDiskUse })

  const openText = async () => {
    try {
      setTextMode({ text: await backend.pipelineToText(toStages(stages)), error: null })
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const applyText = async () => {
    if (!textMode) return
    try {
      const parsed = await backend.pipelineFromText(textMode.text)
      dispatch({ type: 'replace', stages: parsed })
      setTextMode(null)
    } catch (err) {
      setTextMode({ ...textMode, error: toAppError(err) })
    }
  }

  const enabledCount = stages.filter((s) => s.enabled).length

  const savePipeline = async () => {
    const name = await prompt({ title: 'Save pipeline', label: 'Name', initial: tab.init?.savedName ?? `${coll} pipeline`, confirmLabel: 'Save' })
    if (!name) return
    try {
      const item = await backend.saveItem({ id: savedId, kind: 'aggregation', name, db, coll, stages: toStages(stages) })
      setSavedId(item.id)
      toast({ kind: 'success', title: `Saved "${item.name}"` })
    } catch (err) {
      toastError(err, 'Save failed')
    }
  }

  return (
    <div className="view aggregation-view">
      <div className="view-header">
        <div className="breadcrumbs">
          <span>{tab.connName}</span>
          <span className="sep">/</span>
          <span>{db}</span>
          <span className="sep">/</span>
          <strong>{coll}</strong>
          <span className="muted">· aggregation</span>
        </div>
        <span className="spacer" />
        <label className="check small">
          <input type="checkbox" checked={allowDiskUse} onChange={(e) => setAllowDiskUse(e.target.checked)} />
          allowDiskUse
        </label>
        <button className="small ghost" disabled={!!textMode} onClick={() => setDialog({ kind: 'explain', req: { kind: 'aggregate', connId, db, coll, stages: toStages(stages) } })}>
          Explain
        </button>
        <button className="small ghost" disabled={!!textMode} onClick={() => setDialog({ kind: 'code', req: { source: 'aggregate', db, coll, stages: toStages(stages) } })}>
          Code
        </button>
        <button className="small ghost" disabled={!!textMode} onClick={savePipeline}>
          Save
        </button>
        {textMode ? (
          <>
            <button className="small" onClick={() => setTextMode(null)}>
              Discard text
            </button>
            <button className="small primary" onClick={applyText}>
              Apply text
            </button>
          </>
        ) : (
          <button className="small ghost" onClick={openText}>
            Edit as text
          </button>
        )}
        <button className="primary" onClick={() => run(-1)} disabled={loading || !!textMode || enabledCount === 0} title="Run full pipeline (Cmd/Ctrl+Enter)">
          {loading ? <Spinner small /> : '▶'} Run pipeline
        </button>
      </div>

      <div className="split">
        <div className="pipeline-pane">
          {textMode ? (
            <div className="pipeline-text">
              <ErrorBanner error={textMode.error} onDismiss={() => setTextMode({ ...textMode, error: null })} />
              <CodeEditor
                value={textMode.text}
                onChange={(text) => setTextMode((t) => (t ? { ...t, text } : t))}
                onRun={applyText}
                fields={fields}
                minHeight={300}
                ariaLabel="Pipeline text"
              />
              <p className="muted small">Paste a pipeline array from mongosh or Compass. Commented-out stages are dropped on apply.</p>
            </div>
          ) : (
            <ol className="stages">
              {stages.map((s, i) => (
                <li key={s.key} className={`stage-card${s.enabled ? '' : ' disabled'}${previewing === i ? ' previewing' : ''}`}>
                  <div className="stage-head">
                    <input type="checkbox" checked={s.enabled} onChange={() => dispatch({ type: 'toggle', index: i })} aria-label={`Enable stage ${i + 1}`} />
                    <span className="stage-num">{i + 1}</span>
                    <select
                      value={STAGE_TEMPLATES.some((t) => t.operator === s.operator) ? s.operator : '__custom'}
                      onChange={(e) => e.target.value !== '__custom' && dispatch({ type: 'update', index: i, patch: { operator: e.target.value } })}
                      aria-label={`Stage ${i + 1} operator`}
                      title={STAGE_TEMPLATES.find((t) => t.operator === s.operator)?.hint}
                    >
                      {STAGE_TEMPLATES.map((t) => (
                        <option key={t.operator} value={t.operator}>
                          {t.operator}
                        </option>
                      ))}
                      {!STAGE_TEMPLATES.some((t) => t.operator === s.operator) && <option value="__custom">{s.operator}</option>}
                    </select>
                    <span className="spacer" />
                    <button className="icon-btn" title="Move up" aria-label="Move stage up" disabled={i === 0} onClick={() => dispatch({ type: 'move', from: i, to: i - 1 })}>
                      ↑
                    </button>
                    <button
                      className="icon-btn"
                      title="Move down"
                      aria-label="Move stage down"
                      disabled={i === stages.length - 1}
                      onClick={() => dispatch({ type: 'move', from: i, to: i + 1 })}
                    >
                      ↓
                    </button>
                    <button className="icon-btn" title="Duplicate" aria-label="Duplicate stage" onClick={() => dispatch({ type: 'duplicate', index: i })}>
                      ⧉
                    </button>
                    <button className="icon-btn" title="Remove" aria-label="Remove stage" onClick={() => dispatch({ type: 'remove', index: i })}>
                      ×
                    </button>
                  </div>
                  <CodeEditor
                    value={s.body}
                    onChange={(body) => dispatch({ type: 'update', index: i, patch: { body } })}
                    onRun={() => run(-1)}
                    fields={fields}
                    compact
                    maxHeight={260}
                    ariaLabel={`Stage ${i + 1} body`}
                  />
                  <div className="stage-foot">
                    <button className="small ghost" onClick={() => run(i)} disabled={loading} title="Run stages 1 through this one">
                      Preview output of stage {i + 1}
                    </button>
                    <button className="small ghost" onClick={() => dispatch({ type: 'add', after: i })}>
                      + Insert stage below
                    </button>
                  </div>
                </li>
              ))}
            </ol>
          )}
          {!textMode && (
            <button className="block" onClick={() => dispatch({ type: 'add' })}>
              + Add stage
            </button>
          )}
          {readOnly && <p className="muted small">Read-only connection: $out and $merge are blocked.</p>}
        </div>

        <ResultsView
          page={page}
          loading={loading}
          error={error}
          onDismissError={() => setError(null)}
          view={view}
          onViewChange={setView}
          pageSize={pageSize}
          onPageSize={(n) => {
            setPageSize(n)
            if (lastReq.current) execute({ ...lastReq.current, page: 0, pageSize: n })
          }}
          onPage={(n) => lastReq.current && execute({ ...lastReq.current, page: n })}
          onCancel={() => backend.cancel(opId)}
          emptyHint={previewing >= 0 ? undefined : 'Run the pipeline, or preview the output of a single stage.'}
        />
      </div>
      {dialog?.kind === 'explain' && <ExplainDialog request={dialog.req} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'code' && <CodeDialog request={dialog.req} onClose={() => setDialog(null)} />}
    </div>
  )
}
