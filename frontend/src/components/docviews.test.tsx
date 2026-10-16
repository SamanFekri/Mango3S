import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { useState } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Doc, FieldResult, FieldValue, Page } from '../lib/types'
import { setShortcutPlatform, useShortcutDispatcher } from '../state/shortcuts'
import type { FieldApi } from './DocViews'

/** The field name (or array index) shown in a tree row's key cell. */
const keyOf = (r: HTMLElement) => r.querySelector('.tree-field, .tree-index')?.textContent ?? ''
import { ResultsView, type ViewMode } from './ResultsView'

// Documents arrive as canonical Extended JSON (exact BSON types).
const ada = (age: number, city = 'London', name = 'Ada'): Doc => ({
  id: 'ObjectId("64b000000000000000000001")',
  text: `{\n  "_id": ObjectId("64b000000000000000000001"),\n  "name": "${name}",\n  "age": ${age}\n}`,
  json: JSON.stringify({ _id: { $oid: '64b000000000000000000001' }, name, age: { $numberInt: String(age) }, address: { city }, tags: ['a', 'b'] }),
})
const grace: Doc = {
  id: 'ObjectId("64b000000000000000000002")',
  text: '{ "_id": ObjectId("64b000000000000000000002"), "name": "Grace" }',
  json: JSON.stringify({ _id: { $oid: '64b000000000000000000002' }, name: 'Grace', age: { $numberInt: '45' } }),
}

const page = (docs: Doc[]): Page => ({ documents: docs, page: 0, pageSize: 50, hasMore: false, durationMs: 1 })

const fields: Record<string, FieldValue> = {
  '["age"]': { exists: true, type: 'int32', text: '36', expected: '36', editable: true },
  '["name"]': { exists: true, type: 'string', text: 'Ada', expected: '"Ada"', editable: true },
  '["address","city"]': { exists: true, type: 'string', text: 'London', expected: '"London"', editable: true },
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function makeApi(): { get: any; update: any } & FieldApi {
  return {
    get: vi.fn(async (_doc: Doc, path: string[]) => fields[JSON.stringify(path)] ?? { exists: true, type: 'string', text: 'x', expected: '"x"', editable: true }),
    update: vi.fn(),
  }
}

/** Holds the page like CollectionView does, replacing documents the server returns. */
function Harness({ api, view = 'table', readOnly, onChanged }: { api: FieldApi; view?: ViewMode; readOnly?: boolean; onChanged?: (d: Doc) => void }) {
  const [p, setP] = useState(page([ada(36), grace]))
  const [v, setV] = useState<ViewMode>(view)
  useShortcutDispatcher()
  return (
    <ResultsView
      page={p}
      loading={false}
      error={null}
      view={v}
      onViewChange={setV}
      onPage={vi.fn()}
      pageSize={50}
      fieldApi={api}
      readOnly={readOnly}
      confirm={async () => true}
      onDocumentChanged={(d) => {
        onChanged?.(d)
        setP((cur) => ({ ...cur, documents: cur.documents.map((x) => (x.id === d.id ? d : x)) }))
      }}
    />
  )
}

/** The table cell for a document row (1-based) and column header. */
function cell(row: number, column: string): HTMLElement {
  const grid = screen.getByRole('grid')
  const headers = within(grid).getAllByRole('columnheader').map((h) => h.textContent)
  const col = headers.indexOf(column)
  const tr = within(grid).getAllByRole('row')[row]
  return tr.querySelectorAll('td')[col] as HTMLElement
}

const ok = (doc: Doc, previous: string, current: string): FieldResult => ({ document: doc, previous, current, existed: true })
const appError = (code: string, message: string) => new Error(JSON.stringify({ code, message }))

beforeEach(() => {
  vi.clearAllMocks()
  setShortcutPlatform('other')
})

describe('table cell editing', () => {
  it('a single click selects a cell and never starts editing', async () => {
    const api = makeApi()
    render(<Harness api={api} />)
    const age = cell(1, 'age')
    fireEvent.click(age)
    fireEvent.click(age)
    expect(age).toHaveAttribute('aria-selected', 'true')
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(api.get).not.toHaveBeenCalled()
    // Clicking another cell moves the selection, still without editing.
    fireEvent.click(cell(1, 'name'))
    expect(cell(1, 'name')).toHaveAttribute('aria-selected', 'true')
    expect(age).toHaveAttribute('aria-selected', 'false')
    expect(screen.queryByRole('textbox')).toBeNull()
  })

  it('a double click edits only that field', async () => {
    const api = makeApi()
    render(<Harness api={api} />)
    fireEvent.doubleClick(cell(1, 'age'))
    const input = await screen.findByRole('textbox', { name: 'Edit age' })
    expect(input).toHaveValue('36')
    expect(screen.getAllByRole('textbox')).toHaveLength(1)
    expect(api.get).toHaveBeenCalledTimes(1)
    expect(api.get.mock.calls[0][1]).toEqual(['age'])
    expect(cell(1, 'name')).toHaveTextContent('Ada')
    expect(cell(2, 'age')).toHaveTextContent('45')
  })

  it('Enter saves a valid change with a field-level update and shows the server value', async () => {
    const api = makeApi()
    const changed = vi.fn()
    api.update.mockResolvedValue(ok(ada(37), '36', '37'))
    render(<Harness api={api} onChanged={changed} />)
    fireEvent.doubleClick(cell(1, 'age'))
    const input = await screen.findByRole('textbox', { name: 'Edit age' })
    fireEvent.change(input, { target: { value: '37' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => expect(screen.queryByRole('textbox')).toBeNull())
    expect(api.update).toHaveBeenCalledTimes(1)
    const [doc, u] = api.update.mock.calls[0]
    expect(doc.id).toBe(ada(36).id)
    expect(u).toEqual({ path: ['age'], type: 'int32', text: '37', expected: '36', expectedMissing: false })
    expect(changed).toHaveBeenCalledWith(ada(37))
    expect(cell(1, 'age')).toHaveTextContent('37')
  })

  it('Escape cancels without writing', async () => {
    const api = makeApi()
    render(<Harness api={api} />)
    fireEvent.doubleClick(cell(1, 'age'))
    const input = await screen.findByRole('textbox', { name: 'Edit age' })
    fireEvent.change(input, { target: { value: '99' } })
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(api.update).not.toHaveBeenCalled()
    expect(cell(1, 'age')).toHaveTextContent('36')
  })

  it('an invalid value shows a useful error inline and keeps the text', async () => {
    const api = makeApi()
    api.update.mockRejectedValue(appError('INVALID_INPUT', '"abc" is not a whole number (this field is a 32-bit integer).'))
    render(<Harness api={api} />)
    fireEvent.doubleClick(cell(1, 'age'))
    const input = await screen.findByRole('textbox', { name: 'Edit age' })
    fireEvent.change(input, { target: { value: 'abc' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(await screen.findByRole('alert')).toHaveTextContent('is not a whole number (this field is a 32-bit integer)')
    expect(screen.getByRole('textbox', { name: 'Edit age' })).toHaveValue('abc')
    expect(screen.getByRole('textbox', { name: 'Edit age' })).toHaveAttribute('aria-invalid', 'true')
  })

  it('a failed database update never looks successful', async () => {
    const api = makeApi()
    const changed = vi.fn()
    api.update.mockRejectedValue(appError('CONFLICT', 'This field changed since you loaded the page (it is now 40). Reload and try again.'))
    render(<Harness api={api} onChanged={changed} />)
    fireEvent.doubleClick(cell(1, 'age'))
    const input = await screen.findByRole('textbox', { name: 'Edit age' })
    fireEvent.change(input, { target: { value: '37' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(await screen.findByRole('alert')).toHaveTextContent('changed since you loaded the page')
    expect(changed).not.toHaveBeenCalled()
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Escape' })
    expect(cell(1, 'age')).toHaveTextContent('36') // the old value, not the attempted one
  })

  it('leaves unrelated fields and documents unchanged', async () => {
    const api = makeApi()
    api.update.mockResolvedValue(ok(ada(37), '36', '37'))
    render(<Harness api={api} />)
    fireEvent.doubleClick(cell(1, 'age'))
    const input = await screen.findByRole('textbox', { name: 'Edit age' })
    fireEvent.change(input, { target: { value: '37' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => expect(cell(1, 'age')).toHaveTextContent('37'))
    // Only the one path was sent; nothing that could replace the document.
    expect(api.update).toHaveBeenCalledTimes(1)
    expect(Object.keys(api.update.mock.calls[0][1])).not.toContain('document')
    expect(api.update.mock.calls[0][1].path).toEqual(['age'])
    expect(cell(1, 'name')).toHaveTextContent('Ada')
    expect(cell(1, 'address')).toHaveTextContent('{ 1 field }')
    expect(cell(2, 'name')).toHaveTextContent('Grace')
    expect(cell(2, 'age')).toHaveTextContent('45')
  })

  it('does not drop an unsaved edit when another cell is clicked', async () => {
    const api = makeApi()
    render(<Harness api={api} />)
    fireEvent.doubleClick(cell(1, 'age'))
    const input = await screen.findByRole('textbox', { name: 'Edit age' })
    fireEvent.change(input, { target: { value: '50' } })
    fireEvent.click(cell(1, 'name'))
    expect(screen.getByRole('textbox', { name: 'Edit age' })).toHaveValue('50')
    expect(screen.getByRole('alert')).toHaveTextContent('Unsaved edit')
    expect(api.update).not.toHaveBeenCalled()
  })

  it('Tab saves and moves to the next editable cell; Shift+Tab moves back', async () => {
    const api = makeApi()
    api.update.mockResolvedValue(ok(ada(37), '36', '37'))
    render(<Harness api={api} />)
    fireEvent.doubleClick(cell(1, 'name'))
    const input = await screen.findByRole('textbox', { name: 'Edit name' })
    fireEvent.keyDown(input, { key: 'Tab' }) // unchanged: no write, just move
    await waitFor(() => expect(cell(1, 'age')).toHaveAttribute('aria-selected', 'true'))
    expect(api.update).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox')).toBeNull() // moving never starts editing
    // Navigation keys on the grid itself skip the read-only _id column.
    const grid = screen.getByRole('grid')
    fireEvent.keyDown(grid, { key: 'Tab', shiftKey: true })
    expect(cell(1, 'name')).toHaveAttribute('aria-selected', 'true')
    fireEvent.keyDown(grid, { key: 'Tab', shiftKey: true })
    expect(cell(1, 'name')).toHaveAttribute('aria-selected', 'true') // _id is skipped
    fireEvent.keyDown(grid, { key: 'Enter' })
    expect(screen.queryByRole('textbox')).toBeNull() // Enter does not edit: double-click only
  })

  it('respects read-only connections and the immutable _id', async () => {
    const api = makeApi()
    const { unmount } = render(<Harness api={api} readOnly />)
    fireEvent.doubleClick(cell(1, 'age'))
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.getByRole('status')).toHaveTextContent('read-only')
    expect(api.get).not.toHaveBeenCalled()
    unmount()
    render(<Harness api={api} />)
    fireEvent.doubleClick(cell(1, '_id'))
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.getByRole('status')).toHaveTextContent('_id cannot be changed')
  })

  it('undoes the last field edit with the previous exact value', async () => {
    const api = makeApi()
    api.update.mockResolvedValueOnce(ok(ada(37), '36', '37')).mockResolvedValueOnce(ok(ada(36), '37', '36'))
    render(<Harness api={api} />)
    fireEvent.doubleClick(cell(1, 'age'))
    const input = await screen.findByRole('textbox', { name: 'Edit age' })
    fireEvent.change(input, { target: { value: '37' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => expect(cell(1, 'age')).toHaveTextContent('37'))
    const grid = screen.getByRole('grid')
    grid.focus()
    await act(async () => {
      fireEvent.keyDown(grid, { key: 'z', code: 'KeyZ', ctrlKey: true })
    })
    await waitFor(() => expect(api.update).toHaveBeenCalledTimes(2))
    expect(api.update.mock.calls[1][1]).toMatchObject({ path: ['age'], value: '36', expected: '37', expectedMissing: false })
    await waitFor(() => expect(cell(1, 'age')).toHaveTextContent('36'))
  })
})

describe('tree view', () => {
  it('is the view that shows nested fields, editable by double-click only', async () => {
    const api = makeApi()
    api.update.mockResolvedValue(ok(ada(36, 'Paris'), '"London"', '"Paris"'))
    render(<Harness api={api} view="tree" />)
    const tree = screen.getByRole('treegrid')
    // First document is expanded; expand the embedded address document.
    const addressRow = within(tree).getAllByRole('row').find((r) => keyOf(r) === 'address')!
    fireEvent.click(within(addressRow).getByRole('button', { name: 'Expand' }))
    const cityRow = within(tree).getAllByRole('row').find((r) => keyOf(r) === 'city')!
    expect(cityRow).toHaveTextContent('London')
    expect(cityRow).toHaveTextContent('string')
    fireEvent.click(cityRow)
    expect(screen.queryByRole('textbox')).toBeNull()
    fireEvent.doubleClick(cityRow)
    const input = await screen.findByRole('textbox', { name: 'Edit address.city' })
    fireEvent.change(input, { target: { value: 'Paris' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => expect(api.update).toHaveBeenCalled())
    expect(api.update.mock.calls[0][1]).toMatchObject({ path: ['address', 'city'], type: 'string', text: 'Paris', expected: '"London"' })
    await waitFor(() => expect(within(screen.getByRole('treegrid')).getAllByRole('row').find((r) => keyOf(r) === 'city')).toHaveTextContent('Paris'))
  })

  it('shows exact types from canonical JSON and array indexes', () => {
    render(<Harness api={makeApi()} view="tree" />)
    const rows = within(screen.getByRole('treegrid')).getAllByRole('row')
    expect(rows.find((r) => keyOf(r) === 'age')).toHaveTextContent('int32')
    const tags = rows.find((r) => keyOf(r) === 'tags')!
    expect(tags).toHaveTextContent('array')
    fireEvent.click(within(tags).getByRole('button', { name: 'Expand' }))
    const after = within(screen.getByRole('treegrid')).getAllByRole('row')
    expect(after.some((r) => keyOf(r) === '0' && !!r.textContent?.includes('a'))).toBe(true)
  })
})

describe('readability', () => {
  it('shows a type icon per value and stripes rows (odd darker, even lighter)', () => {
    const { unmount } = render(<Harness api={makeApi()} />)
    expect(within(cell(1, 'age')).getByRole('img', { name: 'Int32' })).toHaveTextContent('32')
    expect(within(cell(1, 'name')).getByRole('img', { name: 'String' })).toBeInTheDocument()
    expect(within(cell(1, '_id')).getByRole('img', { name: 'ObjectId' })).toBeInTheDocument()
    expect(within(cell(1, 'tags')).getByRole('img', { name: 'Array' })).toBeInTheDocument()
    const rows = within(screen.getByRole('grid')).getAllByRole('row')
    expect(rows[1]).toHaveClass('row-odd')
    expect(rows[2]).toHaveClass('row-even')
    unmount()
    render(<Harness api={makeApi()} view="tree" />)
    const tree = screen.getByRole('treegrid')
    const address = within(tree).getAllByRole('row').find((r) => r.textContent?.includes('address'))!
    // The type icon is in the key cell, before the field name.
    const keyCell = address.querySelector('td.tree-key')!
    const icon = within(keyCell as HTMLElement).getByRole('img', { name: 'Document' })
    expect(icon.compareDocumentPosition(within(keyCell as HTMLElement).getByText('address')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(address.querySelector('td.tree-value [role="img"]')).toBeNull()
    // Document rows alternate by document as well.
    const roots = within(tree).getAllByRole('row').filter((r) => r.classList.contains('tree-root'))
    expect(roots[0]).toHaveClass('doc-odd')
    expect(roots[1]).toHaveClass('doc-even')
    // Every tree row alternates, not just every document.
    const body = within(tree).getAllByRole('row').slice(1)
    body.forEach((r, i) => expect(r).toHaveClass(i % 2 === 0 ? 'row-odd' : 'row-even'))
  })
})
