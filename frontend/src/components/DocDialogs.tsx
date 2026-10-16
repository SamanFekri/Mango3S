// Right-click menu and small dialogs for document and field commands.
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { toAppError } from '../lib/errors'
import { shortcutHint } from '../lib/shortcuts'
import { ErrorBanner, Modal, Spinner } from './common'
import { highlight } from './highlight'

export interface MenuEntry {
  /** Shortcut id: the menu shows its binding and runs the same command. */
  id: string
  label: string
  run: () => void
  enabled: boolean
  danger?: boolean
}

/** A context menu; null entries are separators. */
export function ContextMenu({ x, y, items, onClose }: { x: number; y: number; items: (MenuEntry | null)[]; onClose: () => void }) {
  const ref = useRef<HTMLUListElement>(null)
  const [pos, setPos] = useState({ x, y })
  // Keep the menu on screen.
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const r = el.getBoundingClientRect()
    setPos({ x: Math.max(4, Math.min(x, window.innerWidth - r.width - 4)), y: Math.max(4, Math.min(y, window.innerHeight - r.height - 4)) })
  }, [x, y])
  useEffect(() => {
    ref.current?.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus()
    const close = (e: Event) => {
      if (e instanceof KeyboardEvent && e.key !== 'Escape') return
      if (e instanceof MouseEvent && ref.current?.contains(e.target as Node)) return
      e.preventDefault?.()
      onClose()
    }
    window.addEventListener('mousedown', close, true)
    window.addEventListener('keydown', close, true)
    window.addEventListener('blur', onClose)
    return () => {
      window.removeEventListener('mousedown', close, true)
      window.removeEventListener('keydown', close, true)
      window.removeEventListener('blur', onClose)
    }
  }, [onClose])
  const move = (e: React.KeyboardEvent) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    e.preventDefault()
    const btns = Array.from(ref.current?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') ?? [])
    const i = btns.indexOf(document.activeElement as HTMLButtonElement)
    btns[(i + (e.key === 'ArrowDown' ? 1 : -1) + btns.length) % btns.length]?.focus()
  }
  return (
    <ul ref={ref} className="context-menu" role="menu" style={{ left: pos.x, top: pos.y }} onKeyDown={move}>
      {items.map((it, i) =>
        it ? (
          <li key={it.id}>
            <button
              role="menuitem"
              className={it.danger ? 'danger-text' : ''}
              disabled={!it.enabled}
              onClick={() => {
                onClose()
                it.run()
              }}
            >
              <span>{it.label}</span>
              {shortcutHint(it.id) && <kbd className="menu-kbd">{shortcutHint(it.id)}</kbd>}
            </button>
          </li>
        ) : (
          <li key={`sep${i}`} className="menu-sep" role="separator" />
        ),
      )}
    </ul>
  )
}

export const VALUE_TYPES = ['string', 'int32', 'int64', 'double', 'decimal', 'boolean', 'date', 'objectId', 'null', 'document', 'array'] as const

const PLACEHOLDER: Record<string, string> = {
  string: 'text (no quotes)',
  int32: '42',
  int64: '9007199254740993',
  double: '3.14',
  decimal: '19.99',
  boolean: 'true or false',
  date: '2024-05-01T12:00:00Z',
  objectId: '24 hex characters',
  null: '',
  document: '{ field: "value" }',
  array: '[1, 2, 3]',
}

/** Add field / value (Ctrl+K): a named field in a document, or a value appended to an array. */
export function AddFieldDialog({
  target,
  isArray,
  onSave,
  onClose,
}: {
  target: string
  isArray: boolean
  onSave: (name: string, type: string, text: string) => Promise<void>
  onClose: () => void
}) {
  const [name, setName] = useState('')
  const [type, setType] = useState<string>('string')
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ReturnType<typeof toAppError> | null>(null)
  const save = async () => {
    setBusy(true)
    setError(null)
    try {
      await onSave(name.trim(), type, text)
      onClose()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  const multi = type === 'document' || type === 'array'
  return (
    <Modal
      title={isArray ? `Add value to ${target}` : `Add field to ${target || 'the document'}`}
      onClose={onClose}
      footer={
        <>
          <button onClick={onClose}>Cancel</button>
          <button className="primary" onClick={save} disabled={busy || (!isArray && !name.trim())}>
            {busy && <Spinner small />} Add
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {!isArray && (
        <label className="field">
          <span>Field name</span>
          <input value={name} onChange={(e) => setName(e.target.value)} autoFocus aria-label="Field name" spellCheck={false} />
        </label>
      )}
      <div className="field-row">
        <label className="field narrow">
          <span>Type</span>
          <select value={type} onChange={(e) => setType(e.target.value)} aria-label="Value type">
            {VALUE_TYPES.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </label>
        {type !== 'null' && (
          <label className="field">
            <span>Value</span>
            {multi ? (
              <textarea className="mono" rows={4} value={text} onChange={(e) => setText(e.target.value)} placeholder={PLACEHOLDER[type]} aria-label="Value" autoFocus={isArray} />
            ) : (
              <input
                className="mono"
                value={text}
                onChange={(e) => setText(e.target.value)}
                placeholder={PLACEHOLDER[type]}
                aria-label="Value"
                autoFocus={isArray}
                onKeyDown={(e) => e.key === 'Enter' && void save()}
              />
            )}
          </label>
        )}
      </div>
      <p className="muted small">The value is stored with exactly this type; invalid input is rejected before anything is written.</p>
    </Modal>
  )
}

/** Rename field (Shift+F2). */
export function RenameFieldDialog({ path, onSave, onClose }: { path: string[]; onSave: (name: string) => Promise<void>; onClose: () => void }) {
  const [name, setName] = useState(path[path.length - 1])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ReturnType<typeof toAppError> | null>(null)
  const save = async () => {
    setBusy(true)
    setError(null)
    try {
      await onSave(name.trim())
      onClose()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal
      title={`Rename ${path.join('.')}`}
      onClose={onClose}
      footer={
        <>
          <button onClick={onClose}>Cancel</button>
          <button className="primary" onClick={save} disabled={busy || !name.trim() || name.trim() === path[path.length - 1]}>
            {busy && <Spinner small />} Rename
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <label className="field">
        <span>New name</span>
        <input value={name} onChange={(e) => setName(e.target.value)} autoFocus aria-label="New field name" spellCheck={false} onKeyDown={(e) => e.key === 'Enter' && void save()} />
      </label>
      <p className="muted small">Uses $rename on this document only; the value and its type are kept.</p>
    </Modal>
  )
}

/** View document (F3): read-only. */
export function DocumentViewer({ title, text, onClose, onEdit }: { title: string; text: string; onClose: () => void; onEdit?: () => void }) {
  return (
    <Modal
      title={title}
      onClose={onClose}
      wide
      footer={
        <>
          <button onClick={() => navigator.clipboard?.writeText(text)}>Copy</button>
          <span className="spacer" />
          {onEdit && (
            <button
              onClick={() => {
                onClose()
                onEdit()
              }}
              title={`Edit document (JSON) (${shortcutHint('doc.editJson')})`}
            >
              Edit…
            </button>
          )}
          <button className="primary" onClick={onClose}>
            Close
          </button>
        </>
      }
    >
      <pre className="doc-text viewer" aria-label="Document">
        {highlight(text)}
      </pre>
    </Modal>
  )
}
