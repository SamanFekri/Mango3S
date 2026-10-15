import { memo, useMemo, useState } from 'react'
import { columnsOf, parseDoc, renderCell } from '../lib/ejson'
import type { AppError } from '../lib/errors'
import { formatCount, formatDuration, pageRange } from '../lib/format'
import type { Doc, Page } from '../lib/types'
import { ErrorBanner, Segmented, Spinner } from './common'
import { highlight } from './highlight'

export type ViewMode = 'json' | 'table'

export interface ResultsViewProps {
  page: Page | null
  loading: boolean
  error: AppError | null
  onDismissError?: () => void
  view: ViewMode
  onViewChange: (v: ViewMode) => void
  onPage: (page: number) => void
  pageSize: number
  onPageSize?: (n: number) => void
  total?: number | null
  counting?: boolean
  onCount?: () => void
  onCancel?: () => void
  /** Document actions; omitted when documents are not editable (aggregation output). */
  onEdit?: (doc: Doc) => void
  onDelete?: (docs: Doc[]) => void
  readOnly?: boolean
  emptyHint?: string
}

const PAGE_SIZES = [10, 20, 50, 100, 200, 500, 1000]

export function ResultsView(props: ResultsViewProps) {
  const { page, loading, error, view, onViewChange, onPage, pageSize, onEdit, onDelete, readOnly } = props
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const docs = useMemo(() => page?.documents ?? [], [page])

  // Selection is per page.
  const pageKey = `${page?.page}-${docs.length}-${docs[0]?.id ?? ''}`
  const [selKey, setSelKey] = useState(pageKey)
  if (selKey !== pageKey) {
    setSelKey(pageKey)
    setSelected(new Set())
  }

  const toggleSel = (id: string) =>
    setSelected((s) => {
      const n = new Set(s)
      if (n.has(id)) n.delete(id)
      else n.add(id)
      return n
    })

  const selectedDocs = docs.filter((d) => d.id && selected.has(d.id))
  const editable = !!onEdit && !readOnly
  const canPrev = !!page && page.page > 0

  return (
    <section className="results" aria-label="Results">
      <div className="results-toolbar">
        <Segmented
          label="Result view"
          value={view}
          onChange={onViewChange}
          options={[
            { value: 'json', label: 'JSON' },
            { value: 'table', label: 'Table' },
          ]}
        />
        {onDelete && !readOnly && selectedDocs.length > 0 && (
          <button className="danger small" onClick={() => onDelete(selectedDocs)}>
            Delete {selectedDocs.length} selected
          </button>
        )}
        <span className="spacer" />
        {loading && (
          <>
            <Spinner small />
            {props.onCancel && (
              <button className="small" onClick={props.onCancel}>
                Cancel
              </button>
            )}
          </>
        )}
        {page && !loading && (
          <span className="muted small">
            {pageRange(page.page, page.pageSize, docs.length)}
            {props.total != null && ` of ${formatCount(props.total)}`} · {formatDuration(page.durationMs)}
          </span>
        )}
        {props.onCount && props.total == null && page && (
          <button className="small ghost" onClick={props.onCount} disabled={props.counting}>
            {props.counting ? <Spinner small /> : 'Count'}
          </button>
        )}
        {props.onPageSize && (
          <select
            className="small"
            aria-label="Page size"
            value={pageSize}
            onChange={(e) => props.onPageSize?.(Number(e.target.value))}
          >
            {PAGE_SIZES.map((n) => (
              <option key={n} value={n}>
                {n} / page
              </option>
            ))}
          </select>
        )}
        <div className="pager">
          <button className="small" aria-label="First page" disabled={!canPrev || loading} onClick={() => onPage(0)}>
            «
          </button>
          <button className="small" aria-label="Previous page" disabled={!canPrev || loading} onClick={() => onPage(page!.page - 1)}>
            ‹
          </button>
          <span className="page-num">{page ? page.page + 1 : '–'}</span>
          <button className="small" aria-label="Next page" disabled={!page?.hasMore || loading} onClick={() => onPage(page!.page + 1)}>
            ›
          </button>
        </div>
      </div>

      <ErrorBanner error={error} onDismiss={props.onDismissError} />
      {page?.notice && <div className="info-banner">{page.notice}</div>}

      <div className={loading ? 'results-body dimmed' : 'results-body'}>
        {!page && !loading && !error && <div className="empty-state">{props.emptyHint ?? 'Run a query to see results.'}</div>}
        {page && docs.length === 0 && !page.notice && <div className="empty-state">No documents match.</div>}
        {view === 'json' ? (
          <div className="doc-list">
            {docs.map((d, i) => (
              <DocCard
                key={d.id || `i${i}`}
                doc={d}
                index={page!.page * page!.pageSize + i + 1}
                selected={!!d.id && selected.has(d.id)}
                onSelect={d.id && onDelete && !readOnly ? () => toggleSel(d.id) : undefined}
                onEdit={editable && d.id ? () => onEdit!(d) : undefined}
                onDelete={onDelete && !readOnly && d.id ? () => onDelete([d]) : undefined}
              />
            ))}
          </div>
        ) : (
          <TableView
            docs={docs}
            selected={selected}
            onSelect={onDelete && !readOnly ? toggleSel : undefined}
            onEdit={editable ? onEdit : undefined}
          />
        )}
      </div>
    </section>
  )
}

const DocCard = memo(function DocCard({
  doc,
  index,
  selected,
  onSelect,
  onEdit,
  onDelete,
}: {
  doc: Doc
  index: number
  selected: boolean
  onSelect?: () => void
  onEdit?: () => void
  onDelete?: () => void
}) {
  const lines = doc.text.split('\n').length
  const [collapsed, setCollapsed] = useState(lines > 40)
  const body = useMemo(() => highlight(collapsed ? doc.text.split('\n').slice(0, 12).join('\n') + '\n  …' : doc.text), [doc.text, collapsed])
  const copy = () => navigator.clipboard?.writeText(doc.text)
  return (
    <article className={selected ? 'doc-card selected' : 'doc-card'} onDoubleClick={onEdit}>
      <header>
        {onSelect && <input type="checkbox" checked={selected} onChange={onSelect} aria-label={`Select document ${index}`} />}
        <span className="doc-index">{index}</span>
        <span className="doc-id mono">{doc.id || '(no _id)'}</span>
        <span className="spacer" />
        {lines > 40 && (
          <button className="small ghost" onClick={() => setCollapsed((c) => !c)}>
            {collapsed ? `Expand (${lines} lines)` : 'Collapse'}
          </button>
        )}
        <button className="small ghost" onClick={copy} title="Copy document">
          Copy
        </button>
        {onEdit && (
          <button className="small ghost" onClick={onEdit}>
            Edit
          </button>
        )}
        {onDelete && (
          <button className="small ghost danger-text" onClick={onDelete}>
            Delete
          </button>
        )}
      </header>
      <pre className="doc-text">{body}</pre>
    </article>
  )
})

function TableView({
  docs,
  selected,
  onSelect,
  onEdit,
}: {
  docs: Doc[]
  selected: Set<string>
  onSelect?: (id: string) => void
  onEdit?: (d: Doc) => void
}) {
  const parsed = useMemo(() => docs.map((d) => parseDoc(d.json)), [docs])
  const cols = useMemo(() => columnsOf(parsed), [parsed])
  if (docs.length === 0) return null
  return (
    <div className="table-wrap">
      <table className="grid">
        <thead>
          <tr>
            {onSelect && <th className="sel" />}
            <th className="rownum">#</th>
            {cols.map((c) => (
              <th key={c}>{c}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {parsed.map((row, i) => {
            const d = docs[i]
            return (
              <tr
                key={d.id || i}
                className={d.id && selected.has(d.id) ? 'selected' : ''}
                onDoubleClick={() => d.id && onEdit?.(d)}
                title={onEdit && d.id ? 'Double-click to edit' : undefined}
              >
                {onSelect && (
                  <td className="sel">
                    {d.id && (
                      <input type="checkbox" checked={selected.has(d.id)} onChange={() => onSelect(d.id)} aria-label={`Select row ${i + 1}`} />
                    )}
                  </td>
                )}
                <td className="rownum">{i + 1}</td>
                {cols.map((c) => {
                  const cell = renderCell(row[c])
                  return (
                    <td key={c} className={`cell-${cell.kind}`} title={cell.kind === 'object' || cell.kind === 'array' ? JSON.stringify(row[c], null, 2).slice(0, 2000) : cell.text}>
                      {cell.text}
                    </td>
                  )
                })}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
