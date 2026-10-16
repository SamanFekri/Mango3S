import { memo, useMemo, useRef, useState } from 'react'
import type { RefObject } from 'react'
import { copyText, getAt } from '../lib/docTree'
import type { AppError } from '../lib/errors'
import { isTextTarget, shortcutHint } from '../lib/shortcuts'
import { useShortcut } from '../state/shortcuts'
import { formatCount, formatDuration, pageRange } from '../lib/format'
import type { Doc, Page } from '../lib/types'
import { ErrorBanner, Segmented, Spinner } from './common'
import { ResultsNotice, TableView, TreeView, useCellEditing, type FieldApi } from './DocViews'
import { highlight } from './highlight'

export type ViewMode = 'tree' | 'json' | 'table'

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
  /** Whole-document actions; omitted when documents are not editable (aggregation output). */
  onEdit?: (doc: Doc) => void
  /** keyboard: triggered by a shortcut, which always asks first. */
  onDelete?: (docs: Doc[], opts?: { keyboard?: boolean }) => void
  readOnly?: boolean
  emptyHint?: string
  /** In-place field editing (tree and table views). */
  fieldApi?: FieldApi
  /** The server's version of a document after a field edit. */
  onDocumentChanged?: (doc: Doc) => void
  /** Reports an unsaved field edit (for the tab's unsaved-work guard). */
  onDirtyChange?: (reason: string | null) => void
  /** Asks the user; defaults to window.confirm. */
  confirm?: (message: string) => Promise<boolean>
  /** Opens the insert dialog, optionally prefilled (paste). */
  onInsert?: (text?: string) => void
  /** Element the results panel exposes for focus shortcuts. */
  panelRef?: RefObject<HTMLElement | null>
}

const PAGE_SIZES = [10, 20, 50, 100, 200, 500, 1000]

const defaultConfirm = (m: string) => Promise.resolve(typeof window !== 'undefined' && typeof window.confirm === 'function' ? window.confirm(m) : true)

function copy(text: string) {
  try {
    void navigator.clipboard?.writeText(text)
  } catch {
    /* clipboard unavailable */
  }
}

export function ResultsView(props: ResultsViewProps) {
  const { page, loading, error, view, onViewChange, onPage, pageSize, onEdit, onDelete, readOnly } = props
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [expand, setExpand] = useState(false)
  const docs = useMemo(() => page?.documents ?? [], [page])
  const ask = props.confirm ?? defaultConfirm
  const sectionRef = useRef<HTMLElement | null>(null)
  const gridRef = useRef<HTMLDivElement | null>(null)

  // Selection is per page.
  const pageKey = `${page?.page}-${docs.length}-${docs[0]?.id ?? ''}`
  const [selKey, setSelKey] = useState(pageKey)
  if (selKey !== pageKey) {
    setSelKey(pageKey)
    setSelected(new Set())
  }

  const ce = useCellEditing({
    docs,
    api: props.fieldApi,
    readOnly,
    onDocumentChanged: props.onDocumentChanged,
    onDirtyChange: props.onDirtyChange,
    confirm: ask,
  })

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

  /** Runs an action that would leave the current edit, asking first if it has changes. */
  const leaveEdit = async (action: () => void) => {
    if (ce.dirty && !(await ask('Discard the unsaved field edit?'))) return
    ce.cancel()
    action()
  }

  // ---- results shortcuts (focus inside the results panel) -----------------
  const selDoc = ce.sel ? docs[ce.sel.doc] : undefined
  const selValue = ce.sel ? getAt(ce.parsed[ce.sel.doc], ce.sel.path) : undefined
  useShortcut('cell.copy', () => (ce.sel && ce.sel.path.length ? (copy(copyText(selValue)), true) : false), { within: sectionRef })
  useShortcut(
    'cell.copyValue',
    () => {
      if (!ce.sel || !props.fieldApi || !selDoc?.id || !ce.sel.path.length) return false
      props.fieldApi.get(selDoc, ce.sel.path).then((f) => copy(f.expected), () => copy(copyText(selValue)))
    },
    { within: sectionRef },
  )
  useShortcut('doc.copy', () => {
    const d = selDoc ?? selectedDocs[0]
    if (!d) return false
    copy(d.text)
  }, { within: sectionRef })
  useShortcut('docs.selectAll', () => {
    if (!onDelete || readOnly) return false
    setSelected(new Set(docs.map((d) => d.id).filter(Boolean)))
  }, { within: sectionRef })
  useShortcut('edit.undo', () => (ce.canUndo ? void ce.undo() : false), { within: sectionRef })
  useShortcut('edit.redo', () => (ce.canRedo ? void ce.redo() : false), { within: sectionRef })
  useShortcut('doc.editJson', () => {
    const d = selDoc ?? selectedDocs[0]
    if (!d || !editable) return false
    void leaveEdit(() => onEdit!(d))
  }, { within: sectionRef })
  useShortcut('doc.delete', () => {
    if (!onDelete || readOnly) return false
    const targets = selectedDocs.length ? selectedDocs : selDoc?.id ? [selDoc] : []
    if (!targets.length) return false
    onDelete(targets, { keyboard: true }) // the parent confirms
  }, { within: sectionRef })
  useShortcut('field.remove', () => {
    if (!ce.sel || !ce.canEditCell(ce.sel)) return false
    void ce.removeField(ce.sel)
  }, { within: sectionRef })
  useShortcut('focus.results', () => {
    ;(gridRef.current ?? sectionRef.current)?.focus()
  })

  const setRefs = (el: HTMLElement | null) => {
    sectionRef.current = el
    if (props.panelRef) (props.panelRef as { current: HTMLElement | null }).current = el
  }

  return (
    <section
      className="results"
      aria-label="Results"
      ref={setRefs}
      tabIndex={-1}
      onPaste={(e) => {
        if (!props.onInsert || readOnly || isTextTarget(e.target as Element)) return
        const text = e.clipboardData.getData('text')
        if (!text.trim()) return
        e.preventDefault()
        props.onInsert(text)
      }}
    >
      <div className="results-toolbar">
        <Segmented
          label="Result view"
          value={view}
          onChange={(v) => void leaveEdit(() => onViewChange(v))}
          options={[
            { value: 'tree', label: 'Tree', title: `Tree view (${shortcutHint('results.tree')})` },
            { value: 'table', label: 'Table', title: `Table view (${shortcutHint('results.table')})` },
            { value: 'json', label: 'JSON', title: `JSON view (${shortcutHint('results.json')})` },
          ]}
        />
        {view === 'table' && (
          <label className="check small" title="Show the fields of embedded documents as their own columns">
            <input type="checkbox" checked={expand} onChange={(e) => void leaveEdit(() => setExpand(e.target.checked))} />
            Embedded fields
          </label>
        )}
        {onDelete && !readOnly && selectedDocs.length > 0 && (
          <button className="danger small" onClick={() => onDelete(selectedDocs)}>
            Delete {selectedDocs.length} selected
          </button>
        )}
        {props.fieldApi && !readOnly && view !== 'json' && (
          <span className="muted small edit-hint">Double-click a value to edit it</span>
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
            onChange={(e) => {
              const n = Number(e.target.value)
              void leaveEdit(() => props.onPageSize?.(n))
            }}
          >
            {PAGE_SIZES.map((n) => (
              <option key={n} value={n}>
                {n} / page
              </option>
            ))}
          </select>
        )}
        <div className="pager">
          <button className="small" aria-label="First page" disabled={!canPrev || loading} onClick={() => void leaveEdit(() => onPage(0))}>
            «
          </button>
          <button className="small" aria-label="Previous page" disabled={!canPrev || loading} onClick={() => void leaveEdit(() => onPage(page!.page - 1))}>
            ‹
          </button>
          <span className="page-num">{page ? page.page + 1 : '–'}</span>
          <button className="small" aria-label="Next page" disabled={!page?.hasMore || loading} onClick={() => void leaveEdit(() => onPage(page!.page + 1))}>
            ›
          </button>
        </div>
      </div>

      <ErrorBanner error={error} onDismiss={props.onDismissError} />
      {page?.notice && <div className="info-banner">{page.notice}</div>}
      {ce.notice && <ResultsNotice text={ce.notice} onDismiss={() => ce.setNotice(null)} />}

      <div className={loading ? 'results-body dimmed' : 'results-body'}>
        {!page && !loading && !error && <div className="empty-state">{props.emptyHint ?? 'Run a query to see results.'}</div>}
        {page && docs.length === 0 && !page.notice && <div className="empty-state">No documents match.</div>}
        {view === 'json' && (
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
        )}
        {view === 'table' && (
          <TableView
            ce={ce}
            selected={selected}
            onSelectDoc={onDelete && !readOnly ? toggleSel : undefined}
            onEditDocument={editable ? (d) => void leaveEdit(() => onEdit!(d)) : undefined}
            expand={expand}
            gridRef={gridRef}
          />
        )}
        {view === 'tree' && (
          <TreeView
            ce={ce}
            selected={selected}
            onSelectDoc={onDelete && !readOnly ? toggleSel : undefined}
            onEditDocument={editable ? (d) => void leaveEdit(() => onEdit!(d)) : undefined}
            gridRef={gridRef}
            pageOffset={page ? page.page * page.pageSize : 0}
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
