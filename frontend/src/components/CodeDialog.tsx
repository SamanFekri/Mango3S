import { useEffect, useState } from 'react'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { CodeLanguage, CodeRequest } from '../lib/types'
import { CodeBlock } from '../lib/syntax'
import { ErrorBanner, Modal, Spinner } from './common'

const LAST_LANG = 'mango3s.codeLanguage'

/** Shows the query/pipeline/SQL as driver code in a chosen language. */
export function CodeDialog({ request, onClose }: { request: Omit<CodeRequest, 'language'>; onClose: () => void }) {
  const [languages, setLanguages] = useState<CodeLanguage[]>([])
  const [lang, setLang] = useState(() => {
    try {
      return localStorage.getItem(LAST_LANG) ?? 'javascript'
    } catch {
      return 'javascript'
    }
  })
  const [code, setCode] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    backend.codeLanguages().then(setLanguages).catch((err) => setError(toAppError(err)))
  }, [])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    backend
      .generateCode({ ...request, language: lang })
      .then((c) => !cancelled && setCode(c))
      .catch((err) => !cancelled && (setCode(''), setError(toAppError(err))))
      .finally(() => !cancelled && setLoading(false))
    try {
      localStorage.setItem(LAST_LANG, lang)
    } catch {
      // storage unavailable
    }
    return () => {
      cancelled = true
    }
  }, [lang, request])

  const copy = async () => {
    await navigator.clipboard?.writeText(code)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1500)
  }

  return (
    <Modal
      title="Generate code"
      onClose={onClose}
      wide
      footer={
        <>
          <span className="muted small">Reads the connection string from the MONGODB_URI environment variable.</span>
          <span className="spacer" />
          <button onClick={onClose}>Close</button>
          <button className="primary" onClick={copy} disabled={!code}>
            {copied ? 'Copied' : 'Copy'}
          </button>
        </>
      }
    >
      <div className="lang-tabs" role="tablist" aria-label="Language">
        {languages.map((l) => (
          <button key={l.id} role="tab" aria-selected={l.id === lang} className={l.id === lang ? 'active small' : 'small'} onClick={() => setLang(l.id)}>
            {l.label}
          </button>
        ))}
        {loading && <Spinner small />}
      </div>
      <ErrorBanner error={error} />
      <CodeBlock code={code} lang={lang} label="Generated code" />
    </Modal>
  )
}
