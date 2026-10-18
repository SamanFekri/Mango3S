import { memo, useMemo, useRef, useState } from 'react'
import type { MouseEvent as ReactMouseEvent, RefObject } from 'react'
import { bsonType, copyText, getAt, isContainer, isPlainObject } from '../lib/docTree'
import type { AppError } from '../lib/errors'
import { SHORTCUT_BY_ID, isTextTarget, shortcutHint } from '../lib/shortcuts'
import { useShortcut } from '../state/shortcuts'
import { formatCount, formatDuration, pageRange } from '../lib/format'
import type { Doc, Page } from '../lib/types'
import { ErrorBanner, Segmented, Spinner } from './common'
import { AddFieldDialog, ContextMenu, DocumentViewer, RenameFieldDialog, type MenuEntry } from './DocDialogs'
import { ResultsNotice, TableView, TreeView, useCellEditing, type CellRef, type FieldApi } from './DocViews'
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
  /** Refresh document: re-read it from the server. */
  onRefreshDocument?: (doc: Doc) => Promise<void>
  /** Follow reference: a DBRef's target, or a value to look up by _id in a collection the user picks. */
  onFollowReference?: (ref: { coll?: string; db?: string; idJSON: string }) => void
  /** Update dialog / Remove dialog for the selected documents (or the query). */
  onBulkDialog?: (kind: 'update' | 'remove', selectedIds: string[]) => void
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
  const bodyRef = useRef<HTMLDivElement | null>(null)

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

  /** Runs an action that would leave the current edit, asking first if it has changes. */
  const leaveEdit = async (action: () => void) => {
    if (ce.dirty && !(await ask('Discard the unsaved field edit?'))) return
    ce.cancel()
    action()
  }

  // ---- document and field commands -----------------------------------------
  // Each command runs from its shortcut (focus in the results panel), from
  // the right-click menu (which shows that shortcut) and from buttons, so
  // all three do exactly the same thing.
  const selDoc = ce.sel ? docs[ce.sel.doc] : undefined
  const selValue = ce.sel ? getAt(ce.parsed[ce.sel.doc], ce.sel.path) : undefined
  const fieldSel = !!ce.sel && ce.sel.path.length > 0
  const targetDoc = selDoc ?? (selectedDocs.length === 1 ? selectedDocs[0] : undefined)
  const writable = !!props.fieldApi && !readOnly
  const [dialog, setDialog] = useState<
    | { kind: 'view'; doc: Doc }
    | { kind: 'add'; container: CellRef; isArray: boolean; label: string }
    | { kind: 'rename'; ref: CellRef }
    | null
  >(null)
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null)

  const parentIsArray = (ref: CellRef) => Array.isArray(getAt(ce.parsed[ref.doc], ref.path.slice(0, -1)))
  const reference = (): { coll?: string; db?: string; idJSON: string } | null => {
    if (!fieldSel) return null
    const v = selValue
    if (isPlainObject(v) && typeof v.$ref === 'string' && v.$id !== undefined) {
      return { coll: v.$ref, db: typeof v.$db === 'string' ? v.$db : undefined, idJSON: JSON.stringify(v.$id) }
    }
    // A manual reference: an ObjectId pointing at another collection's _id.
    if (bsonType(v) === 'objectId') return { idJSON: JSON.stringify(v) }
    return null
  }
  const addTarget = (): { container: CellRef; isArray: boolean; label: string } | null => {
    if (!targetDoc?.id) return null
    const docIndex = docs.indexOf(targetDoc)
    let path = ce.sel && ce.sel.doc === docIndex ? ce.sel.path : []
    if (path.length && !isContainer(getAt(ce.parsed[docIndex], path))) path = path.slice(0, -1)
    const isArray = Array.isArray(getAt(ce.parsed[docIndex], path))
    return { container: { doc: docIndex, path }, isArray, label: path.join('.') }
  }
  const docsToDelete = selectedDocs.length ? selectedDocs : selDoc?.id ? [selDoc] : []

  const commands: Record<string, { run: () => void; enabled: boolean }> = {
    'doc.view': { enabled: !!targetDoc, run: () => targetDoc && setDialog({ kind: 'view', doc: targetDoc }) },
    'doc.editJson': { enabled: editable && !!targetDoc?.id, run: () => targetDoc && void leaveEdit(() => onEdit!(targetDoc)) },
    'doc.copy': { enabled: !!targetDoc, run: () => targetDoc && copy(targetDoc.text) },
    'doc.refresh': { enabled: !!props.onRefreshDocument && !!targetDoc?.id, run: () => targetDoc && void leaveEdit(() => void props.onRefreshDocument!(targetDoc)) },
    'doc.followRef': {
      enabled: !!props.onFollowReference && !!reference(),
      run: () => {
        const ref = reference()
        if (ref) props.onFollowReference!(ref)
      },
    },
    'cell.copy': { enabled: fieldSel, run: () => copy(copyText(selValue)) },
    'cell.copyValue': {
      enabled: fieldSel,
      run: () => {
        if (props.fieldApi && selDoc?.id && ce.sel) props.fieldApi.get(selDoc, ce.sel.path).then((f) => copy(f.expected), () => copy(copyText(selValue)))
        else copy(copyText(selValue))
      },
    },
    'field.edit': { enabled: fieldSel && writable && ce.canEditCell(ce.sel!), run: () => ce.sel && ce.startEdit(ce.sel) },
    'field.add': {
      enabled: writable && !!addTarget(),
      run: () => {
        const t = addTarget()
        if (t) setDialog({ kind: 'add', ...t })
      },
    },
    'field.rename': {
      enabled: fieldSel && writable && ce.canEditCell(ce.sel!) && !parentIsArray(ce.sel!),
      run: () => ce.sel && setDialog({ kind: 'rename', ref: ce.sel }),
    },
    'field.remove': { enabled: fieldSel && writable && ce.canEditCell(ce.sel!), run: () => ce.sel && void ce.removeField(ce.sel) },
    'doc.insert': { enabled: !!props.onInsert && !readOnly, run: () => props.onInsert?.() },
    'docs.paste': {
      enabled: !!props.onInsert && !readOnly,
      run: () => {
        const ask = navigator.clipboard?.readText?.()
        if (!ask) return ce.setNotice(`Press ${shortcutHint('docs.paste')} in the results to paste documents.`)
        ask.then(
          (t) => (t.trim() ? props.onInsert?.(t) : ce.setNotice('The clipboard is empty.')),
          () => ce.setNotice(`Clipboard access was denied; press ${shortcutHint('docs.paste')} in the results instead.`),
        )
      },
    },
    'dialog.update': { enabled: !!props.onBulkDialog && !readOnly, run: () => props.onBulkDialog?.('update', docsToDelete.map((d) => d.id)) },
    'dialog.remove': { enabled: !!props.onBulkDialog && !readOnly, run: () => props.onBulkDialog?.('remove', docsToDelete.map((d) => d.id)) },
    'doc.delete': { enabled: !!onDelete && !readOnly && docsToDelete.length > 0, run: () => onDelete?.(docsToDelete, { keyboard: true }) },
  }
  const cmd = (id: string) => () => (commands[id].enabled ? commands[id].run() : false)
  useShortcut('doc.view', cmd('doc.view'), { within: sectionRef })
  useShortcut('doc.editJson', cmd('doc.editJson'), { within: sectionRef })
  useShortcut('doc.copy', cmd('doc.copy'), { within: sectionRef })
  useShortcut('doc.refresh', cmd('doc.refresh'), { within: sectionRef })
  useShortcut('doc.followRef', cmd('doc.followRef'), { within: sectionRef })
  useShortcut('doc.delete', cmd('doc.delete'), { within: sectionRef })
  useShortcut('cell.copy', cmd('cell.copy'), { within: sectionRef })
  useShortcut('cell.copyValue', cmd('cell.copyValue'), { within: sectionRef })
  useShortcut('field.edit', cmd('field.edit'), { within: sectionRef })
  useShortcut('field.add', cmd('field.add'), { within: sectionRef })
  useShortcut('field.rename', cmd('field.rename'), { within: sectionRef })
  useShortcut('field.remove', cmd('field.remove'), { within: sectionRef })
  useShortcut('dialog.update', cmd('dialog.update'), { within: sectionRef })
  useShortcut('dialog.remove', cmd('dialog.remove'), { within: sectionRef })
  useShortcut('docs.selectAll', () => {
    if (!onDelete || readOnly) return false
    setSelected(new Set(docs.map((d) => d.id).filter(Boolean)))
  }, { within: sectionRef })
  useShortcut('edit.undo', () => (ce.canUndo ? void ce.undo() : false), { within: sectionRef })
  useShortcut('edit.redo', () => (ce.canRedo ? void ce.redo() : false), { within: sectionRef })

  const menuItems = (): (MenuEntry | null)[] => {
    const e = (id: string, danger = false): MenuEntry => ({ id, label: SHORTCUT_BY_ID[id].label, ...commands[id], danger })
    const fieldItems = fieldSel ? [e('cell.copy'), e('cell.copyValue'), e('field.edit'), e('field.add'), e('field.rename'), e('field.remove', true), null] : [e('field.add'), null]
    return [
      e('doc.view'), e('doc.editJson'), e('doc.copy'), e('doc.refresh'), ...(reference() ? [e('doc.followRef')] : []), null,
      ...fieldItems,
      e('doc.insert'), e('docs.paste'), null,
      e('dialog.update'), e('dialog.remove'), e('doc.delete', true),
    ]
  }
  const openMenu = (ref: CellRef, ev: ReactMouseEvent) => {
    ev.preventDefault()
    ce.select(ref)
    setMenu({ x: ev.clientX, y: ev.clientY })
  }

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
        <Pager label="Pagination" page={page} loading={loading} onPage={(n) => void leaveEdit(() => onPage(n))} />
      </div>

      <ErrorBanner error={error} onDismiss={props.onDismissError} />
      {page?.notice && <div className="info-banner">{page.notice}</div>}
      {ce.notice && <ResultsNotice text={ce.notice} onDismiss={() => ce.setNotice(null)} />}

      <div className={loading ? 'results-body dimmed' : 'results-body'} ref={bodyRef}>
        {!page && !loading && !error && <div className="empty-state">{props.emptyHint ?? 'Run a query to see results.'}</div>}
        {page && docs.length === 0 && !page.notice && <div className="empty-state">No documents match.</div>}
        {view === 'json' && (
          <div className="doc-list">
            {docs.map((d, i) => (
              <DocCard
                key={d.id || `i${i}`}
                focused={ce.sel?.doc === i}
                onFocusDoc={() => ce.select({ doc: i, path: [] })}
                onContextMenu={(ev) => openMenu({ doc: i, path: [] }, ev)}
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
            onContextMenu={openMenu}
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
            onContextMenu={openMenu}
          />
        )}
      </div>
      {page && docs.length > 0 && (
        <div className="results-footer">
          <span className="muted small">
            {pageRange(page.page, page.pageSize, docs.length)}
            {props.total != null && ` of ${formatCount(props.total)}`}
          </span>
          <span className="spacer" />
          <Pager
            label="Pagination (bottom)"
            page={page}
            loading={loading}
            onPage={(n) =>
              void leaveEdit(() => {
                onPage(n)
                bodyRef.current?.scrollTo?.({ top: 0 })
              })
            }
          />
        </div>
      )}
      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems()} onClose={() => setMenu(null)} />}
      {dialog?.kind === 'view' && (
        <DocumentViewer
          title={`Document ${dialog.doc.id || ''}`}
          text={dialog.doc.text}
          onClose={() => setDialog(null)}
          onEdit={editable && dialog.doc.id ? () => onEdit!(dialog.doc) : undefined}
        />
      )}
      {dialog?.kind === 'add' && (
        <AddFieldDialog
          target={dialog.label}
          isArray={dialog.isArray}
          onClose={() => setDialog(null)}
          onSave={(name, type, text) => ce.addField(dialog.container, name, type, text)}
        />
      )}
      {dialog?.kind === 'rename' && (
        <RenameFieldDialog path={dialog.ref.path} onClose={() => setDialog(null)} onSave={(name) => ce.renameField(dialog.ref, name)} />
      )}
    </section>
  )
}

const DocCard = memo(function DocCard({
  doc,
  index,
  selected,
  focused,
  onFocusDoc,
  onContextMenu,
  onSelect,
  onEdit,
  onDelete,
}: {
  doc: Doc
  index: number
  selected: boolean
  focused: boolean
  onFocusDoc: () => void
  onContextMenu: (e: ReactMouseEvent) => void
  onSelect?: () => void
  onEdit?: () => void
  onDelete?: () => void
}) {
  const lines = doc.text.split('\n').length
  const [collapsed, setCollapsed] = useState(lines > 40)
  const body = useMemo(() => highlight(collapsed ? doc.text.split('\n').slice(0, 12).join('\n') + '\n  …' : doc.text), [doc.text, collapsed])
  const copy = () => navigator.clipboard?.writeText(doc.text)
  return (
    <article
      className={`doc-card${selected ? ' selected' : ''}${focused ? ' focused' : ''}`}
      onClick={onFocusDoc}
      onDoubleClick={onEdit}
      onContextMenu={onContextMenu}
      aria-current={focused || undefined}
    >
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

/** First / previous / page number / next. Shown above and below the results. */
function Pager({ label, page, loading, onPage }: { label: string; page: Page | null; loading: boolean; onPage: (n: number) => void }) {
  const canPrev = !!page && page.page > 0
  return (
    <nav className="pager" aria-label={label}>
      <button className="small" aria-label="First page" disabled={!canPrev || loading} onClick={() => onPage(0)}>
        «
      </button>
      <button className="small" aria-label="Previous page" disabled={!canPrev || loading} onClick={() => onPage(page!.page - 1)}>
        ‹
      </button>
      <span className="page-num" aria-label="Page">
        {page ? page.page + 1 : '–'}
      </span>
      <button className="small" aria-label="Next page" disabled={!page?.hasMore || loading} onClick={() => onPage(page!.page + 1)}>
        ›
      </button>
    </nav>
  )
}
