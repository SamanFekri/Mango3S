// Tree and table result views with in-place field editing.
//
// Interaction contract (tested in docviews.test.tsx):
//  - A single click selects a cell. It never starts editing.
//  - A double click edits that one field only.
//  - Enter saves, Escape cancels, Tab / Shift+Tab save and move to the next /
//    previous editable cell. Invalid values show an error next to the field.
//  - An edit with changes is never dropped silently: clicking elsewhere keeps
//    the editor open and says so; the tab reports unsaved work.
//  - The grid shows the new value only after the server confirms the update,
//    using the document the server returns.
import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { KeyboardEvent as ReactKeyboardEvent, MouseEvent as ReactMouseEvent, ReactNode, RefObject } from 'react'
import { bsonType, childrenOf, displayValue, getAt, isContainer, pathKey, tableColumns, type FieldPath } from '../lib/docTree'
import { parseDoc, renderCell, type JsonObject } from '../lib/ejson'
import { toAppError } from '../lib/errors'
import type { Doc, FieldResult, FieldValue } from '../lib/types'
import { Spinner } from './common'
import { TypeIcon } from './icons'

/** Server operations for one collection; omitted when results aren't editable. */
export interface FieldApi {
  get(doc: Doc, path: FieldPath): Promise<FieldValue>
  update(
    doc: Doc,
    u: {
      path: FieldPath
      type: string
      text: string
      value?: string
      expected: string
      expectedMissing: boolean
      unset?: boolean
      renameTo?: string
      append?: boolean
    },
  ): Promise<FieldResult>
}

export interface CellRef {
  doc: number
  path: FieldPath
}

const sameRef = (a: CellRef | null, b: CellRef | null) => !!a && !!b && a.doc === b.doc && pathKey(a.path) === pathKey(b.path)

interface EditState {
  ref: CellRef
  loading: boolean
  field?: FieldValue
  text: string
  error?: string
  saving: boolean
  /** Shown when the user tried to leave an edit with unsaved changes. */
  blocked?: boolean
}

interface UndoEntry {
  docId: string
  path: FieldPath
  before: string | null // shell syntax, null = field was missing
  after: string | null
}

export interface CellEditing {
  docs: Doc[]
  parsed: JsonObject[]
  editable: boolean
  sel: CellRef | null
  edit: EditState | null
  dirty: boolean
  select: (ref: CellRef) => void
  startEdit: (ref: CellRef) => void
  setText: (t: string) => void
  commit: () => Promise<boolean>
  cancel: () => void
  canEditCell: (ref: CellRef) => boolean
  removeField: (ref: CellRef) => Promise<void>
  /** Adds a field to the document/embedded document at container, or a value to the array at container. */
  addField: (container: CellRef, name: string, type: string, text: string) => Promise<void>
  renameField: (ref: CellRef, newName: string) => Promise<void>
  undo: () => Promise<boolean>
  redo: () => Promise<boolean>
  notice: string | null
  setNotice: (n: string | null) => void
  canUndo: boolean
  canRedo: boolean
}

export function useCellEditing(opts: {
  docs: Doc[]
  api?: FieldApi
  readOnly?: boolean
  onDocumentChanged?: (d: Doc) => void
  onDirtyChange?: (reason: string | null) => void
  confirm?: (message: string) => Promise<boolean>
}): CellEditing {
  const { docs, api, readOnly, onDocumentChanged, onDirtyChange } = opts
  const parsed = useMemo(() => docs.map((d) => parseDoc(d.json)), [docs])
  const [sel, setSel] = useState<CellRef | null>(null)
  const [edit, setEdit] = useState<EditState | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [undoStack, setUndo] = useState<UndoEntry[]>([])
  const [redoStack, setRedo] = useState<UndoEntry[]>([])
  const editable = !!api && !readOnly
  const editRef = useRef(edit)
  editRef.current = edit

  const dirty = !!edit && !!edit.field && edit.text !== edit.field.text
  useEffect(() => {
    onDirtyChange?.(dirty ? 'a field edit that has not been saved' : null)
  }, [dirty, onDirtyChange])
  useEffect(() => () => onDirtyChange?.(null), [onDirtyChange])

  const canEditCell = useCallback(
    (ref: CellRef) => editable && !!docs[ref.doc]?.id && ref.path.length > 0 && ref.path[0] !== '_id',
    [editable, docs],
  )

  const select = useCallback((ref: CellRef) => {
    const e = editRef.current
    if (e && !sameRef(e.ref, ref)) {
      if (e.field && e.text !== e.field.text) {
        setEdit({ ...e, blocked: true })
        return // keep the unsaved edit; the editor says why
      }
      setEdit(null)
    }
    setSel(ref)
  }, [])

  const startEdit = useCallback(
    (ref: CellRef) => {
      const e = editRef.current
      if (e && sameRef(e.ref, ref)) return
      if (e && e.field && e.text !== e.field.text) {
        setEdit({ ...e, blocked: true })
        return
      }
      setSel(ref)
      if (!api || readOnly) {
        setNotice(readOnly ? 'This connection is read-only.' : 'These results are not editable.')
        return
      }
      const doc = docs[ref.doc]
      if (!doc?.id) {
        setNotice('This document has no _id, so its fields cannot be edited in place.')
        return
      }
      if (ref.path[0] === '_id') {
        setNotice('_id cannot be changed.')
        return
      }
      setNotice(null)
      setEdit({ ref, loading: true, text: '', saving: false })
      api.get(doc, ref.path).then(
        (field) => {
          setEdit((cur) =>
            cur && sameRef(cur.ref, ref)
              ? field.editable
                ? { ...cur, loading: false, field, text: field.text }
                : null
              : cur,
          )
          if (!field.editable) setNotice(field.reason ?? 'This field cannot be edited in place.')
        },
        (err) => {
          setEdit((cur) => (cur && sameRef(cur.ref, ref) ? null : cur))
          setNotice(toAppError(err).message)
        },
      )
    },
    [api, readOnly, docs],
  )

  const setText = useCallback((t: string) => setEdit((e) => (e ? { ...e, text: t, error: undefined, blocked: false } : e)), [])

  const cancel = useCallback(() => setEdit(null), [])

  const pushUndo = (docId: string, path: FieldPath, res: FieldResult) => {
    setUndo((u) => [...u.slice(-49), { docId, path, before: res.existed ? res.previous : null, after: res.current || null }])
    setRedo([])
  }

  const commit = useCallback(async () => {
    const e = editRef.current
    if (!e || !api) return true
    if (e.loading || !e.field) return false
    if (e.text === e.field.text) {
      setEdit(null) // nothing changed: no write
      return true
    }
    const doc = docs[e.ref.doc]
    setEdit({ ...e, saving: true, error: undefined })
    try {
      const res = await api.update(doc, {
        path: e.ref.path,
        type: e.field.exists ? e.field.type : 'auto',
        text: e.text,
        expected: e.field.expected,
        expectedMissing: !e.field.exists,
      })
      onDocumentChanged?.(res.document)
      pushUndo(doc.id, e.ref.path, res)
      setEdit(null)
      setNotice(null)
      return true
    } catch (err) {
      // Keep the user's text; the stored value is unchanged.
      setEdit((cur) => (cur ? { ...cur, saving: false, error: toAppError(err).message } : cur))
      return false
    }
  }, [api, docs, onDocumentChanged])

  const findDoc = (id: string) => docs.findIndex((d) => d.id === id)

  const apply = async (entry: UndoEntry, to: string | null, from: string | null) => {
    const i = findDoc(entry.docId)
    if (i < 0 || !api) {
      setNotice('That document is not on this page any more.')
      return null
    }
    try {
      const res = await api.update(docs[i], {
        path: entry.path,
        type: 'auto',
        text: '',
        value: to ?? undefined,
        unset: to === null,
        expected: from ?? '',
        expectedMissing: from === null,
      })
      onDocumentChanged?.(res.document)
      setNotice(null)
      return res
    } catch (err) {
      setNotice(toAppError(err).message)
      return null
    }
  }

  const undo = useCallback(async () => {
    const entry = undoStack[undoStack.length - 1]
    if (!entry || editRef.current) return false
    if (await apply(entry, entry.before, entry.after)) {
      setUndo((u) => u.slice(0, -1))
      setRedo((r) => [...r, entry])
    }
    return true
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [undoStack, docs, api])

  const redo = useCallback(async () => {
    const entry = redoStack[redoStack.length - 1]
    if (!entry || editRef.current) return false
    if (await apply(entry, entry.after, entry.before)) {
      setRedo((r) => r.slice(0, -1))
      setUndo((u) => [...u, entry])
    }
    return true
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [redoStack, docs, api])

  const removeField = useCallback(
    async (ref: CellRef) => {
      if (!canEditCell(ref) || !api) return
      const doc = docs[ref.doc]
      try {
        const field = await api.get(doc, ref.path)
        if (!field.exists) {
          setNotice('This document has no such field.')
          return
        }
        if (opts.confirm && !(await opts.confirm(`Remove the field “${ref.path.join('.')}” from this document?`))) return
        const res = await api.update(doc, { path: ref.path, type: field.type, text: '', expected: field.expected, expectedMissing: false, unset: true })
        onDocumentChanged?.(res.document)
        setUndo((u) => [...u.slice(-49), { docId: doc.id, path: ref.path, before: res.previous, after: null }])
        setRedo([])
      } catch (err) {
        setNotice(toAppError(err).message)
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [canEditCell, api, docs, onDocumentChanged, opts.confirm],
  )

  const addField = useCallback(
    async (container: CellRef, name: string, type: string, text: string) => {
      if (!api || readOnly) throw new Error(JSON.stringify({ code: 'READ_ONLY', message: 'This connection is read-only.' }))
      const doc = docs[container.doc]
      const target = getAt(parsed[container.doc], container.path)
      if (Array.isArray(target)) {
        const arr = await api.get(doc, container.path)
        const res = await api.update(doc, { path: container.path, type, text, expected: arr.expected, expectedMissing: false, append: true })
        onDocumentChanged?.(res.document)
        setUndo((u) => [...u.slice(-49), { docId: doc.id, path: container.path, before: res.previous, after: res.current }])
        setRedo([])
        return
      }
      const path = [...container.path, name]
      const res = await api.update(doc, { path, type, text, expected: '', expectedMissing: true })
      onDocumentChanged?.(res.document)
      setUndo((u) => [...u.slice(-49), { docId: doc.id, path, before: null, after: res.current }])
      setRedo([])
      setSel({ doc: container.doc, path })
    },
    [api, readOnly, docs, parsed, onDocumentChanged],
  )

  const renameField = useCallback(
    async (ref: CellRef, newName: string) => {
      if (!api || readOnly) throw new Error(JSON.stringify({ code: 'READ_ONLY', message: 'This connection is read-only.' }))
      const doc = docs[ref.doc]
      const field = await api.get(doc, ref.path)
      const res = await api.update(doc, { path: ref.path, type: field.type, text: '', expected: field.expected, expectedMissing: !field.exists, renameTo: newName })
      onDocumentChanged?.(res.document)
      setSel({ doc: ref.doc, path: [...ref.path.slice(0, -1), newName] })
    },
    [api, readOnly, docs, onDocumentChanged],
  )

  // A new page invalidates selection; an unsaved edit is guarded by the caller.
  const pageKey = docs.map((d) => d.id).join('|')
  const lastKey = useRef(pageKey)
  if (lastKey.current !== pageKey) {
    lastKey.current = pageKey
    if (sel && sel.doc >= docs.length) setSel(null)
    if (edit && (edit.ref.doc >= docs.length || docs[edit.ref.doc]?.id === undefined)) setEdit(null)
  }

  return {
    docs, parsed, editable, sel, edit, dirty, select, startEdit, setText, commit, cancel, canEditCell, removeField, addField, renameField, undo, redo,
    notice, setNotice, canUndo: undoStack.length > 0, canRedo: redoStack.length > 0,
  }
}

/** The inline editor for one field. */
function CellEditor({ ce, onMove }: { ce: CellEditing; onMove: (delta: number) => void }) {
  const e = ce.edit!
  const inputRef = useRef<HTMLInputElement & HTMLTextAreaElement>(null)
  useEffect(() => {
    if (!e.loading) {
      inputRef.current?.focus()
      inputRef.current?.select()
    }
  }, [e.loading])
  if (e.loading || !e.field) {
    return (
      <span className="cell-editor loading">
        <Spinner small /> Loading…
      </span>
    )
  }
  const type = e.field.exists ? e.field.type : 'new field'
  const multiline = ['document', 'array'].includes(e.field.type) || e.text.includes('\n')
  const onKeyDown = async (ev: ReactKeyboardEvent) => {
    ev.stopPropagation() // the grid's navigation keys don't apply while typing
    if (ev.key === 'Escape') {
      ev.preventDefault()
      ce.cancel()
    } else if (ev.key === 'Enter' && !(multiline && ev.shiftKey)) {
      ev.preventDefault()
      await ce.commit()
    } else if (ev.key === 'Tab') {
      ev.preventDefault()
      if (await ce.commit()) onMove(ev.shiftKey ? -1 : 1)
    }
  }
  const common = {
    ref: inputRef,
    value: e.text,
    disabled: e.saving,
    'aria-label': `Edit ${ce.edit!.ref.path.join('.')}`,
    'aria-invalid': !!e.error,
    spellCheck: false,
    onChange: (ev: { target: { value: string } }) => ce.setText(ev.target.value),
    onKeyDown,
    onClick: (ev: { stopPropagation: () => void }) => ev.stopPropagation(),
    onDoubleClick: (ev: { stopPropagation: () => void }) => ev.stopPropagation(),
  }
  return (
    <span className={e.error ? 'cell-editor invalid' : 'cell-editor'}>
      <span className="cell-editor-row">
        {multiline ? <textarea {...common} rows={Math.min(8, e.text.split('\n').length + 1)} /> : <input {...common} />}
        <span className="type-badge" title="The type is kept. Use Edit document (JSON) to change it.">
          {type}
        </span>
        {e.saving && <Spinner small />}
      </span>
      {e.error && (
        <span className="cell-error" role="alert">
          {e.error}
        </span>
      )}
      {e.blocked && !e.error && (
        <span className="cell-error" role="alert">
          Unsaved edit — press Enter to save or Esc to discard.
        </span>
      )}
      {!e.error && !e.blocked && <span className="cell-hint">Enter saves · Esc cancels{e.field.type === 'string' ? '' : ' · shell syntax for documents and arrays'}</span>}
    </span>
  )
}

/** Moves the selection through a list of cells; returns the new ref or null at the ends. */
function step(list: CellRef[], cur: CellRef | null, delta: number): CellRef | null {
  if (list.length === 0) return null
  const i = cur ? list.findIndex((r) => sameRef(r, cur)) : -1
  const j = i < 0 ? (delta > 0 ? 0 : list.length - 1) : i + delta
  return j >= 0 && j < list.length ? list[j] : null
}

/** Keyboard handling shared by both views (focus on the grid, not editing). */
function useGridKeys(ce: CellEditing, cells: CellRef[], editableCells: CellRef[], arrows: (key: string) => CellRef | null | 'handled') {
  return (ev: ReactKeyboardEvent) => {
    if (ce.edit) return
    if (ev.key === 'Tab' && !ev.ctrlKey && !ev.metaKey && !ev.altKey) {
      const next = step(editableCells, ce.sel, ev.shiftKey ? -1 : 1)
      if (next) {
        ev.preventDefault()
        ce.select(next)
      }
      return
    }
    if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(ev.key) && !ev.metaKey && !ev.ctrlKey && !ev.altKey) {
      ev.preventDefault()
      if (!ce.sel && cells.length) {
        ce.select(cells[0])
        return
      }
      if (ev.key === 'Home') return cells.length && ce.select(cells[0])
      if (ev.key === 'End') return cells.length && ce.select(cells[cells.length - 1])
      const r = arrows(ev.key)
      if (r && r !== 'handled') ce.select(r)
    }
  }
}

function scrollIntoView(container: RefObject<HTMLElement | null>, sel: CellRef | null) {
  if (!sel || !container.current) return
  const el = container.current.querySelector<HTMLElement>(`[data-cell="${CSS.escape(`${sel.doc}:${pathKey(sel.path)}`)}"]`)
  el?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' })
}

// ---- table ------------------------------------------------------------------

export function TableView({
  ce,
  selected,
  onSelectDoc,
  onEditDocument,
  expand,
  gridRef,
  onContextMenu,
}: {
  ce: CellEditing
  selected: Set<string>
  onSelectDoc?: (id: string) => void
  onEditDocument?: (d: Doc) => void
  expand: boolean
  gridRef: RefObject<HTMLDivElement | null>
  onContextMenu?: (ref: CellRef, e: ReactMouseEvent) => void
}) {
  const cols = useMemo(() => tableColumns(ce.parsed, expand), [ce.parsed, expand])
  const cells = useMemo(() => ce.docs.flatMap((_, i) => cols.map((path) => ({ doc: i, path }))), [ce.docs, cols])
  const editableCells = useMemo(() => cells.filter((c) => ce.canEditCell(c)), [cells, ce])
  const colIndex = (p: FieldPath) => cols.findIndex((c) => pathKey(c) === pathKey(p))
  const onKeyDown = useGridKeys(ce, cells, editableCells, (key) => {
    const s = ce.sel!
    const c = Math.max(0, colIndex(s.path))
    if (key === 'ArrowLeft') return c > 0 ? { doc: s.doc, path: cols[c - 1] } : null
    if (key === 'ArrowRight') return c < cols.length - 1 ? { doc: s.doc, path: cols[c + 1] } : null
    if (key === 'ArrowUp') return s.doc > 0 ? { doc: s.doc - 1, path: cols[c] } : null
    if (key === 'ArrowDown') return s.doc < ce.docs.length - 1 ? { doc: s.doc + 1, path: cols[c] } : null
    return null
  })
  useEffect(() => scrollIntoView(gridRef, ce.sel), [ce.sel, gridRef])
  if (ce.docs.length === 0) return null
  const move = (delta: number) => {
    const next = step(editableCells, ce.edit?.ref ?? ce.sel, delta)
    if (next) ce.select(next)
    gridRef.current?.focus()
  }
  return (
    <div className="table-wrap" ref={gridRef} tabIndex={0} role="grid" aria-label="Documents (table)" onKeyDown={onKeyDown}>
      <table className="grid">
        <thead>
          <tr>
            {onSelectDoc && <th className="sel" />}
            <th className="rownum">#</th>
            {cols.map((c) => (
              <th key={pathKey(c)} title={c.join('.')}>
                {c.join('.')}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {ce.parsed.map((row, i) => {
            const d = ce.docs[i]
            return (
              <tr key={d.id || i} className={`${d.id && selected.has(d.id) ? 'selected' : ''}${i % 2 === 0 ? ' row-odd' : ' row-even'}`}>
                {onSelectDoc && (
                  <td className="sel">
                    {d.id && <input type="checkbox" checked={selected.has(d.id)} onChange={() => onSelectDoc(d.id)} aria-label={`Select row ${i + 1}`} />}
                  </td>
                )}
                <td className="rownum">
                  {i + 1}
                  {onEditDocument && d.id && (
                    <button className="row-edit" title="Edit document (JSON)" aria-label={`Edit document ${i + 1} as JSON`} onClick={() => onEditDocument(d)}>
                      {'{ }'}
                    </button>
                  )}
                </td>
                {cols.map((path) => {
                  const ref = { doc: i, path }
                  const v = getAt(row, path)
                  const cell = renderCell(v)
                  const isSel = sameRef(ce.sel, ref)
                  const isEdit = sameRef(ce.edit?.ref ?? null, ref)
                  return (
                    <td
                      key={pathKey(path)}
                      data-cell={`${i}:${pathKey(path)}`}
                      role="gridcell"
                      aria-selected={isSel}
                      className={`cell-${cell.kind}${isSel ? ' cell-selected' : ''}${isEdit ? ' cell-editing' : ''}${ce.canEditCell(ref) ? '' : ' cell-locked'}`}
                      title={isEdit ? undefined : cell.kind === 'object' || cell.kind === 'array' ? JSON.stringify(v, null, 2).slice(0, 2000) : `${cell.text} (${bsonType(v)})`}
                      onClick={() => ce.select(ref)}
                      onDoubleClick={() => ce.startEdit(ref)}
                      onContextMenu={(e) => onContextMenu?.(ref, e)}
                    >
                      {isEdit ? (
                        <CellEditor ce={ce} onMove={move} />
                      ) : (
                        <>
                          {v !== undefined && <TypeIcon type={bsonType(v)} />}
                          {cell.text}
                        </>
                      )}
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

// ---- tree -------------------------------------------------------------------

interface TreeRow {
  ref: CellRef
  depth: number
  label: string
  value: ReturnType<typeof getAt>
  container: boolean
  count: number
}

export function TreeView({
  ce,
  selected,
  onSelectDoc,
  onEditDocument,
  gridRef,
  pageOffset,
  onContextMenu,
}: {
  ce: CellEditing
  selected: Set<string>
  onSelectDoc?: (id: string) => void
  onEditDocument?: (d: Doc) => void
  gridRef: RefObject<HTMLDivElement | null>
  pageOffset: number
  onContextMenu?: (ref: CellRef, e: ReactMouseEvent) => void
}) {
  // Expanded nodes by "doc:path"; the first document starts expanded.
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(['0:[]']))
  const pageKey = ce.docs.map((d) => d.id).join('|')
  const [lastPage, setLastPage] = useState(pageKey)
  if (lastPage !== pageKey) {
    setLastPage(pageKey)
    setExpanded(new Set(['0:[]']))
  }
  const k = (r: CellRef) => `${r.doc}:${pathKey(r.path)}`
  const toggle = (r: CellRef, open?: boolean) =>
    setExpanded((s) => {
      const n = new Set(s)
      const want = open ?? !n.has(k(r))
      if (want) n.add(k(r))
      else n.delete(k(r))
      return n
    })

  const rows = useMemo(() => {
    const out: TreeRow[] = []
    ce.parsed.forEach((doc, i) => {
      const root: CellRef = { doc: i, path: [] }
      out.push({ ref: root, depth: 0, label: '', value: doc, container: true, count: Object.keys(doc).length })
      if (!expanded.has(k(root))) return
      const walk = (v: ReturnType<typeof getAt>, path: FieldPath, depth: number) => {
        for (const [key, child] of childrenOf(v)) {
          const p = [...path, key]
          const ref = { doc: i, path: p }
          const container = isContainer(child)
          out.push({ ref, depth, label: key, value: child, container, count: childrenOf(child).length })
          if (container && expanded.has(k(ref))) walk(child, p, depth + 1)
        }
      }
      walk(doc, [], 1)
    })
    return out
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ce.parsed, expanded])

  const cells = useMemo(() => rows.map((r) => r.ref), [rows])
  const editableCells = useMemo(() => cells.filter((c) => ce.canEditCell(c)), [cells, ce])
  const onKeyDown = useGridKeys(ce, cells, editableCells, (key) => {
    const s = ce.sel!
    const i = rows.findIndex((r) => sameRef(r.ref, s))
    const row = rows[i]
    if (key === 'ArrowUp') return i > 0 ? rows[i - 1].ref : null
    if (key === 'ArrowDown') return i < rows.length - 1 ? rows[i + 1].ref : null
    if (key === 'ArrowRight' && row?.container) {
      if (!expanded.has(k(row.ref))) toggle(row.ref, true)
      else if (rows[i + 1] && rows[i + 1].depth > row.depth) return rows[i + 1].ref
      return 'handled'
    }
    if (key === 'ArrowLeft' && row) {
      if (row.container && expanded.has(k(row.ref))) {
        toggle(row.ref, false)
        return 'handled'
      }
      if (row.ref.path.length > 0) return { doc: row.ref.doc, path: row.ref.path.slice(0, -1) }
    }
    return null
  })
  useEffect(() => scrollIntoView(gridRef, ce.sel), [ce.sel, gridRef])
  if (ce.docs.length === 0) return null
  const move = (delta: number) => {
    const next = step(editableCells, ce.edit?.ref ?? ce.sel, delta)
    if (next) ce.select(next)
    gridRef.current?.focus()
  }
  return (
    <div className="tree-wrap" ref={gridRef} tabIndex={0} role="treegrid" aria-label="Documents (tree)" onKeyDown={onKeyDown}>
      <table className="grid tree-grid">
        <thead>
          <tr>
            <th className="tree-key">Key</th>
            <th>Value</th>
            <th className="tree-type">Type</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, rowIndex) => {
            const d = ce.docs[r.ref.doc]
            const isRoot = r.ref.path.length === 0
            const isSel = sameRef(ce.sel, r.ref)
            const isEdit = sameRef(ce.edit?.ref ?? null, r.ref)
            const open = expanded.has(k(r.ref))
            const type = isRoot ? 'document' : bsonType(r.value)
            return (
              <tr
                key={k(r.ref)}
                data-cell={k(r.ref)}
                role="row"
                aria-level={r.depth + 1}
                aria-expanded={r.container ? open : undefined}
                aria-selected={isSel}
                className={`${isRoot ? 'tree-root' : ''}${isSel ? ' cell-selected' : ''}${isRoot && d.id && selected.has(d.id) ? ' selected' : ''}${rowIndex % 2 === 0 ? ' row-odd' : ' row-even'}${isRoot ? (r.ref.doc % 2 === 0 ? ' doc-odd' : ' doc-even') : ''}`}
                onClick={() => ce.select(r.ref)}
                onDoubleClick={() => (isRoot ? toggle(r.ref) : ce.startEdit(r.ref))}
                onContextMenu={(e) => onContextMenu?.(r.ref, e)}
              >
                <td className="tree-key" style={{ paddingLeft: 8 + r.depth * 16 }}>
                  {r.container ? (
                    <button
                      className={open ? 'caret open' : 'caret'}
                      aria-label={open ? 'Collapse' : 'Expand'}
                      onClick={(e) => {
                        e.stopPropagation()
                        toggle(r.ref)
                      }}
                      onDoubleClick={(e) => e.stopPropagation()}
                    />
                  ) : (
                    <span className="caret-spacer" />
                  )}
                  {isRoot ? (
                    <>
                      {onSelectDoc && d.id && (
                        <input
                          type="checkbox"
                          checked={selected.has(d.id)}
                          onClick={(e) => e.stopPropagation()}
                          onChange={() => onSelectDoc(d.id)}
                          aria-label={`Select document ${pageOffset + r.ref.doc + 1}`}
                        />
                      )}
                      <span className="doc-index">({pageOffset + r.ref.doc + 1})</span> <span className="mono">{d.id || '(no _id)'}</span>
                    </>
                  ) : (
                    <>
                      <span className="key-type">
                        <TypeIcon type={type} />
                      </span>
                      <span className={/^\d+$/.test(r.label) && r.ref.path.length > 0 && Array.isArray(getAt(ce.parsed[r.ref.doc], r.ref.path.slice(0, -1))) ? 'tree-index' : 'tree-field'}>
                        {r.label}
                      </span>
                    </>
                  )}
                </td>
                <td className={`tree-value cell-${renderCell(r.value).kind}${isEdit ? ' cell-editing' : ''}`} role="gridcell">
                  {isEdit ? (
                    <CellEditor ce={ce} onMove={move} />
                  ) : isRoot ? (
                    <span className="muted">
                      {'{'} {r.count} {r.count === 1 ? 'field' : 'fields'} {'}'}
                      {onEditDocument && d.id && (
                        <button
                          className="small ghost row-edit"
                          onClick={(e) => {
                            e.stopPropagation()
                            onEditDocument(d)
                          }}
                        >
                          Edit document (JSON)
                        </button>
                      )}
                    </span>
                  ) : r.container ? (
                    <span className="muted">{displayValue(r.value)}</span>
                  ) : (
                    displayValue(r.value, 300)
                  )}
                </td>
                <td className="tree-type muted small">
                  {isRoot && <TypeIcon type="document" />}
                  {type}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

export const ResultsNotice = memo(function ResultsNotice({ text, onDismiss }: { text: string; onDismiss: () => void }): ReactNode {
  return (
    <div className="info-banner small" role="status">
      {text}
      <button className="small ghost" onClick={onDismiss} aria-label="Dismiss">
        ×
      </button>
    </div>
  )
})
