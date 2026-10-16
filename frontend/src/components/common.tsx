import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { errorTitle, type AppError } from '../lib/errors'
import { useApp } from '../state/AppContext'

export function Spinner({ small }: { small?: boolean }) {
  return <span className={small ? 'spinner small' : 'spinner'} role="status" aria-label="Loading" />
}

export function ErrorBanner({ error, onDismiss }: { error: AppError | null; onDismiss?: () => void }) {
  if (!error) return null
  return (
    <div className="error-banner" role="alert">
      <div>
        <strong>{errorTitle(error)}</strong>
        <div className="error-message">{error.message}</div>
        {error.detail && error.detail !== error.message && (
          <details>
            <summary>Details</summary>
            <pre>{error.detail}</pre>
          </details>
        )}
      </div>
      {onDismiss && (
        <button className="icon-btn" onClick={onDismiss} aria-label="Dismiss error">
          ×
        </button>
      )}
    </div>
  )
}

export function Modal({
  title,
  onClose,
  children,
  footer,
  wide,
}: {
  title: ReactNode
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  wide?: boolean
}) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || e.defaultPrevented) return
      // Only the topmost dialog closes (a confirmation over a form, …).
      const dialogs = document.querySelectorAll('[role="dialog"]')
      if (dialogs[dialogs.length - 1] !== ref.current) return
      e.preventDefault()
      onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
  return (
    <div className="modal-backdrop" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div ref={ref} className={wide ? 'modal wide' : 'modal'} role="dialog" aria-modal="true" aria-label={typeof title === 'string' ? title : undefined}>
        <header className="modal-header">
          <h2>{title}</h2>
          <button className="icon-btn" onClick={onClose} aria-label="Close">
            ×
          </button>
        </header>
        <div className="modal-body">{children}</div>
        {footer && <footer className="modal-footer">{footer}</footer>}
      </div>
    </div>
  )
}

export function ConfirmDialog() {
  const { confirmRequest: req, resolveConfirm } = useApp()
  const [typed, setTyped] = useState('')
  const confirmBtn = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    setTyped('')
    if (req && !req.typeToConfirm) confirmBtn.current?.focus()
  }, [req])
  if (!req) return null
  const blocked = !!req.typeToConfirm && typed !== req.typeToConfirm
  return (
    <Modal
      title={req.title}
      onClose={() => resolveConfirm(false)}
      footer={
        <>
          <button onClick={() => resolveConfirm(false)}>Cancel</button>
          <button
            ref={confirmBtn}
            className={req.danger ? 'danger' : 'primary'}
            disabled={blocked}
            onClick={() => resolveConfirm(true)}
          >
            {req.confirmLabel ?? 'OK'}
          </button>
        </>
      }
    >
      <div className="confirm-message">{req.message}</div>
      {req.typeToConfirm && (
        <label className="field">
          <span>
            Type <code>{req.typeToConfirm}</code> to confirm
          </span>
          <input autoFocus value={typed} onChange={(e) => setTyped(e.target.value)} spellCheck={false} />
        </label>
      )}
    </Modal>
  )
}

export function PromptDialog() {
  const { promptRequest: req, resolvePrompt } = useApp()
  const [value, setValue] = useState('')
  useEffect(() => setValue(req?.initial ?? ''), [req])
  if (!req) return null
  const submit = () => value.trim() && resolvePrompt(value.trim())
  return (
    <Modal
      title={req.title}
      onClose={() => resolvePrompt(null)}
      footer={
        <>
          <button onClick={() => resolvePrompt(null)}>Cancel</button>
          <button className="primary" disabled={!value.trim()} onClick={submit}>
            {req.confirmLabel ?? 'OK'}
          </button>
        </>
      }
    >
      <label className="field">
        <span>{req.label}</span>
        <input autoFocus value={value} onChange={(e) => setValue(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && submit()} />
      </label>
    </Modal>
  )
}

export function Toasts() {
  const { toasts, dismissToast } = useApp()
  return (
    <div className="toasts" aria-live="polite">
      {toasts.map((t) => (
        <div key={t.id} className={`toast ${t.kind}`} onClick={() => dismissToast(t.id)}>
          <strong>{t.title}</strong>
          {t.message && <div>{t.message}</div>}
        </div>
      ))}
    </div>
  )
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
  label,
}: {
  value: T
  options: { value: T; label: string; title?: string }[]
  onChange: (v: T) => void
  label: string
}) {
  return (
    <div className="segmented" role="radiogroup" aria-label={label}>
      {options.map((o) => (
        <button
          key={o.value}
          role="radio"
          aria-checked={o.value === value}
          className={o.value === value ? 'active' : ''}
          title={o.title}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}
