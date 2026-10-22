import { useEffect, useState } from 'react'
import { ErrorBanner, Modal, Spinner } from './common'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatCount } from '../lib/format'
import type { SchemaReport, Validation, ValidationReport } from '../lib/types'
import { useApp } from '../state/AppContext'

/** Schema tab › Create validation rules… (shown when Settings › General ›
 * "Suggest validation rules from the schema" is on). Builds a $jsonSchema
 * validator from the analyzed sample, checks it against the existing
 * documents, then saves it to the collection. */
export function ValidationFromSchema({ connId, db, coll, report, onClose }: { connId: string; db: string; coll: string; report: SchemaReport; onClose: () => void }) {
  const { toast, confirm } = useApp()
  const [requiredPercent, setRequiredPercent] = useState(100)
  const [rejectUnknown, setRejectUnknown] = useState(false)
  const [v, setV] = useState<Validation>({ validator: '', level: 'strict', action: 'error' })
  const [check, setCheck] = useState<ValidationReport | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    backend
      .suggestValidator(report, { requiredPercent, rejectUnknown })
      .then((validator) => {
        setV((cur) => ({ ...cur, validator }))
        setCheck(null)
      })
      .catch((err) => setError(toAppError(err)))
  }, [report, requiredPercent, rejectUnknown])

  const runCheck = async () => {
    setBusy(true)
    setError(null)
    try {
      setCheck(await backend.checkValidator(connId, db, coll, v.validator))
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  const save = async () => {
    if (check && check.failing > 0 && v.action === 'error') {
      const ok = await confirm({
        title: 'Some documents break these rules',
        message: `${formatCount(check.failing)} existing documents don't match. They stay as they are, but updating them will fail until they do. Save anyway?`,
        confirmLabel: 'Save rules',
        danger: true,
      })
      if (!ok) return
    }
    setBusy(true)
    setError(null)
    try {
      await backend.setValidation(connId, db, coll, v)
      toast({ kind: 'success', title: 'Validation rules saved', message: `${db}.${coll}` })
      onClose()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Create validation rules"
      onClose={onClose}
      wide
      footer={
        <>
          <button onClick={onClose}>Cancel</button>
          <button onClick={runCheck} disabled={busy || !v.validator.trim()}>
            {busy && <Spinner small />} Check existing documents
          </button>
          <button className="primary" onClick={save} disabled={busy || !check}>
            Save to {coll}
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <p className="small muted">
        Built from {formatCount(report.sampled)} sampled documents. Check them against the existing documents before saving; you can edit the rules
        below.
      </p>
      <div className="field-row">
        <label className="field">
          <span>Required when present in at least</span>
          <select value={requiredPercent} onChange={(e) => setRequiredPercent(Number(e.target.value))} aria-label="Required threshold">
            {[100, 99, 95, 90, 75, 50].map((n) => (
              <option key={n} value={n}>
                {n}% of documents
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>Level</span>
          <select value={v.level} onChange={(e) => setV({ ...v, level: e.target.value as Validation['level'] })} aria-label="Validation level">
            <option value="strict">Strict: every insert and update</option>
            <option value="moderate">Moderate: skip documents that already break the rules</option>
            <option value="off">Off: save the rules without enforcing them</option>
          </select>
        </label>
        <label className="field">
          <span>When a document breaks the rules</span>
          <select value={v.action} onChange={(e) => setV({ ...v, action: e.target.value as Validation['action'] })} aria-label="Validation action">
            <option value="error">Reject it</option>
            <option value="warn">Allow it, log a warning</option>
          </select>
        </label>
      </div>
      <label className="check">
        <input type="checkbox" checked={rejectUnknown} onChange={(e) => setRejectUnknown(e.target.checked)} />
        Reject fields that weren't seen in the sample
      </label>
      <label className="field">
        <span>Validator</span>
        <textarea
          className="mono"
          rows={14}
          value={v.validator}
          onChange={(e) => {
            setV({ ...v, validator: e.target.value })
            setCheck(null)
          }}
          spellCheck={false}
          aria-label="Validator"
        />
      </label>
      {check && (
        <p className={`small ${check.failing ? 'pc-fail' : ''}`} role="status">
          {check.failing === 0
            ? `All ${formatCount(check.checked)} existing documents match.`
            : `${formatCount(check.failing)} of ${formatCount(check.checked)} documents don't match${check.sample.length ? `, e.g. _id ${check.sample.slice(0, 3).join(', ')}` : ''}.`}
        </p>
      )}
    </Modal>
  )
}
