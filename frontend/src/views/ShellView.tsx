import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { CompletionContext, CompletionResult } from '@codemirror/autocomplete'
import type { EditorView, KeyBinding } from '@codemirror/view'
import { CodeEditor, selectionOrAll } from '../components/CodeEditor'
import { Spinner } from '../components/common'
import { highlight } from '../components/highlight'
import { backend } from '../lib/backend'
import { toAppError } from '../lib/errors'
import { formatDuration } from '../lib/format'
import { timeAgo } from '../lib/library'
import { currentCollection, historyStep, shellCompletions } from '../lib/shellComplete'
import type { Tab } from '../lib/tabs'
import type { HistoryEntry, ShellOutput } from '../lib/types'
import { useApp } from '../state/AppContext'

interface Entry {
  id: number
  db: string
  outputs: ShellOutput[]
}

const STARTER = `// Cmd/Ctrl+Enter runs everything, or just the selected text.
// Ctrl+Space completes collections, methods and fields; Alt+↑/↓ recalls history.
show collections
`

export function ShellView({ tab }: { tab: Tab }) {
  const { sessions, prompt, toast, toastError } = useApp()
  const connId = tab.connId!
  const session = sessions.find((s) => s.id === connId)
  const [db, setDb] = useState(tab.db || 'test')
  const [script, setScript] = useState(tab.init?.text ?? STARTER)
  const [outputs, setOutputs] = useState<Entry[]>([])
  const [running, setRunning] = useState(false)
  const [showHistory, setShowHistory] = useState(false)
  const [history, setHistory] = useState<HistoryEntry[]>([])
  const [savedId, setSavedId] = useState(tab.init?.savedId)
  const editor = useRef<EditorView | null>(null)
  const out = useRef<HTMLDivElement>(null)
  const seq = useRef(0)
  const opId = `shell-${tab.id}`

  // Completion context: collections of the current db and fields per collection.
  const collections = useRef<string[]>([])
  const fieldCache = useRef(new Map<string, string[] | 'loading'>())
  useEffect(() => {
    backend
      .listCollections(connId, db)
      .then((cs) => (collections.current = cs.map((c) => c.name)))
      .catch(() => (collections.current = []))
    fieldCache.current.clear()
  }, [connId, db])

  const completionSource = useCallback(
    (ctx: CompletionContext): CompletionResult | null => {
      const start = Math.max(0, ctx.pos - 2000)
      const before = ctx.state.sliceDoc(start, ctx.pos)
      const coll = currentCollection(before)
      let fields: string[] = []
      if (coll) {
        const cached = fieldCache.current.get(coll)
        if (Array.isArray(cached)) fields = cached
        else if (!cached) {
          fieldCache.current.set(coll, 'loading')
          backend
            .sampleFields(connId, db, coll)
            .then((f) => fieldCache.current.set(coll, f))
            .catch(() => fieldCache.current.set(coll, []))
        }
      }
      const res = shellCompletions(before, { collections: collections.current, fields })
      if (!res || (!ctx.explicit && res.from === before.length && !/[.\s]$/.test(before))) return null
      return { from: start + res.from, options: res.options, validFor: /^[$\w]*$/ }
    },
    [connId, db],
  )

  // History recall: this tab's runs first, then the persisted script history.
  const runs = useRef<string[]>([])
  const histIndex = useRef(-1)
  const draft = useRef('')
  const recall = useCallback(
    (older: boolean) => {
      const v = editor.current
      if (!v) return false
      const recallList = [...runs.current, ...history.map((h) => h.text ?? '').filter(Boolean)].filter((s, i, a) => a.indexOf(s) === i)
      if (histIndex.current === -1) draft.current = v.state.doc.toString()
      histIndex.current = historyStep(recallList, histIndex.current, older)
      const text = histIndex.current === -1 ? draft.current : recallList[histIndex.current]
      v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: text }, selection: { anchor: text.length } })
      return true
    },
    [history],
  )
  const keyBindings: KeyBinding[] = useMemo(
    () => [
      { key: 'Alt-ArrowUp', run: () => recall(true) },
      { key: 'Alt-ArrowDown', run: () => recall(false) },
    ],
    [recall],
  )

  const loadHistory = useCallback(() => {
    backend
      .listHistory('script', '', 100)
      .then(setHistory)
      .catch(() => undefined)
  }, [])
  useEffect(loadHistory, [loadHistory])

  useEffect(
    () => () => {
      backend.cancel(opId).catch(() => undefined)
    },
    [opId],
  )

  useEffect(() => {
    out.current?.scrollTo({ top: out.current.scrollHeight, behavior: 'smooth' })
  }, [outputs])

  const run = async () => {
    const code = selectionOrAll(editor.current).trim()
    if (!code || running) return
    setRunning(true)
    seq.current += 1
    const id = seq.current
    runs.current = [code, ...runs.current.filter((r) => r !== code)].slice(0, 50)
    histIndex.current = -1
    try {
      const res = await backend.runShell(opId, connId, db, code)
      setOutputs((h) => [...h.slice(-49), { id, db, outputs: res.outputs }])
      setDb(res.currentDb)
    } catch (err) {
      const e = toAppError(err)
      setOutputs((h) => [
        ...h.slice(-49),
        { id, db, outputs: [{ statement: code.split('\n')[0], kind: 'error', text: e.message, hasMore: false, durationMs: 0 }] },
      ])
    } finally {
      setRunning(false)
      loadHistory()
    }
  }

  const save = async () => {
    const name = await prompt({ title: 'Save script', label: 'Name', initial: tab.init?.savedName, confirmLabel: 'Save' })
    if (!name) return
    try {
      const item = await backend.saveItem({ id: savedId, kind: 'script', name, db, text: script })
      setSavedId(item.id)
      toast({ kind: 'success', title: `Saved "${item.name}"` })
    } catch (err) {
      toastError(err, 'Save failed')
    }
  }

  const insert = (text: string) => {
    const v = editor.current
    if (!v) return
    v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: text } })
    v.focus()
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
        <button className={showHistory ? 'small active' : 'small ghost'} onClick={() => setShowHistory((s) => !s)} aria-pressed={showHistory}>
          History
        </button>
        <button className="small ghost" onClick={save}>
          Save
        </button>
        <button className="small ghost" onClick={() => setOutputs([])} disabled={outputs.length === 0}>
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
      <div className={showHistory ? 'shell-layout with-history' : 'shell-layout'}>
        <div className="shell-split">
          <CodeEditor
            value={script}
            onChange={setScript}
            onRun={run}
            editorRef={editor}
            minHeight={140}
            maxHeight={320}
            ariaLabel="Shell script"
            autoFocus
            completionSource={completionSource}
            keyBindings={keyBindings}
          />
          <div className="shell-output" ref={out} aria-live="polite">
            {outputs.length === 0 && <div className="empty-state">Output appears here.</div>}
            {outputs.map((entry) =>
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
        {showHistory && (
          <aside className="shell-history" aria-label="Script history">
            <div className="muted small pad">Click to load into the editor.</div>
            <ul>
              {history.map((h) => (
                <li key={h.id}>
                  <button className={h.ok ? 'history-item' : 'history-item failed'} onClick={() => insert(h.text ?? h.summary)} title={h.error}>
                    <code>{h.summary}</code>
                    <span className="muted small">
                      {h.db} · {timeAgo(h.at)}
                    </span>
                  </button>
                </li>
              ))}
              {history.length === 0 && <li className="muted small pad">No scripts run yet.</li>}
            </ul>
          </aside>
        )}
      </div>
    </div>
  )
}
