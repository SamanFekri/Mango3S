import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { EditorView } from '@codemirror/view'
import { CodeEditor } from '../components/CodeEditor'
import { Modal, Spinner } from '../components/common'
import { AIDialog } from '../components/AIDialog'
import { CodeDialog } from '../components/CodeDialog'
import { DocumentEditor, INSERT_TEMPLATE } from '../components/DocumentEditor'
import { ExplainDialog } from '../components/ExplainDialog'
import { ExportWizard } from '../components/ExportWizard'
import { ImportWizard } from '../components/ImportWizard'
import { QueryBuilder } from '../components/QueryBuilder'
import type { FieldApi } from '../components/DocViews'
import { ResultsView, type ViewMode } from '../components/ResultsView'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatBytes, formatCount } from '../lib/format'
import { emptyBuilder, type BuilderState } from '../lib/queryBuilder'
import type { Tab } from '../lib/tabs'
import type { CodeRequest, CollectionStats, Doc, ExplainRequest, Page, QueryRequest } from '../lib/types'
import { shortcutHint } from '../lib/shortcuts'
import { useApp, useTabDirty } from '../state/AppContext'
import { useShortcut } from '../state/shortcuts'

interface EditorState {
  mode: 'insert' | 'edit'
  doc?: Doc
  text: string
}

export function CollectionView({ tab }: { tab: Tab }) {
  const { settings, sessions, toast, toastError, confirm, dispatchTabs, prompt } = useApp()
  const connId = tab.connId!
  const db = tab.db!
  const coll = tab.coll!
  const session = sessions.find((s) => s.id === connId)
  const readOnly = !!session?.readOnly
  const opId = `query-${tab.id}`

  const init = tab.init?.query
  const [filter, setFilter] = useState(init?.filter || '{}')
  const [projection, setProjection] = useState(init?.projection ?? '')
  const [sort, setSort] = useState(init?.sort ?? '')
  const [skip, setSkip] = useState(init?.skip ?? 0)
  const [limit, setLimit] = useState(init?.limit ?? 0)
  const [savedId, setSavedId] = useState(tab.init?.savedId)
  const [dialog, setDialog] = useState<
    | { kind: 'explain'; req: ExplainRequest }
    | { kind: 'code'; req: Omit<CodeRequest, 'language'> }
    | { kind: 'export'; req: QueryRequest }
    | { kind: 'import' }
    | { kind: 'ai' }
    | null
  >(null)
  const [pageSize, setPageSize] = useState(settings?.pageSize ?? 50)
  const [view, setView] = useState<ViewMode>(settings?.defaultView ?? 'tree')
  const [cellDirty, setCellDirty] = useState<string | null>(null)
  const filterEditor = useRef<EditorView | null>(null)

  const [page, setPage] = useState<Page | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [total, setTotal] = useState<number | null>(null)
  const [counting, setCounting] = useState(false)

  const [fields, setFields] = useState<string[]>([])
  const [showBuilder, setShowBuilder] = useState(false)
  const [builder, setBuilder] = useState<BuilderState>(emptyBuilder)
  const [editor, setEditor] = useState<EditorState | null>(null)
  const [stats, setStats] = useState<CollectionStats | 'loading' | null>(null)

  // Keep the last executed request so paging re-runs exactly what produced the results.
  const lastReq = useRef<QueryRequest | null>(null)

  const buildReq = useCallback(
    (pageNo: number, size = pageSize): QueryRequest => ({
      connId,
      db,
      coll,
      filter,
      projection,
      sort,
      skip,
      limit,
      page: pageNo,
      pageSize: size,
    }),
    [connId, db, coll, filter, projection, sort, skip, limit, pageSize],
  )

  const execute = useCallback(
    async (req: QueryRequest) => {
      setLoading(true)
      setError(null)
      try {
        const p = await backend.runQuery(opId, req)
        if (lastReq.current?.filter !== req.filter || lastReq.current?.skip !== req.skip || lastReq.current?.limit !== req.limit) {
          setTotal(null)
        }
        lastReq.current = req
        setPage(p)
      } catch (err) {
        const e = toAppError(err)
        if (e.code !== 'CANCELED') setError(e)
      } finally {
        setLoading(false)
      }
    },
    [opId],
  )

  const run = useCallback(() => execute(buildReq(0)), [execute, buildReq])

  const goto = (n: number) => execute({ ...(lastReq.current ?? buildReq(0)), page: n })

  const count = async () => {
    setCounting(true)
    try {
      setTotal(await backend.countQuery(`count-${tab.id}`, lastReq.current ?? buildReq(0)))
    } catch (err) {
      toastError(err, 'Count failed')
    } finally {
      setCounting(false)
    }
  }

  // Initial load: first page of the collection plus field names for autocomplete.
  useEffect(() => {
    run()
    backend.sampleFields(connId, db, coll).then(setFields).catch(() => undefined)
    return () => {
      backend.cancel(opId).catch(() => undefined)
    }
  }, [connId, db, coll])

  const refresh = () => execute(lastReq.current ?? buildReq(0))

  // Collection History (Connection Manager → Collection History).
  useEffect(() => {
    backend.recordCollectionOpened(connId, db, coll).catch(() => undefined)
  }, [connId, db, coll])

  const fieldApi = useMemo<FieldApi>(
    () => ({
      get: (doc, path) => backend.getField(connId, db, coll, doc.id, path),
      update: (doc, u) =>
        backend.updateField({ connId, db, coll, id: doc.id, path: u.path, type: u.type, text: u.text, value: u.value, expected: u.expected, expectedMissing: u.expectedMissing, unset: !!u.unset }),
    }),
    [connId, db, coll],
  )
  const onDocumentChanged = useCallback(
    (nd: Doc) => setPage((p) => (p ? { ...p, documents: p.documents.map((d) => (d.id === nd.id ? nd : d)) } : p)),
    [],
  )
  useTabDirty(tab.id, cellDirty || (editor ? 'an open document editor' : null))

  const onDelete = async (docs: Doc[], opts?: { keyboard?: boolean }) => {
    if (settings?.confirmDeletes !== false || opts?.keyboard) {
      const ok = await confirm({
        title: docs.length === 1 ? 'Delete document' : `Delete ${docs.length} documents`,
        message: (
          <>
            Permanently delete {docs.length === 1 ? <code>{docs[0].id}</code> : `${docs.length} documents`} from{' '}
            <code>
              {db}.{coll}
            </code>
            ?
          </>
        ),
        confirmLabel: 'Delete',
        danger: true,
      })
      if (!ok) return
    }
    try {
      const n = await backend.deleteDocuments(connId, db, coll, docs.map((d) => d.id))
      toast({ kind: 'success', title: `Deleted ${n} document${n === 1 ? '' : 's'}` })
      if (n < docs.length) toast({ kind: 'info', title: `${docs.length - n} document(s) were already gone` })
      setTotal(null)
      refresh()
    } catch (err) {
      toastError(err, 'Delete failed')
    }
  }

  const onEdit = async (doc: Doc) => {
    // Re-read so the editor starts from the current server version.
    try {
      const fresh = await backend.getDocument(connId, db, coll, doc.id)
      setEditor({ mode: 'edit', doc: fresh, text: fresh.text })
    } catch (err) {
      toastError(err, 'Could not open document')
    }
  }

  const copyAsShell = async () => {
    try {
      const code = await backend.queryToShell(buildReq(0))
      await navigator.clipboard?.writeText(code)
      toast({ kind: 'success', title: 'Copied shell code', message: code.split('\n')[0] })
    } catch (err) {
      toastError(err)
    }
  }

  const saveQuery = async (asNew = false) => {
    const name = await prompt({ title: asNew ? 'Save query as' : 'Save query', label: 'Name', initial: tab.init?.savedName ?? `${coll} query`, confirmLabel: 'Save' })
    if (!name) return
    try {
      const item = await backend.saveItem({ id: asNew ? undefined : savedId, kind: 'query', name, db, coll, query: { filter, projection, sort, skip, limit } })
      setSavedId(item.id)
      toast({ kind: 'success', title: `Saved "${item.name}"` })
    } catch (err) {
      toastError(err, 'Save failed')
    }
  }

  const format = async () => {
    try {
      const fmt = async (t: string) => (t.trim() ? backend.formatDocument(t) : t)
      const [f, p, so] = await Promise.all([fmt(filter), fmt(projection), fmt(sort)])
      setFilter(f)
      setProjection(p)
      setSort(so)
    } catch (err) {
      toastError(err, 'Cannot format')
    }
  }
  const openAggregation = () => dispatchTabs({ type: 'open', tab: { kind: 'aggregation', connId, connName: tab.connName, db, coll } })
  const insert = (text = INSERT_TEMPLATE) => (readOnly ? false : setEditor({ mode: 'insert', text }))

  useShortcut('query.run', () => void run())
  useShortcut('view.refresh', () => void refresh())
  useShortcut('op.cancel', () => (loading ? void backend.cancel(opId) : false))
  useShortcut('query.format', () => void format())
  useShortcut('query.save', () => void saveQuery())
  useShortcut('query.saveAs', () => void saveQuery(true))
  useShortcut('query.builder', () => setShowBuilder((b) => !b))
  useShortcut('query.code', () => setDialog({ kind: 'code', req: { source: 'query', db, coll, query: buildReq(0) } }))
  useShortcut('query.explain', () => setDialog({ kind: 'explain', req: { kind: 'find', query: buildReq(0) } }))
  useShortcut('doc.insert', () => insert())
  useShortcut('tab.aggregate', openAggregation)
  useShortcut('results.tree', () => setView('tree'))
  useShortcut('results.table', () => setView('table'))
  useShortcut('results.json', () => setView('json'))
  useShortcut('focus.editor', () => filterEditor.current?.focus())

  const openStats = async () => {
    setStats('loading')
    try {
      setStats(await backend.collectionStats(connId, db, coll))
    } catch (err) {
      setStats(null)
      toastError(err, 'Could not load collection stats')
    }
  }

  return (
    <div className="view collection-view">
      <div className="view-header">
        <div className="breadcrumbs">
          <span>{tab.connName}</span>
          <span className="sep">/</span>
          <span>{db}</span>
          <span className="sep">/</span>
          <strong>{coll}</strong>
          {readOnly && <span className="badge">read-only</span>}
        </div>
        <span className="spacer" />
        <button className="small ghost" onClick={() => setDialog({ kind: 'ai' })} title="Generate a query from a description">
          Ask AI
        </button>
        <button className="small ghost" onClick={openStats}>
          Stats &amp; indexes
        </button>
        <button className="small ghost" onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'schema', connId, connName: tab.connName, db, coll }, reuse: true })}>
          Schema
        </button>
        <button className="small ghost" onClick={() => setDialog({ kind: 'explain', req: { kind: 'find', query: buildReq(0) } })} title={`Explain (${shortcutHint('query.explain')})`}>
          Explain
        </button>
        <button className="small ghost" onClick={() => setDialog({ kind: 'code', req: { source: 'query', db, coll, query: buildReq(0) } })} title={`Query code (${shortcutHint('query.code')})`}>
          Code
        </button>
        <button className="small ghost" onClick={() => void saveQuery()} title={`Save (${shortcutHint('query.save')})`}>
          Save
        </button>
        <button className="small ghost" onClick={() => setDialog({ kind: 'export', req: buildReq(0) })}>
          Export…
        </button>
        {!readOnly && (
          <button className="small ghost" onClick={() => setDialog({ kind: 'import' })}>
            Import…
          </button>
        )}
        <button className="small ghost" onClick={openAggregation} title={`Aggregation editor (${shortcutHint('tab.aggregate')})`}>
          Aggregate
        </button>
        <button className="small ghost" onClick={copyAsShell}>
          Copy as shell
        </button>
        {!readOnly && (
          <button className="small" onClick={() => insert()} title={`Insert document (${shortcutHint('doc.insert')})`}>
            + Insert document
          </button>
        )}
      </div>

      <div className="query-bar">
        <div className="query-grid">
          <label>Filter</label>
          <CodeEditor value={filter} onChange={setFilter} onRun={run} fields={fields} compact maxHeight={160} ariaLabel="Filter" placeholder="{ field: value }" editorRef={filterEditor} />
          <label>Projection</label>
          <CodeEditor value={projection} onChange={setProjection} onRun={run} fields={fields} compact maxHeight={100} ariaLabel="Projection" placeholder="{ field: 1 }" />
          <label>Sort</label>
          <div className="sort-row">
            <CodeEditor value={sort} onChange={setSort} onRun={run} fields={fields} compact maxHeight={100} ariaLabel="Sort" placeholder="{ field: -1 }" />
            <label className="inline">
              Skip
              <input type="number" min={0} value={skip} onChange={(e) => setSkip(Math.max(0, Number(e.target.value) || 0))} />
            </label>
            <label className="inline">
              Limit
              <input type="number" min={0} value={limit} onChange={(e) => setLimit(Math.max(0, Number(e.target.value) || 0))} title="0 = no limit" />
            </label>
          </div>
        </div>
        <div className="query-actions">
          <button className="primary" onClick={run} disabled={loading} title={`Run (${shortcutHint('query.run')} or F5)`}>
            {loading ? <Spinner small /> : '▶'} Run
          </button>
          <button className={showBuilder ? 'active small' : 'small'} onClick={() => setShowBuilder((s) => !s)} aria-pressed={showBuilder} title={`Visual query builder (${shortcutHint('query.builder')})`}>
            Visual builder
          </button>
          <button
            className="small ghost"
            onClick={() => {
              setFilter('{}')
              setProjection('')
              setSort('')
              setSkip(0)
              setLimit(0)
            }}
          >
            Reset
          </button>
        </div>
      </div>

      {showBuilder && (
        <QueryBuilder
          state={builder}
          onChange={setBuilder}
          fields={fields}
          onApply={(q, andRun) => {
            setFilter(q.filter)
            setSort(q.sort)
            setProjection(q.projection)
            if (andRun) {
              execute({ ...buildReq(0), filter: q.filter, sort: q.sort, projection: q.projection })
            }
          }}
        />
      )}

      <ResultsView
        page={page}
        loading={loading}
        error={error}
        onDismissError={() => setError(null)}
        view={view}
        onViewChange={setView}
        onPage={goto}
        pageSize={pageSize}
        onPageSize={(n) => {
          setPageSize(n)
          execute({ ...(lastReq.current ?? buildReq(0)), page: 0, pageSize: n })
        }}
        total={total}
        counting={counting}
        onCount={count}
        onCancel={() => backend.cancel(opId)}
        onEdit={onEdit}
        onDelete={onDelete}
        readOnly={readOnly}
        fieldApi={fieldApi}
        onDocumentChanged={onDocumentChanged}
        onDirtyChange={setCellDirty}
        confirm={(message) => confirm({ title: 'Please confirm', message, confirmLabel: 'OK', danger: true })}
        onInsert={(text) => void insert(text)}
      />

      {editor && (
        <DocumentEditor
          mode={editor.mode}
          title={editor.mode === 'insert' ? `Insert into ${db}.${coll}` : `Edit ${editor.doc?.id}`}
          initial={editor.text}
          fields={fields}
          onClose={() => setEditor(null)}
          onSave={async (text) => {
            if (editor.mode === 'insert') {
              const ids = await backend.insertDocuments(connId, db, coll, text)
              toast({ kind: 'success', title: `Inserted ${ids.length} document${ids.length === 1 ? '' : 's'}` })
            } else {
              await backend.replaceDocument(connId, db, coll, editor.doc!.id, text)
              toast({ kind: 'success', title: 'Document saved' })
            }
            setEditor(null)
            setTotal(null)
            refresh()
          }}
        />
      )}

      {dialog?.kind === 'explain' && <ExplainDialog request={dialog.req} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'code' && <CodeDialog request={dialog.req} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'export' && <ExportWizard query={dialog.req} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'import' && <ImportWizard connId={connId} db={db} coll={coll} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'ai' && (
        <AIDialog
          connId={connId}
          db={db}
          coll={coll}
          onClose={() => setDialog(null)}
          onUse={(p) => {
            if (p.kind === 'aggregate') {
              dispatchTabs({ type: 'open', tab: { kind: 'aggregation', connId, connName: tab.connName, db, coll, init: { stages: p.pipeline } } })
              return
            }
            setFilter(p.filter || '{}')
            setProjection(p.projection)
            setSort(p.sort)
            setLimit(p.limit)
            setSkip(0)
          }}
        />
      )}

      {stats && (
        <Modal title={`${db}.${coll}`} onClose={() => setStats(null)}>
          {stats === 'loading' ? (
            <Spinner />
          ) : (
            <>
              <dl className="stats">
                <dt>Documents</dt>
                <dd>{formatCount(stats.count)}</dd>
                <dt>Data size</dt>
                <dd>{formatBytes(stats.size)}</dd>
                <dt>Avg. document</dt>
                <dd>{formatBytes(stats.avgObjSize)}</dd>
                <dt>Storage size</dt>
                <dd>{formatBytes(stats.storageSize)}</dd>
                <dt>Index size</dt>
                <dd>{formatBytes(stats.totalIndexSize)}</dd>
              </dl>
              <h3>Indexes</h3>
              <table className="grid compact">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Keys</th>
                    <th>Unique</th>
                  </tr>
                </thead>
                <tbody>
                  {stats.indexes.map((i) => (
                    <tr key={i.name}>
                      <td>{i.name}</td>
                      <td className="mono">{i.keys}</td>
                      <td>{i.unique ? 'yes' : ''}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </>
          )}
        </Modal>
      )}
    </div>
  )
}
