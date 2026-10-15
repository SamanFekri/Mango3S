import { useEffect, useRef, useState } from 'react'
import type { EditorView } from '@codemirror/view'
import { CodeEditor, selectionOrAll } from '../components/CodeEditor'
import { Spinner } from '../components/common'
import { highlight } from '../components/highlight'
import { backend } from '../lib/backend'
import { toAppError } from '../lib/errors'
import { formatDuration } from '../lib/format'
import type { Tab } from '../lib/tabs'
import type { ShellOutput } from '../lib/types'
import { useApp } from '../state/AppContext'

interface Entry {
  id: number
  db: string
  outputs: ShellOutput[]
}

const STARTER = `// Cmd/Ctrl+Enter runs everything, or just the selected text.
// Type help for the list of supported commands.
show collections
`

export function ShellView({ tab }: { tab: Tab }) {
  const { sessions } = useApp()
  const connId = tab.connId!
  const session = sessions.find((s) => s.id === connId)
  const [db, setDb] = useState(tab.db || 'test')
  const [script, setScript] = useState(STARTER)
  const [history, setHistory] = useState<Entry[]>([])
  const [running, setRunning] = useState(false)
  const editor = useRef<EditorView | null>(null)
  const out = useRef<HTMLDivElement>(null)
  const seq = useRef(0)
  const opId = `shell-${tab.id}`

  useEffect(
    () => () => {
      backend.cancel(opId).catch(() => undefined)
    },
    [opId],
  )

  useEffect(() => {
    out.current?.scrollTo({ top: out.current.scrollHeight, behavior: 'smooth' })
  }, [history])

  const run = async () => {
    const code = selectionOrAll(editor.current).trim()
    if (!code || running) return
    setRunning(true)
    seq.current += 1
    const id = seq.current
    try {
      const res = await backend.runShell(opId, connId, db, code)
      setHistory((h) => [...h.slice(-49), { id, db, outputs: res.outputs }])
      setDb(res.currentDb)
    } catch (err) {
      const e = toAppError(err)
      setHistory((h) => [
        ...h.slice(-49),
        { id, db, outputs: [{ statement: code.split('\n')[0], kind: 'error', text: e.message, hasMore: false, durationMs: 0 }] },
      ])
    } finally {
      setRunning(false)
    }
  }

  return (
    <div className="view shell-view">
      <div className="view-header">
        <div className="breadcrumbs">
          <span>{session?.name ?? tab.connName}</span>
          <span className="sep">/</span>
          <label className="inline">
            db
            <input className="db-input mono" value={db} onChange={(e) => setDb(e.target.value)} aria-label="Current database" spellCheck={false} />
          </label>
        </div>
        <span className="spacer" />
        <button className="small ghost" onClick={() => setHistory([])} disabled={history.length === 0}>
          Clear output
        </button>
        {running ? (
          <button className="small" onClick={() => backend.cancel(opId)}>
            <Spinner small /> Cancel
          </button>
        ) : (
          <button className="primary" onClick={run} title="Run (Cmd/Ctrl+Enter)">
            ▶ Run
          </button>
        )}
      </div>
      <div className="shell-split">
        <CodeEditor value={script} onChange={setScript} onRun={run} editorRef={editor} minHeight={140} maxHeight={320} ariaLabel="Shell script" autoFocus />
        <div className="shell-output" ref={out} aria-live="polite">
          {history.length === 0 && <div className="empty-state">Output appears here.</div>}
          {history.map((entry) =>
            entry.outputs.map((o, i) => (
              <div key={`${entry.id}-${i}`} className={`shell-entry ${o.kind}`}>
                <div className="shell-stmt">
                  <span className="prompt">{entry.db}&gt;</span> <code>{o.statement}</code>
                  <span className="spacer" />
                  {o.kind !== 'error' && <span className="muted small">{formatDuration(o.durationMs)}</span>}
                </div>
                {o.kind === 'documents' ? (
                  <>
                    {(o.documents ?? []).length === 0 && <div className="muted small">(no documents)</div>}
                    {(o.documents ?? []).map((d, j) => (
                      <pre key={j} className="doc-text">
                        {highlight(d.text)}
                      </pre>
                    ))}
                    {o.text && <div className="muted small">{o.text}</div>}
                  </>
                ) : (
                  <pre className={o.kind === 'error' ? 'shell-error' : 'doc-text'}>{o.kind === 'error' ? o.text : highlight(o.text ?? '')}</pre>
                )}
              </div>
            )),
          )}
        </div>
      </div>
    </div>
  )
}
