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
import { shortcutHint } from '../lib/shortcuts'
import { useApp, useTabDirty } from '../state/AppContext'
import { useShortcut } from '../state/shortcuts'

interface Entry {
  id: number
  db: string
  outputs: ShellOutput[]
}

const STARTER = `// Cmd/Ctrl+Enter runs everything, or just the selected text.
// F5 runs the whole script, F6 the statement at the cursor, F9 the selection.
// Ctrl+Space completes collections, methods and fields; Alt+↑/↓ recalls history.
show collections
`

/** The block of non-blank lines around the cursor (Studio 3T's "statement at cursor"). */
export function statementAt(text: string, pos: number): string {
  const lines = text.split('\n')
  let offset = 0
  let line = 0
  for (; line < lines.length; line++) {
    if (pos <= offset + lines[line].length) break
    offset += lines[line].length + 1
  }
  line = Math.min(line, lines.length - 1)
  const blank = (i: number) => lines[i].trim() === '' || lines[i].trim().startsWith('//')
  if (blank(line)) return ''
  let a = line
  let b = line
  while (a > 0 && !blank(a - 1)) a--
  while (b < lines.length - 1 && !blank(b + 1)) b++
  return lines.slice(a, b + 1).join('\n').trim()
}

export function ShellView({ tab }: { tab: Tab }) {
  const { sessions, profiles, prompt, toast, toastError } = useApp()
  const connId = tab.connId!
  const session = sessions.find((s) => s.id === connId)
  // IntelliShell settings from the connection (Connection Manager → IntelliShell).
  const shellCfg = profiles.find((p) => p.id === connId)?.shell
  const [db, setDb] = useState(tab.db || shellCfg?.defaultDb || 'test')
  const initial = tab.init?.text ?? (shellCfg?.initScript && !shellCfg.runInitScript ? `${shellCfg.initScript.trim()}\n` : STARTER)
  const [script, setScript] = useState(initial)
  const [savedText, setSavedText] = useState(initial)
  useTabDirty(tab.id, script !== savedText && script.trim() !== '' ? 'a script that is not saved to the library' : null)
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

  const run = async (mode: 'auto' | 'all' | 'selection' | 'statement' = 'auto', text?: string) => {
    const v = editor.current
    let code = text ?? ''
    if (!text) {
      if (mode === 'all') code = v?.state.doc.toString() ?? script
      else if (mode === 'selection') {
        const sel = v?.state.selection.main
        code = sel && !sel.empty ? v!.state.sliceDoc(sel.from, sel.to) : ''
        if (!code) {
          toast({ kind: 'info', title: 'Nothing selected', message: 'Select the commands to run, or press F5 to run the whole script.' })
          return
        }
      } else if (mode === 'statement') code = v ? statementAt(v.state.doc.toString(), v.state.selection.main.head) : ''
      else code = selectionOrAll(v)
    }
    code = code.trim()
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

  const save = async (asNew = false) => {
    const name = await prompt({ title: asNew ? 'Save script as' : 'Save script', label: 'Name', initial: tab.init?.savedName, confirmLabel: 'Save' })
    if (!name) return
    try {
      const item = await backend.saveItem({ id: asNew ? undefined : savedId, kind: 'script', name, db, text: script })
      setSavedId(item.id)
      setSavedText(script)
      toast({ kind: 'success', title: `Saved "${item.name}"` })
    } catch (err) {
      toastError(err, 'Save failed')
    }
  }

  // Run the connection's initialization commands once when the tab opens.
  const initRan = useRef(false)
  useEffect(() => {
    if (initRan.current || tab.init?.text || !shellCfg?.runInitScript || !shellCfg.initScript.trim()) return
    initRan.current = true
    void run('all', shellCfg.initScript)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [shellCfg])

  useShortcut('query.run', () => void run('all'))
  useShortcut('query.runSelection', () => void run('selection'))
  useShortcut('query.runStatement', () => void run('statement'))
  useShortcut('op.cancel', () => (running ? void backend.cancel(opId) : false))
  useShortcut('query.save', () => void save())
  useShortcut('query.saveAs', () => void save(true))
  useShortcut('focus.editor', () => editor.current?.focus())

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
        <button className="small ghost" onClick={() => void save()} title={`Save (${shortcutHint('query.save')})`}>
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
          <button className="primary" onClick={() => void run()} title={`Run the selection or everything (${shortcutHint('query.run')}); F5 runs all, F6 the statement at the cursor, F9 the selection`}>
            ▶ Run
          </button>
        )}
      </div>
      <div className={showHistory ? 'shell-layout with-history' : 'shell-layout'}>
        <div className="shell-split">
          <CodeEditor
            value={script}
            onChange={setScript}
            onRun={() => void run()}
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
