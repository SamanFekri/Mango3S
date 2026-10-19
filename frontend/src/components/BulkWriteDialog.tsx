import { useEffect, useState } from 'react'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { BulkPreview, BulkResult, BulkWriteRequest } from '../lib/types'
import { CodeEditor } from './CodeEditor'
import { ErrorBanner, Modal, Spinner } from './common'
import { ConfirmInput } from './p2common'

/**
 * The Update dialog (Ctrl+U, updateMany) and Remove dialog (F8,
 * deleteMany). Preview first; running needs the target typed and stops if
 * the number of matching documents changed. Everything touched is backed up.
 */
export function BulkWriteDialog({
  connId,
  db,
  coll,
  kind,
  selectedIds,
  queryFilter,
  onDone,
  onClose,
}: {
  connId: string
  db: string
  coll: string
  kind: 'update' | 'remove'
  selectedIds: string[]
  queryFilter: string
  onDone: (r: BulkResult) => void
  onClose: () => void
}) {
  const [scope, setScope] = useState<'selected' | 'query'>(selectedIds.length ? 'selected' : 'query')
  const [filter, setFilter] = useState(queryFilter || '{}')
  const [update, setUpdate] = useState('{ $set: { field: "value" } }')
  const [preview, setPreview] = useState<BulkPreview | null>(null)
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState<'' | 'preview' | 'run'>('')
  const [error, setError] = useState<AppError | null>(null)
  const req: BulkWriteRequest = { connId, db, coll, kind: kind === 'remove' ? 'delete' : 'update', scope, ids: selectedIds, filter, update }

  // Any change invalidates the preview.
  useEffect(() => {
    setPreview(null)
    setConfirm('')
  }, [scope, filter, update])

  const runPreview = async () => {
    setBusy('preview')
    setError(null)
    try {
      setPreview(await backend.previewBulkWrite(req))
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy('')
    }
  }
  const run = async () => {
    if (!preview) return
    setBusy('run')
    setError(null)
    try {
      onDone(await backend.executeBulkWrite(req, confirm, preview.affected))
      onClose()
    } catch (err) {
      setError(toAppError(err))
      setPreview(null) // e.g. the count changed: preview again
    } finally {
      setBusy('')
    }
  }
  const verb = kind === 'remove' ? 'Remove' : 'Update'
  return (
    <Modal
      title={`${verb} documents in ${db}.${coll}`}
      onClose={onClose}
      wide
      footer={
        <>
          <button onClick={onClose}>Cancel</button>
          <span className="spacer" />
          <button onClick={() => void runPreview()} disabled={!!busy}>
            {busy === 'preview' && <Spinner small />} Preview
          </button>
          <button className="danger" onClick={() => void run()} disabled={!!busy || !preview || preview.affected === 0 || confirm !== preview.confirmText}>
            {busy === 'run' && <Spinner small />} {verb} {preview ? preview.affected : ''} document{preview?.affected === 1 ? '' : 's'}
          </button>
        </>
      }
    >
      <div className="segmented" role="radiogroup" aria-label="Which documents">
        <button role="radio" aria-checked={scope === 'selected'} className={scope === 'selected' ? 'active' : ''} disabled={!selectedIds.length} onClick={() => setScope('selected')}>
          {selectedIds.length ? `The ${selectedIds.length} selected` : 'Selected (none)'}
        </button>
        <button role="radio" aria-checked={scope === 'query'} className={scope === 'query' ? 'active' : ''} onClick={() => setScope('query')}>
          All matching a query
        </button>
      </div>
      {scope === 'query' && (
        <label className="field">
          <span>Filter</span>
          <CodeEditor value={filter} onChange={setFilter} compact ariaLabel="Filter" maxHeight={120} />
        </label>
      )}
      {kind === 'update' && (
        <label className="field">
          <span>Update (operators such as $set, $unset, $inc — or an aggregation pipeline)</span>
          <CodeEditor value={update} onChange={setUpdate} compact ariaLabel="Update" maxHeight={180} />
        </label>
      )}
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {preview && (
        <section className="panel">
          <p>
            <strong>{preview.affected}</strong> document{preview.affected === 1 ? '' : 's'} will be {kind === 'remove' ? 'deleted' : 'updated'}
            {preview.sampleIds.length > 0 && <> (e.g. {preview.sampleIds.join(', ')})</>}. Their current versions are backed up first (Library → Backups).
          </p>
          <pre className="doc-text small">{preview.shell}</pre>
          {preview.affected > 0 && <ConfirmInput text={preview.confirmText} value={confirm} onChange={setConfirm} action={`${verb}s ${preview.affected} documents`} />}
        </section>
      )}
    </Modal>
  )
}
