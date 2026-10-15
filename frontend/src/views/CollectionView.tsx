import { useCallback, useEffect, useRef, useState } from 'react'
import { CodeEditor } from '../components/CodeEditor'
import { Modal, Spinner } from '../components/common'
import { DocumentEditor, INSERT_TEMPLATE } from '../components/DocumentEditor'
import { QueryBuilder } from '../components/QueryBuilder'
import { ResultsView, type ViewMode } from '../components/ResultsView'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatBytes, formatCount } from '../lib/format'
import { emptyBuilder, type BuilderState } from '../lib/queryBuilder'
import type { Tab } from '../lib/tabs'
import type { CollectionStats, Doc, Page, QueryRequest } from '../lib/types'
import { useApp } from '../state/AppContext'

interface EditorState {
  mode: 'insert' | 'edit'
  doc?: Doc
  text: string
}

export function CollectionView({ tab }: { tab: Tab }) {
  const { settings, sessions, toast, toastError, confirm, dispatchTabs } = useApp()
  const connId = tab.connId!
  const db = tab.db!
  const coll = tab.coll!
  const session = sessions.find((s) => s.id === connId)
  const readOnly = !!session?.readOnly
  const opId = `query-${tab.id}`

  const [filter, setFilter] = useState('{}')
  const [projection, setProjection] = useState('')
  const [sort, setSort] = useState('')
  const [skip, setSkip] = useState(0)
  const [limit, setLimit] = useState(0)
  const [pageSize, setPageSize] = useState(settings?.pageSize ?? 50)
  const [view, setView] = useState<ViewMode>(settings?.defaultView ?? 'json')

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

  const onDelete = async (docs: Doc[]) => {
    if (settings?.confirmDeletes !== false) {
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
        <button className="small ghost" onClick={openStats}>
          Stats &amp; indexes
        </button>
        <button
          className="small ghost"
          onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'aggregation', connId, connName: tab.connName, db, coll } })}
        >
          Aggregate
        </button>
        <button className="small ghost" onClick={copyAsShell}>
          Copy as shell
        </button>
        {!readOnly && (
          <button className="small" onClick={() => setEditor({ mode: 'insert', text: INSERT_TEMPLATE })}>
            + Insert document
          </button>
        )}
      </div>

      <div className="query-bar">
        <div className="query-grid">
          <label>Filter</label>
          <CodeEditor value={filter} onChange={setFilter} onRun={run} fields={fields} compact maxHeight={160} ariaLabel="Filter" placeholder="{ field: value }" />
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
          <button className="primary" onClick={run} disabled={loading} title="Run (Cmd/Ctrl+Enter)">
            {loading ? <Spinner small /> : '▶'} Run
          </button>
          <button className={showBuilder ? 'active small' : 'small'} onClick={() => setShowBuilder((s) => !s)} aria-pressed={showBuilder}>
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
