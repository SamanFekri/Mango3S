import { useState } from 'react'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { CodeEditor } from './CodeEditor'
import { ErrorBanner, Modal, Spinner } from './common'

export interface DocumentEditorProps {
  mode: 'insert' | 'edit'
  title: string
  initial: string
  fields?: string[]
  onClose: () => void
  /** Persists the text; throw to keep the dialog open with the error shown. */
  onSave: (text: string) => Promise<void>
}

export const INSERT_TEMPLATE = `{
  // Shell syntax: unquoted keys, ObjectId(), ISODate(), NumberLong() ...
  // Insert several documents at once with an array: [ {...}, {...} ]

}`

export function DocumentEditor({ mode, title, initial, fields, onClose, onSave }: DocumentEditorProps) {
  const [text, setText] = useState(initial)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  const format = async () => {
    try {
      setText(await backend.formatDocument(text))
      setError(null)
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const save = async () => {
    setBusy(true)
    setError(null)
    try {
      await onSave(text)
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  const close = () => {
    if (text !== initial && !window.confirm('Discard unsaved changes?')) return
    onClose()
  }

  return (
    <Modal
      title={title}
      onClose={close}
      wide
      footer={
        <>
          <span className="muted small">{mode === 'edit' ? 'Replaces the whole document. _id cannot change.' : 'Cmd/Ctrl+Enter to save'}</span>
          <span className="spacer" />
          <button onClick={format}>Format</button>
          <button onClick={close}>Cancel</button>
          <button className="primary" onClick={save} disabled={busy}>
            {busy && <Spinner small />} {mode === 'insert' ? 'Insert' : 'Save'}
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <CodeEditor value={text} onChange={setText} onRun={save} fields={fields} minHeight={360} maxHeight={560} ariaLabel="Document" autoFocus />
    </Modal>
  )
}
