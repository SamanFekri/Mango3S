import { useEffect, useState } from 'react'
import { ErrorBanner, Modal, Spinner } from './common'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatCount } from '../lib/format'
import type { SessionInfo, TestDataPreview } from '../lib/types'
import { useApp } from '../state/AppContext'

const randomSeed = () => Math.floor(Math.random() * 1_000_000)

/** Collection › Generate test data…: a template with generator markers
 * (pre-filled from the collection's schema), a preview of 5 documents, then a
 * background job that inserts them in batches. */
export function TestDataDialog({ conn, db, coll, onClose }: { conn: SessionInfo; db: string; coll: string; onClose: () => void }) {
  const { confirm, toast } = useApp()
  const [template, setTemplate] = useState<string | null>(null)
  const [seed, setSeed] = useState(randomSeed)
  const [count, setCount] = useState(1000)
  const [preview, setPreview] = useState<TestDataPreview | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  const [busy, setBusy] = useState(false)
  const [help, setHelp] = useState(false)

  useEffect(() => {
    backend
      .testDataTemplateFromSchema(conn.id, db, coll)
      .then(setTemplate)
      .catch((err) => {
        setTemplate('{\n  name: $name,\n  email: $email\n}')
        setError(toAppError(err))
      })
  }, [conn.id, db, coll])

  useEffect(() => {
    if (template === null) return
    const id = window.setTimeout(() => {
      backend
        .previewTestData({ connId: conn.id, db, coll, template, seed, count })
        .then((p) => {
          setPreview(p)
          setError(null)
        })
        .catch((err) => setError(toAppError(err)))
    }, 300)
    return () => window.clearTimeout(id)
  }, [template, seed, conn.id, db, coll, count])

  const generate = async () => {
    if (!template) return
    if (preview && preview.existing > 0) {
      const ok = await confirm({
        title: 'The collection isn\'t empty',
        message: `${db}.${coll} already has about ${formatCount(preview.existing)} documents. Add ${formatCount(count)} generated documents to them?`,
        confirmLabel: 'Add documents',
        danger: true,
      })
      if (!ok) return
    }
    setBusy(true)
    try {
      await backend.startTestData({ connId: conn.id, db, coll, template, seed, count })
      toast({ kind: 'info', title: `Generating ${formatCount(count)} documents`, message: 'Progress is in the Jobs panel.' })
      onClose()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={`Generate test data: ${db}.${coll}`}
      onClose={onClose}
      wide
      footer={
        <>
          <button onClick={onClose}>Cancel</button>
          <button className="primary" onClick={() => void generate()} disabled={busy || !preview || conn.readOnly}>
            {busy && <Spinner small />} Generate {formatCount(count)} documents
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {template === null ? (
        <Spinner />
      ) : (
        <div className="testdata">
          <div className="testdata-edit">
            <label className="field">
              <span>
                Template{' '}
                <button className="link small" onClick={() => setHelp(!help)}>
                  {help ? 'Hide generators' : 'Generators…'}
                </button>
              </span>
              <textarea className="mono" rows={14} value={template} onChange={(e) => setTemplate(e.target.value)} spellCheck={false} aria-label="Template" />
            </label>
            {help && preview && (
              <table className="grid compact small" aria-label="Generators">
                <tbody>
                  {preview.generators.map((g) => (
                    <tr key={g.name}>
                      <td className="mono">{g.example}</td>
                      <td>{g.help}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            <div className="field-row">
              <label className="field narrow">
                <span>Documents</span>
                <input type="number" min={1} max={10000000} value={count} onChange={(e) => setCount(Math.max(1, Number(e.target.value)))} aria-label="Number of documents" />
              </label>
              <label className="field narrow">
                <span>Seed</span>
                <input type="number" value={seed} onChange={(e) => setSeed(Number(e.target.value))} aria-label="Seed" />
              </label>
              <button className="small" onClick={() => setSeed(randomSeed())}>
                New seed
              </button>
            </div>
            <p className="muted small">The same template and seed give the same documents (dates are relative to when you generate).</p>
          </div>
          <div className="testdata-preview">
            <div className="small muted">Preview</div>
            {preview?.documents.map((d, i) => (
              <pre key={i} className="doc-text small">
                {d.text}
              </pre>
            ))}
          </div>
        </div>
      )}
    </Modal>
  )
}
