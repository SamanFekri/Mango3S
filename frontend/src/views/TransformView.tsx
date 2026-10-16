import { useEffect, useState } from 'react'
import { ErrorBanner, Segmented, Spinner } from '../components/common'
import { highlight } from '../components/highlight'
import { Checks, ConfirmInput, JobStatus, Stat, useJobDone } from '../components/p2common'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatCount } from '../lib/format'
import type { Tab } from '../lib/tabs'
import type { MaskMethod, MaskRule, MaskRuleSet, ReschemaOp, TransformPreview, TransformReport, TransformSpec } from '../lib/types'
import { useApp } from '../state/AppContext'

const OP_TYPES: { id: ReschemaOp['type']; label: string }[] = [
  { id: 'rename', label: 'Rename / move' },
  { id: 'remove', label: 'Remove' },
  { id: 'set', label: 'Set value' },
  { id: 'copy', label: 'Copy' },
  { id: 'convert', label: 'Convert type' },
  { id: 'flatten', label: 'Flatten sub-document' },
  { id: 'nest', label: 'Nest fields' },
]

const newOp = (): ReschemaOp => ({ type: 'rename', field: '', to: '', value: '', onlyIfMissing: false, targetType: 'string', separator: '_', fields: [] })
const newRule = (): MaskRule => ({ field: '', method: 'hash', keepLast: 4, maskChar: '*', value: '', maxDays: 30, percent: 10, onUnsupported: '' })

/** Reschema and masking share one workflow: edit, preview (dry run), apply, report. */
export function TransformView({ tab }: { tab: Tab }) {
  const kind = tab.kind === 'mask' ? 'mask' : 'reschema'
  const { toast, prompt, toastError } = useApp()
  const [spec, setSpec] = useState<TransformSpec>({
    connId: tab.connId!,
    db: tab.db!,
    coll: tab.coll!,
    filter: '',
    mode: 'copy',
    targetColl: `${tab.coll}_${kind === 'mask' ? 'masked' : 'v2'}`,
    onError: 'skip',
  })
  const [ops, setOps] = useState<ReschemaOp[]>([newOp()])
  const [rules, setRules] = useState<MaskRule[]>([newRule()])
  const [types, setTypes] = useState<string[]>([])
  const [methods, setMethods] = useState<MaskMethod[]>([])
  const [ruleSets, setRuleSets] = useState<MaskRuleSet[]>([])
  const [fields, setFields] = useState<string[]>([])
  const [preview, setPreview] = useState<TransformPreview | null>(null)
  const [confirm, setConfirm] = useState('')
  const [jobId, setJobId] = useState<string | null>(null)
  const [report, setReport] = useState<TransformReport | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    backend.sampleFields(spec.connId, spec.db, spec.coll).then(setFields).catch(() => undefined)
    if (kind === 'reschema') backend.reschemaTypes().then(setTypes).catch(() => undefined)
    else {
      backend.maskMethods().then(setMethods).catch(() => undefined)
      backend.listMaskRuleSets().then(setRuleSets).catch(() => undefined)
    }
  }, [kind, spec.connId, spec.db, spec.coll])

  useJobDone(jobId, async (j) => {
    try {
      setReport(await backend.getTransformReport(j.id))
    } catch {
      // the job row shows why it failed
    }
  })

  const runPreview = async () => {
    setBusy(true)
    setError(null)
    setReport(null)
    try {
      const pv = kind === 'reschema' ? await backend.previewReschema(spec, ops) : await backend.previewMask(spec, rules)
      setPreview(pv)
      setConfirm('')
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  const apply = async () => {
    if (!preview) return
    try {
      const job = await backend.startTransform(preview.planId, confirm)
      setJobId(job.id)
      setPreview(null)
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const saveRuleSet = async () => {
    const name = await prompt({ title: 'Save masking rules', label: 'Rule set name', confirmLabel: 'Save' })
    if (!name) return
    try {
      await backend.saveMaskRuleSet({ name, rules })
      setRuleSets(await backend.listMaskRuleSets())
      toast({ kind: 'success', title: `Saved "${name}"`, message: 'Use it in scheduled masked copies and the MCP server.' })
    } catch (err) {
      toastError(err, 'Save failed')
    }
  }

  const setOp = (i: number, patch: Partial<ReschemaOp>) => setOps(ops.map((o, j) => (j === i ? { ...o, ...patch } : o)))
  const setRule = (i: number, patch: Partial<MaskRule>) => setRules(rules.map((r, j) => (j === i ? { ...r, ...patch } : r)))
  const listId = `fields-${tab.id}`

  return (
    <div className="view transform-view">
      <div className="view-header">
        <h1>{kind === 'mask' ? 'Mask data' : 'Reschema'}</h1>
        <span className="muted">
          {spec.db}.{spec.coll}
        </span>
      </div>
      <datalist id={listId}>
        {fields.map((f) => (
          <option key={f} value={f} />
        ))}
      </datalist>

      {kind === 'reschema' ? (
        <section className="panel">
          <h3>Operations (applied in order)</h3>
          {ops.map((op, i) => (
            <div className="op-row" key={i}>
              <span className="stage-num">{i + 1}</span>
              <select value={op.type} onChange={(e) => setOp(i, { type: e.target.value as ReschemaOp['type'] })} aria-label="Operation">
                {OP_TYPES.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.label}
                  </option>
                ))}
              </select>
              {op.type === 'nest' ? (
                <input placeholder="fields: a, b, c" value={op.fields.join(', ')} onChange={(e) => setOp(i, { fields: e.target.value.split(',').map((s) => s.trim()).filter(Boolean) })} />
              ) : (
                <input list={listId} placeholder="field" value={op.field} onChange={(e) => setOp(i, { field: e.target.value })} spellCheck={false} />
              )}
              {(op.type === 'rename' || op.type === 'copy' || op.type === 'nest') && (
                <input placeholder={op.type === 'nest' ? 'into sub-document' : 'new path, e.g. address.city'} value={op.to} onChange={(e) => setOp(i, { to: e.target.value })} spellCheck={false} />
              )}
              {op.type === 'set' && (
                <>
                  <input className="mono" placeholder='value, e.g. "active" or ISODate("2024-01-01")' value={op.value} onChange={(e) => setOp(i, { value: e.target.value })} />
                  <label className="check small">
                    <input type="checkbox" checked={op.onlyIfMissing} onChange={(e) => setOp(i, { onlyIfMissing: e.target.checked })} />
                    only if missing
                  </label>
                </>
              )}
              {op.type === 'convert' && (
                <select value={op.targetType} onChange={(e) => setOp(i, { targetType: e.target.value })} aria-label="Target type">
                  {types.map((t) => (
                    <option key={t}>{t}</option>
                  ))}
                </select>
              )}
              {op.type === 'flatten' && <input className="narrow-input" placeholder="separator" value={op.separator} onChange={(e) => setOp(i, { separator: e.target.value })} />}
              <button className="icon-btn" aria-label="Remove operation" onClick={() => setOps(ops.filter((_, j) => j !== i))}>
                ×
              </button>
            </div>
          ))}
          <button className="small ghost" onClick={() => setOps([...ops, newOp()])}>
            + Operation
          </button>
        </section>
      ) : (
        <section className="panel">
          <div className="qb-head">
            <h3>Masking rules</h3>
            <span className="spacer" />
            {ruleSets.length > 0 && (
              <select
                defaultValue=""
                onChange={(e) => {
                  const rs = ruleSets.find((r) => r.id === e.target.value)
                  if (rs) setRules(rs.rules.map((r) => ({ ...newRule(), ...r })))
                }}
                aria-label="Load rule set"
              >
                <option value="">Load rule set…</option>
                {ruleSets.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </select>
            )}
            <button className="small ghost" onClick={saveRuleSet}>
              Save as rule set
            </button>
          </div>
          {rules.map((r, i) => {
            const m = methods.find((x) => x.id === r.method)
            return (
              <div className="op-row" key={i}>
                <input list={listId} placeholder="field (e.g. email, contacts.phone)" value={r.field} onChange={(e) => setRule(i, { field: e.target.value })} spellCheck={false} />
                <select value={r.method} onChange={(e) => setRule(i, { method: e.target.value })} aria-label="Method" title={m?.description}>
                  {methods.map((x) => (
                    <option key={x.id} value={x.id}>
                      {x.label}
                    </option>
                  ))}
                </select>
                {r.method === 'partial' && (
                  <label className="inline">
                    keep last
                    <input type="number" min={0} max={8} value={r.keepLast} onChange={(e) => setRule(i, { keepLast: Number(e.target.value) })} />
                  </label>
                )}
                {r.method === 'constant' && <input className="mono" placeholder='"n/a"' value={r.value} onChange={(e) => setRule(i, { value: e.target.value })} />}
                {r.method === 'dateShift' && (
                  <label className="inline">
                    ± days
                    <input type="number" min={1} value={r.maxDays} onChange={(e) => setRule(i, { maxDays: Number(e.target.value) })} />
                  </label>
                )}
                {r.method === 'noise' && (
                  <label className="inline">
                    ± %
                    <input type="number" min={1} max={100} value={r.percent} onChange={(e) => setRule(i, { percent: Number(e.target.value) })} />
                  </label>
                )}
                <select value={r.onUnsupported} onChange={(e) => setRule(i, { onUnsupported: e.target.value as MaskRule['onUnsupported'] })} aria-label="Unsupported types" title="What to do with values of a type this method cannot mask">
                  <option value="">unsupported type: fail</option>
                  <option value="null">unsupported type: null it</option>
                  <option value="skip">unsupported type: leave as is</option>
                </select>
                <span className="muted small">{m ? m.types.join(', ') : ''}</span>
                <button className="icon-btn" aria-label="Remove rule" onClick={() => setRules(rules.filter((_, j) => j !== i))}>
                  ×
                </button>
              </div>
            )
          })}
          <button className="small ghost" onClick={() => setRules([...rules, newRule()])}>
            + Rule
          </button>
        </section>
      )}

      <section className="panel">
        <div className="field-row">
          <label className="field">
            <span>Only documents matching</span>
            <input className="mono" value={spec.filter} onChange={(e) => setSpec({ ...spec, filter: e.target.value })} placeholder="{}" />
          </label>
          <div className="field">
            <span>Write to</span>
            <Segmented
              label="Mode"
              value={spec.mode}
              onChange={(mode) => setSpec({ ...spec, mode })}
              options={[
                { value: 'copy', label: 'New collection' },
                { value: 'inPlace', label: 'In place (backed up)' },
              ]}
            />
          </div>
          {spec.mode === 'copy' && (
            <label className="field">
              <span>Target collection</span>
              <input value={spec.targetColl} onChange={(e) => setSpec({ ...spec, targetColl: e.target.value })} spellCheck={false} />
            </label>
          )}
          <label className="field narrow">
            <span>On error</span>
            <select value={spec.onError} onChange={(e) => setSpec({ ...spec, onError: e.target.value as TransformSpec['onError'] })}>
              <option value="skip">skip document</option>
              <option value="fail">stop</option>
            </select>
          </label>
        </div>
        <div className="form-actions">
          <button className="primary" onClick={runPreview} disabled={busy}>
            {busy && <Spinner small />} Preview (dry run)
          </button>
          <span className="muted small">The preview runs the exact transformation the apply will run, without writing.</span>
        </div>
      </section>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />

      {preview && (
        <section className="panel">
          <div className="metrics">
            <Stat label="Matching documents" value={formatCount(preview.preview.total)} />
            <Stat label={preview.preview.estimated ? 'Changed (first 100k)' : 'Would change'} value={formatCount(preview.preview.changed)} />
            <Stat label="Errors" value={formatCount(preview.preview.errors)} tone={preview.preview.errors ? 'warn' : 'good'} />
            <Stat label="Target" value={preview.target} />
          </div>
          {preview.preview.errorSamples.length > 0 && (
            <details>
              <summary>{preview.preview.errorSamples.length} error example(s)</summary>
              <ul className="small">
                {preview.preview.errorSamples.map((e) => (
                  <li key={e} className="mono">
                    {e}
                  </li>
                ))}
              </ul>
            </details>
          )}
          <div className="before-after">
            {preview.preview.samples.slice(0, 4).map((s) => (
              <div className="ba-pair" key={s.id}>
                <div>
                  <div className="muted small">Before {s.id}</div>
                  <pre className="doc-text">{highlight(s.before.text)}</pre>
                </div>
                <div>
                  <div className="muted small">{s.error ? `Error: ${s.error}` : s.changed ? 'After' : 'After (unchanged)'}</div>
                  <pre className="doc-text">{highlight(s.after.text)}</pre>
                </div>
              </div>
            ))}
          </div>
          <ConfirmInput text={preview.confirmText} value={confirm} onChange={setConfirm} action="Modifies the collection in place" />
          <div className="form-actions">
            <button onClick={() => setPreview(null)}>Discard</button>
            <button className="primary" disabled={!!preview.confirmText && confirm !== preview.confirmText} onClick={apply}>
              Apply
            </button>
          </div>
        </section>
      )}

      <JobStatus jobId={jobId} />
      {report && (
        <section className="panel">
          <h3>Result {report.valid ? '— validated' : '— validation failed'}</h3>
          <div className="metrics">
            <Stat label="Scanned" value={formatCount(report.scanned)} />
            <Stat label="Changed" value={formatCount(report.changed)} />
            <Stat label="Written" value={formatCount(report.written)} />
            <Stat label="Skipped" value={formatCount(report.skipped)} tone={report.skipped ? 'warn' : undefined} />
          </div>
          <Checks checks={report.validation} />
          {report.backupPath && (
            <p className="muted small">
              Backup for undo: <code>{report.backupPath}</code> (restore it from Library → Backups)
            </p>
          )}
        </section>
      )}
    </div>
  )
}
