import { useEffect, useState } from 'react'
import { backend, newOpId } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { AIContext, AIProposal, AIRequest, AIStatus, AIWritePreview, AIWriteResult } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Modal, Segmented, Spinner } from './common'
import { highlight } from './highlight'
import { Checks, ConfirmInput } from './p2common'

/** Natural language → validated query proposal. Never executes on its own. */
export function AIDialog({
  connId,
  db,
  coll,
  initialMode = 'auto',
  onUse,
  onClose,
}: {
  connId: string
  db: string
  coll: string
  initialMode?: AIRequest['mode']
  onUse: (p: AIProposal) => void
  onClose: () => void
}) {
  const { dispatchTabs, toast } = useApp()
  const [status, setStatus] = useState<AIStatus | null>(null)
  const [req, setReq] = useState<AIRequest>({ connId, db, coll, prompt: '', mode: initialMode, includeSamples: false, allowWrites: false })
  const [context, setContext] = useState<AIContext | null>(null)
  const [proposal, setProposal] = useState<AIProposal | null>(null)
  const [writePreview, setWritePreview] = useState<AIWritePreview | null>(null)
  const [confirm, setConfirm] = useState('')
  const [result, setResult] = useState<AIWriteResult | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [opId] = useState(() => newOpId('ai'))

  useEffect(() => {
    backend.getAIStatus().then(setStatus).catch(() => undefined)
  }, [])

  const showContext = async () => {
    try {
      setContext(await backend.previewAIContext(req))
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const generate = async () => {
    setBusy(true)
    setError(null)
    setProposal(null)
    setWritePreview(null)
    setResult(null)
    try {
      setProposal(await backend.generateAI(opId, req))
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  const reviewWrite = async () => {
    if (!proposal) return
    try {
      setWritePreview(await backend.previewAIWrite(proposal.id))
      setConfirm('')
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const executeWrite = async () => {
    if (!writePreview) return
    setBusy(true)
    try {
      setResult(await backend.executeAIWrite(writePreview.proposalId, confirm))
      setWritePreview(null)
      toast({ kind: 'success', title: 'Write executed', message: 'A backup of the affected documents was saved.' })
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  const usable = status?.enabled && status.hasKey

  return (
    <Modal title="Ask AI" onClose={onClose} wide>
      {status && !usable && (
        <div className="info-banner">
          {status.enabled ? 'Add an Anthropic API key in Settings → AI (or set ANTHROPIC_API_KEY).' : 'AI assistance is off. Enable it in Settings → AI.'}
          <button
            className="small"
            onClick={() => {
              dispatchTabs({ type: 'open', tab: { kind: 'settings' } })
              onClose()
            }}
          >
            Open Settings
          </button>
        </div>
      )}
      <label className="field">
        <span>
          Describe what you want from <code>{db}.{coll}</code>
        </span>
        <textarea
          rows={3}
          value={req.prompt}
          onChange={(e) => setReq({ ...req, prompt: e.target.value })}
          placeholder="e.g. orders shipped to Paris last month, newest first"
          onKeyDown={(e) => (e.metaKey || e.ctrlKey) && e.key === 'Enter' && generate()}
          autoFocus
        />
      </label>
      <div className="field-row">
        <Segmented
          label="Kind"
          value={req.mode}
          onChange={(mode) => setReq({ ...req, mode })}
          options={[
            { value: 'auto', label: 'Auto' },
            { value: 'find', label: 'Query' },
            { value: 'aggregate', label: 'Pipeline' },
          ]}
        />
        <label className="check" title={status?.allowSamples ? '' : 'Allow masked samples in Settings → AI first'}>
          <input type="checkbox" disabled={!status?.allowSamples} checked={req.includeSamples} onChange={(e) => setReq({ ...req, includeSamples: e.target.checked })} />
          Include 3 masked sample documents
        </label>
        <label className="check">
          <input type="checkbox" checked={req.allowWrites} onChange={(e) => setReq({ ...req, allowWrites: e.target.checked })} />
          Allow write proposals
        </label>
        <span className="spacer" />
        <button className="small ghost" onClick={showContext}>
          What is sent?
        </button>
        <button className="primary" onClick={generate} disabled={busy || !usable || !req.prompt.trim()}>
          {busy && <Spinner small />} Generate
        </button>
      </div>
      {context && (
        <details open className="sent-context">
          <summary>
            Sent to {status?.model}: collection name and {context.fields.length} field names/types{context.samples.length ? `, ${context.samples.length} masked samples` : ''}. No connection details or credentials.
          </summary>
          <pre className="doc-text small">{context.fields.map((f) => `${f.path}: ${f.types.join('|')} (${f.presence}%)`).join('\n')}</pre>
          {context.samples.map((s, i) => (
            <pre key={i} className="doc-text small">
              {highlight(s)}
            </pre>
          ))}
        </details>
      )}
      <ErrorBanner error={error} onDismiss={() => setError(null)} />

      {proposal && (
        <section className="panel">
          <div className="qb-head">
            <strong>{proposal.kind}</strong>
            {proposal.isWrite && <span className="badge danger-badge">writes data</span>}
            <span className="spacer" />
            <span className="muted small">
              {proposal.model} · {proposal.inputTokens}/{proposal.outputTokens} tokens
            </span>
          </div>
          <p>{proposal.explanation}</p>
          {proposal.warnings.map((w) => (
            <div key={w} className="info-banner small">
              {w}
            </div>
          ))}
          <pre className="doc-text">{highlight(proposal.shell)}</pre>
          <Checks checks={proposal.validation} />
          <div className="form-actions">
            <span className="muted small">Nothing has run. Review the query before using it.</span>
            <span className="spacer" />
            {!proposal.isWrite && proposal.valid && (
              <button
                className="primary"
                onClick={() => {
                  onUse(proposal)
                  onClose()
                }}
              >
                {proposal.kind === 'aggregate' ? 'Open in aggregation editor' : 'Load into query editor'}
              </button>
            )}
            {proposal.isWrite && proposal.valid && proposal.kind !== 'aggregate' && (
              <button className="danger" onClick={reviewWrite}>
                Review write…
              </button>
            )}
          </div>
        </section>
      )}

      {writePreview && (
        <section className="panel">
          <p>
            This will {proposal?.kind === 'deleteMany' ? 'delete' : 'update'} <strong>{writePreview.affected}</strong> document(s) in <code>{writePreview.confirmText}</code>
            {writePreview.sampleIds.length > 0 && <> (e.g. {writePreview.sampleIds.slice(0, 5).join(', ')})</>}. Their current versions are backed up first.
          </p>
          <ConfirmInput text={writePreview.confirmText} value={confirm} onChange={setConfirm} action="Runs an AI-generated write" />
          <div className="form-actions">
            <button onClick={() => setWritePreview(null)}>Cancel</button>
            <button className="danger" disabled={busy || confirm !== writePreview.confirmText} onClick={executeWrite}>
              Execute
            </button>
          </div>
        </section>
      )}
      {result && (
        <div className="success-banner">
          Done: {result.modified ? `${result.modified} modified` : `${result.deleted} deleted`}. Backup: <code>{result.backupPath}</code>
        </div>
      )}
    </Modal>
  )
}
