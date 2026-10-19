import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { useState } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { findConflicts, setKeymapOverrides } from '../lib/shortcuts'
import type { Doc, FieldResult, FieldValue, Page } from '../lib/types'
import { setShortcutPlatform, useShortcut, useShortcutDispatcher } from '../state/shortcuts'
import type { FieldApi } from './DocViews'
import { ResultsView, type ResultsViewProps, type ViewMode } from './ResultsView'

// owner is a DBRef; managerId is a manual reference (an ObjectId).
const doc: Doc = {
  id: '1',
  text: '{\n  "_id": 1,\n  "name": "Ada",\n  "owner": DBRef("users", ObjectId("64b000000000000000000009"))\n}',
  json: JSON.stringify({
    _id: { $numberInt: '1' },
    name: 'Ada',
    age: { $numberInt: '36' },
    address: { city: 'London' },
    tags: ['a'],
    owner: { $ref: 'users', $id: { $oid: '64b000000000000000000009' } },
    managerId: { $oid: '64b00000000000000000000a' },
  }),
}
const page: Page = { documents: [doc], page: 0, pageSize: 50, hasMore: false, durationMs: 1 }
const fv = (type: string, text: string, expected: string): FieldValue => ({ exists: true, type, text, expected, editable: true })
const result = (d: Doc = doc): FieldResult => ({ document: d, previous: '"Ada"', current: '"Ada"', existed: true })

function api() {
  return {
    get: vi.fn(async (_d: Doc, path: string[]) => (path[0] === 'tags' ? fv('array', '["a"]', '["a"]') : fv('string', 'Ada', '"Ada"'))),
    update: vi.fn(async (_d: Doc, _u: Record<string, unknown>) => result()),
  }
}

type Extra = Partial<ResultsViewProps> & { view?: ViewMode; explain?: () => void }

function Harness({ fieldApi, view = 'table', explain, ...extra }: Extra & { fieldApi: FieldApi }) {
  const [v, setV] = useState<ViewMode>(view)
  useShortcutDispatcher()
  // The collection tab's own F8 (Explain) is registered too, like in the app.
  useShortcut('query.explain', () => explain?.())
  return <ResultsView page={page} loading={false} error={null} view={v} onViewChange={setV} onPage={vi.fn()} pageSize={50} fieldApi={fieldApi} confirm={async () => true} {...extra} />
}

const cell = (column: string) => {
  const grid = screen.getByRole('grid')
  const col = within(grid).getAllByRole('columnheader').map((h) => h.textContent).indexOf(column)
  return within(grid).getAllByRole('row')[1].querySelectorAll('td')[col] as HTMLElement
}
const press = (init: KeyboardEventInit & { code?: string }) =>
  act(() => {
    ;(document.activeElement ?? document.body).dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }))
  })
const focusCell = (column: string) => {
  fireEvent.click(cell(column))
  screen.getByRole('grid').focus()
}
const menuItem = (name: RegExp) => within(screen.getByRole('menu')).getByRole('menuitem', { name })

beforeEach(() => {
  setShortcutPlatform('other')
  setKeymapOverrides({})
})

describe('document and field context menu', () => {
  it('lists the document and field actions with their shortcuts', () => {
    render(<Harness fieldApi={api()} onEdit={vi.fn()} onDelete={vi.fn()} onInsert={vi.fn()} onRefreshDocument={vi.fn()} onBulkDialog={vi.fn()} onFollowReference={vi.fn()} />)
    fireEvent.contextMenu(cell('name'))
    const expected: [RegExp, string][] = [
      [/View document \(JSON\)/, 'F3'], [/Edit document \(JSON\)/, 'Ctrl+J'], [/Copy document/, 'Ctrl+Shift+C'], [/Refresh document/, 'Ctrl+Shift+R'],
      [/Copy value as text/, 'Ctrl+C'], [/Copy value \(exact/, 'Alt+Shift+C'], [/^Edit field/, 'Ctrl+E'], [/Add field/, 'Ctrl+K'], [/Rename field/, 'Shift+F2'],
      [/Remove field/, 'Del'], [/Insert document/, 'Ctrl+D'], [/Paste documents/, 'Ctrl+V'], [/Update dialog/, 'Ctrl+U'], [/Remove dialog/, 'F8'], [/Remove document/, 'Shift+Del'],
    ]
    for (const [name, key] of expected) {
      expect(menuItem(name)).toHaveTextContent(key)
      expect(menuItem(name)).toBeEnabled()
    }
    expect(screen.queryByRole('menuitem', { name: /Follow reference/ })).toBeNull() // only for references
  })

  it('a menu item and its shortcut run the same command', async () => {
    const a = api()
    a.update.mockResolvedValue({ ...result(), previous: '"Ada"', current: '' })
    render(<Harness fieldApi={a} />)
    fireEvent.contextMenu(cell('name'))
    fireEvent.click(menuItem(/Remove field/))
    await waitFor(() => expect(a.update).toHaveBeenCalledTimes(1))
    const viaMenu = a.update.mock.calls[0]
    focusCell('name')
    press({ key: 'Delete' })
    await waitFor(() => expect(a.update).toHaveBeenCalledTimes(2))
    expect(a.update.mock.calls[1]).toEqual(viaMenu)
    expect(viaMenu[1]).toMatchObject({ path: ['name'], unset: true, expected: '"Ada"' })
  })

  it('disables write actions on read-only connections but keeps viewing and copying', () => {
    render(<Harness fieldApi={api()} readOnly onEdit={vi.fn()} onDelete={vi.fn()} onInsert={vi.fn()} onBulkDialog={vi.fn()} />)
    fireEvent.contextMenu(cell('name'))
    for (const n of [/^Edit field/, /Add field/, /Rename field/, /Remove field/, /Insert document/, /Paste documents/, /Remove document/, /Edit document/]) {
      expect(menuItem(n)).toBeDisabled()
    }
    for (const n of [/View document/, /Copy document/, /Copy value as text/]) expect(menuItem(n)).toBeEnabled()
  })
})

describe('field shortcuts', () => {
  it('Ctrl+E edits the selected field; a single click still never does', async () => {
    const a = api()
    render(<Harness fieldApi={a} />)
    fireEvent.click(cell('name'))
    expect(screen.queryByRole('textbox')).toBeNull()
    screen.getByRole('grid').focus()
    press({ key: 'e', code: 'KeyE', ctrlKey: true })
    expect(await screen.findByRole('textbox', { name: 'Edit name' })).toHaveValue('Ada')
  })

  it('Ctrl+K adds a typed field to the document', async () => {
    const a = api()
    render(<Harness fieldApi={a} />)
    focusCell('name')
    press({ key: 'k', code: 'KeyK', ctrlKey: true })
    const dlg = await screen.findByRole('dialog', { name: /Add field to the document/ })
    fireEvent.change(within(dlg).getByLabelText('Field name'), { target: { value: 'score' } })
    fireEvent.change(within(dlg).getByLabelText('Value type'), { target: { value: 'int64' } })
    fireEvent.change(within(dlg).getByLabelText('Value'), { target: { value: '9007199254740993' } })
    fireEvent.click(within(dlg).getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(a.update).toHaveBeenCalled())
    expect(a.update.mock.calls[0][1]).toMatchObject({ path: ['score'], type: 'int64', text: '9007199254740993', expectedMissing: true })
  })

  it('Ctrl+K on an array appends a value', async () => {
    const a = api()
    render(<Harness fieldApi={a} />)
    focusCell('tags')
    press({ key: 'k', code: 'KeyK', ctrlKey: true })
    const dlg = await screen.findByRole('dialog', { name: /Add value to tags/ })
    fireEvent.change(within(dlg).getByLabelText('Value'), { target: { value: 'b' } })
    fireEvent.click(within(dlg).getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(a.update).toHaveBeenCalled())
    expect(a.update.mock.calls[0][1]).toMatchObject({ path: ['tags'], append: true, type: 'string', text: 'b', expected: '["a"]' })
  })

  it('Shift+F2 renames the field with $rename', async () => {
    const a = api()
    render(<Harness fieldApi={a} />)
    focusCell('name')
    press({ key: 'F2', shiftKey: true })
    const dlg = await screen.findByRole('dialog', { name: 'Rename name' })
    fireEvent.change(within(dlg).getByLabelText('New field name'), { target: { value: 'fullName' } })
    fireEvent.click(within(dlg).getByRole('button', { name: 'Rename' }))
    await waitFor(() => expect(a.update).toHaveBeenCalled())
    expect(a.update.mock.calls[0][1]).toMatchObject({ path: ['name'], renameTo: 'fullName', expected: '"Ada"' })
  })

  it('a failed add keeps the dialog open with the server error', async () => {
    const a = api()
    a.update.mockRejectedValue(new Error(JSON.stringify({ code: 'INVALID_INPUT', message: '"x" is not a whole number (this field is a 64-bit integer).' })))
    render(<Harness fieldApi={a} />)
    focusCell('name')
    press({ key: 'k', code: 'KeyK', ctrlKey: true })
    const dlg = await screen.findByRole('dialog', { name: /Add field/ })
    fireEvent.change(within(dlg).getByLabelText('Field name'), { target: { value: 'n' } })
    fireEvent.change(within(dlg).getByLabelText('Value type'), { target: { value: 'int64' } })
    fireEvent.change(within(dlg).getByLabelText('Value'), { target: { value: 'x' } })
    fireEvent.click(within(dlg).getByRole('button', { name: 'Add' }))
    expect(await within(dlg).findByText(/not a whole number/)).toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: /Add field/ })).toBeInTheDocument()
  })
})

describe('document shortcuts', () => {
  it('F3 views, Ctrl+J edits, Ctrl+Shift+R refreshes and Shift+Delete removes the document', async () => {
    const onEdit = vi.fn()
    const onDelete = vi.fn()
    const onRefreshDocument = vi.fn(async () => undefined)
    render(<Harness fieldApi={api()} onEdit={onEdit} onDelete={onDelete} onRefreshDocument={onRefreshDocument} />)
    focusCell('name')
    press({ key: 'F3' })
    const viewer = await screen.findByRole('dialog', { name: /Document 1/ })
    expect(within(viewer).getByLabelText('Document').textContent).toContain('"Ada"')
    fireEvent.click(within(viewer).getAllByRole('button', { name: 'Close' }).at(-1)!)
    screen.getByRole('grid').focus()
    press({ key: 'j', code: 'KeyJ', ctrlKey: true })
    await waitFor(() => expect(onEdit).toHaveBeenCalledWith(doc))
    press({ key: 'R', code: 'KeyR', ctrlKey: true, shiftKey: true })
    expect(onRefreshDocument).toHaveBeenCalledWith(doc)
    press({ key: 'Delete', shiftKey: true })
    expect(onDelete).toHaveBeenCalledWith([doc], { keyboard: true })
  })

  it('Shift+F7 follows a DBRef or an ObjectId, and only those', () => {
    const onFollowReference = vi.fn()
    render(<Harness fieldApi={api()} onFollowReference={onFollowReference} />)
    focusCell('owner')
    press({ key: 'F7', shiftKey: true })
    expect(onFollowReference).toHaveBeenLastCalledWith({ coll: 'users', db: undefined, idJSON: '{"$oid":"64b000000000000000000009"}' })
    focusCell('managerId')
    press({ key: 'F7', shiftKey: true })
    expect(onFollowReference).toHaveBeenLastCalledWith({ idJSON: '{"$oid":"64b00000000000000000000a"}' }) // the tab asks which collection
    fireEvent.contextMenu(cell('managerId'))
    expect(menuItem(/Follow reference/)).toHaveTextContent('Shift+F7')
    fireEvent.keyDown(window, { key: 'Escape' })
    focusCell('name')
    press({ key: 'F7', shiftKey: true })
    expect(onFollowReference).toHaveBeenCalledTimes(2)
  })

  it('F8 opens the Remove dialog in results but stays Explain elsewhere; Ctrl+U opens the Update dialog', () => {
    const onBulkDialog = vi.fn()
    const explain = vi.fn()
    render(<Harness fieldApi={api()} onBulkDialog={onBulkDialog} explain={explain} />)
    focusCell('name')
    press({ key: 'F8' })
    expect(onBulkDialog).toHaveBeenLastCalledWith('remove', ['1'])
    expect(explain).not.toHaveBeenCalled()
    press({ key: 'u', code: 'KeyU', ctrlKey: true })
    expect(onBulkDialog).toHaveBeenLastCalledWith('update', ['1'])
    ;(document.activeElement as HTMLElement).blur()
    press({ key: 'F8' })
    expect(explain).toHaveBeenCalledTimes(1)
  })

  it('field shortcuts do nothing without a selected field, and never while typing in a field', async () => {
    const a = api()
    render(<Harness fieldApi={a} />)
    screen.getByRole('grid').focus()
    press({ key: 'Delete' })
    press({ key: 'e', code: 'KeyE', ctrlKey: true })
    expect(a.update).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox')).toBeNull()
    fireEvent.doubleClick(cell('name'))
    const input = await screen.findByRole('textbox', { name: 'Edit name' })
    press({ key: 'Delete' }) // deletes a character in the editor, not the field
    press({ key: 'c', code: 'KeyC', ctrlKey: true })
    expect(a.update).not.toHaveBeenCalled()
    expect(input).toBeInTheDocument()
  })
})

describe('customized keymap', () => {
  it('a customized binding replaces the default for the command', async () => {
    setKeymapOverrides({ other: { 'field.edit': ['Ctrl+Shift+E'] } })
    render(<Harness fieldApi={api()} />)
    focusCell('name')
    press({ key: 'e', code: 'KeyE', ctrlKey: true })
    expect(screen.queryByRole('textbox')).toBeNull()
    press({ key: 'E', code: 'KeyE', ctrlKey: true, shiftKey: true })
    expect(await screen.findByRole('textbox', { name: 'Edit name' })).toBeInTheDocument()
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Escape' })
    fireEvent.contextMenu(cell('name'))
    expect(menuItem(/^Edit field/)).toHaveTextContent('Ctrl+Shift+E') // menus show the custom key
  })

  it('reports conflicts between customized shortcuts', () => {
    expect(findConflicts('other')).toEqual([])
    expect(findConflicts('mac')).toEqual([])
    setKeymapOverrides({ other: { 'field.add': ['Ctrl+E'] } })
    expect(findConflicts('other')).toEqual([{ binding: 'Ctrl+E', ids: ['field.add', 'field.edit'].sort() }].map((c) => ({ ...c, ids: expect.arrayContaining(c.ids) })))
  })
})
