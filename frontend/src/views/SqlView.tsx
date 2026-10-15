import { useCallback, useRef, useState } from 'react'
import type { EditorView } from '@codemirror/view'
import { CodeDialog } from '../components/CodeDialog'
import { CodeEditor, selectionOrAll } from '../components/CodeEditor'
import { Spinner } from '../components/common'
import { highlight } from '../components/highlight'
import { ResultsView, type ViewMode } from '../components/ResultsView'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { Tab } from '../lib/tabs'
import type { CodeRequest, Page, SQLTranslation } from '../lib/types'
import { useApp } from '../state/AppContext'

const STARTER = `-- Cmd/Ctrl+Enter runs the query (or the selected text).
-- Supported: SELECT … FROM coll WHERE … GROUP BY … HAVING … ORDER BY … LIMIT … OFFSET …
SELECT *
FROM collection_name
LIMIT 20`

export const SQL_HELP = [
  'SELECT *, fields, field AS alias, DISTINCT',
  'COUNT(*), COUNT(f), SUM, AVG, MIN, MAX with GROUP BY / HAVING',
  "WHERE =, !=, <>, <, <=, >, >=, AND, OR, NOT, IN (…), LIKE / ILIKE '%x_', IS [NOT] NULL, BETWEEN",
  "Values: numbers, 'strings', TRUE, FALSE, OBJECTID('…'), DATE '2024-01-31'",
  'Nested fields: address.city or "odd name"; ORDER BY … ASC|DESC; LIMIT n OFFSET m',
  'Not supported: JOIN, subqueries, UNION, functions, arithmetic, INSERT/UPDATE/DELETE',
]

export function SqlView({ tab }: { tab: Tab }) {
  const { settings, prompt, toast, toastError } = useApp()
  const connId = tab.connId!
  const [db, setDb] = useState(tab.db ?? 'test')
  const [sql, setSql] = useState(tab.init?.text ?? STARTER)
  const [translation, setTranslation] = useState<SQLTranslation | null>(null)
  const [page, setPage] = useState<Page | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [view, setView] = useState<ViewMode>(settings?.defaultView ?? 'json')
  const [pageSize, setPageSize] = useState(settings?.pageSize ?? 50)
  const [showHelp, setShowHelp] = useState(false)
  const [code, setCode] = useState<Omit<CodeRequest, 'language'> | null>(null)
  const [savedId, setSavedId] = useState(tab.init?.savedId)
  const editor = useRef<EditorView | null>(null)
  const lastSQL = useRef('')
  const opId = `sql-${tab.id}`

  const execute = useCallback(
    async (text: string, pageNo: number, size = pageSize) => {
      setLoading(true)
      setError(null)
      try {
        const res = await backend.runSQL(opId, { connId, db, sql: text, page: pageNo, pageSize: size })
        lastSQL.current = text
        setTranslation(res.translation)
        setPage(res.page)
      } catch (err) {
        const e = toAppError(err)
        if (e.code !== 'CANCELED') setError(e)
      } finally {
        setLoading(false)
      }
    },
    [opId, connId, db, pageSize],
  )

  const run = () => {
    const text = selectionOrAll(editor.current).trim()
    if (text) execute(text, 0)
  }

  const translate = async () => {
    try {
      setTranslation(await backend.translateSQL(selectionOrAll(editor.current)))
      setError(null)
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const save = async () => {
    const name = await prompt({ title: 'Save SQL query', label: 'Name', initial: tab.init?.savedName, confirmLabel: 'Save' })
    if (!name) return
    try {
      const item = await backend.saveItem({ id: savedId, kind: 'sql', name, db, text: sql })
      setSavedId(item.id)
      toast({ kind: 'success', title: `Saved "${item.name}"` })
    } catch (err) {
      toastError(err, 'Save failed')
    }
  }

  return (
    <div className="view sql-view">
      <div className="view-header">
        <div className="breadcrumbs">
          <span>{tab.connName}</span>
          <span className="sep">/</span>
          <label className="inline">
            db
            <input className="db-input mono" value={db} onChange={(e) => setDb(e.target.value)} aria-label="Database" spellCheck={false} />
          </label>
          <span className="muted">· SQL</span>
        </div>
        <span className="spacer" />
        <button className="small ghost" onClick={() => setShowHelp((h) => !h)} aria-pressed={showHelp}>
          Syntax
        </button>
        <button className="small ghost" onClick={translate}>
          Translate
        </button>
        <button className="small ghost" onClick={() => setCode({ source: 'sql', db, coll: '', sql: selectionOrAll(editor.current) })}>
          Code
        </button>
        <button className="small ghost" onClick={save}>
          Save
        </button>
        {loading ? (
          <button className="small" onClick={() => backend.cancel(opId)}>
            <Spinner small /> Cancel
          </button>
        ) : (
          <button className="primary" onClick={run} title="Run (Cmd/Ctrl+Enter)">
            ▶ Run
          </button>
        )}
      </div>
      {showHelp && (
        <ul className="sql-help small">
          {SQL_HELP.map((l) => (
            <li key={l}>{l}</li>
          ))}
        </ul>
      )}
      <CodeEditor value={sql} onChange={setSql} onRun={run} editorRef={editor} minHeight={120} maxHeight={260} ariaLabel="SQL" language="sql" />
      {translation && (
        <div className="translation">
          <span className="badge">{translation.kind}</span>
          <pre className="doc-text">{highlight(translation.shell)}</pre>
        </div>
      )}
      <ResultsView
        page={page}
        loading={loading}
        error={error}
        onDismissError={() => setError(null)}
        view={view}
        onViewChange={setView}
        pageSize={pageSize}
        onPageSize={(n) => {
          setPageSize(n)
          if (lastSQL.current) execute(lastSQL.current, 0, n)
        }}
        onPage={(n) => lastSQL.current && execute(lastSQL.current, n)}
        onCancel={() => backend.cancel(opId)}
        emptyHint="Run a SELECT statement to see results."
      />
      {code && <CodeDialog request={code} onClose={() => setCode(null)} />}
    </div>
  )
}
