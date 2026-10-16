import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { useState } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SHORTCUT_BY_ID, keysFor, normalizeBinding, setKeymapOverrides } from '../lib/shortcuts'
import type { Settings } from '../lib/types'
import { setShortcutPlatform } from '../state/shortcuts'

const backend = vi.hoisted(() => ({ exportTextFile: vi.fn(), importTextFile: vi.fn(), previewBulkWrite: vi.fn(), executeBulkWrite: vi.fn() }))
const app = vi.hoisted(() => ({ settings: {} as Partial<Settings>, saveSettings: vi.fn(), toast: vi.fn(), toastError: vi.fn() }))
vi.mock('../lib/backend', () => ({ backend }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { BulkWriteDialog } from './BulkWriteDialog'
import { KeymapSettings, parseKeymapFile } from './KeymapSettings'

const rerender = { current: () => {} }
function Harness() {
  const [, force] = useState(0)
  rerender.current = () => force((n) => n + 1)
  return <KeymapSettings />
}

const row = (id: string) => document.querySelector(`tr[data-action="${id}"]`) as HTMLElement

beforeEach(() => {
  setShortcutPlatform('other')
  app.settings = {}
  setKeymapOverrides({})
  vi.clearAllMocks()
  app.saveSettings = vi.fn(async (next: Settings) => {
    app.settings = next
    setKeymapOverrides(next.keymap)
    rerender.current()
  })
})

describe('Studio 3T document and field shortcuts', () => {
  // https://studio3t.com/knowledge-base/3t-build/desktop-ide/hotkeys/ (Collection Tab › Documents / Fields)
  const documented: [string, string, string][] = [
    ['doc.copy', 'Ctrl+Shift+C', 'Shift+Meta+C'],
    ['doc.view', 'F3', 'F3'],
    ['doc.insert', 'Ctrl+D', 'Meta+D'],
    ['doc.editJson', 'Ctrl+J', 'Meta+J'],
    ['doc.delete', 'Shift+Delete', 'Shift+Delete'], // macOS Shift+Fn+Del is the forward-delete key
    ['doc.refresh', 'Ctrl+Shift+R', 'Shift+Meta+R'],
    ['doc.followRef', 'Shift+F7', 'Shift+F7'],
    ['cell.copy', 'Ctrl+C', 'Meta+C'],
    ['cell.copyValue', 'Alt+Shift+C', 'Alt+Shift+C'],
    ['field.add', 'Ctrl+K', 'Meta+K'],
    ['field.edit', 'Ctrl+E', 'Meta+E'],
    ['field.rename', 'Shift+F2', 'Shift+F2'],
    ['field.remove', 'Delete', 'Delete'], // macOS Fn+Del
    ['dialog.update', 'Ctrl+U', 'Meta+U'],
    ['dialog.remove', 'F8', 'F8'],
  ]
  it('Paste documents uses Ctrl+V / ⌘V but is not in the documented Documents table', () => {
    const def = SHORTCUT_BY_ID['docs.paste']
    expect(def.source).toBe('default')
    expect(keysFor(def, 'other')).toEqual(['Mod+V'])
  })
  it.each(documented)('%s is %s (Windows/Linux) and %s (macOS)', (id, other, mac) => {
    const def = SHORTCUT_BY_ID[id]
    expect(def.source).toBe('studio3t')
    expect(keysFor(def, 'other').map((k) => normalizeBinding(k, 'other'))).toEqual([other])
    expect(keysFor(def, 'mac').map((k) => normalizeBinding(k, 'mac'))).toEqual([mac])
  })
})

describe('Preferences › Keymap', () => {
  it('adds a shortcut by recording keys, removes it and resets', async () => {
    render(<Harness />)
    fireEvent.click(within(row('field.edit')).getByRole('button', { name: '+ Add' }))
    fireEvent.keyDown(within(row('field.edit')).getByLabelText(/Record a shortcut/), { key: 'E', code: 'KeyE', ctrlKey: true, shiftKey: true })
    await waitFor(() => expect(app.saveSettings).toHaveBeenCalled())
    expect(app.settings.keymap?.other?.['field.edit']).toEqual(['Mod+E', 'Mod+Shift+E'])
    expect(within(row('field.edit')).getByText('custom')).toBeInTheDocument()
    fireEvent.click(within(row('field.edit')).getByRole('button', { name: /Remove Ctrl\+E from Edit field/ }))
    await waitFor(() => expect(app.settings.keymap?.other?.['field.edit']).toEqual(['Mod+Shift+E']))
    fireEvent.click(within(row('field.edit')).getByRole('button', { name: 'Reset' }))
    await waitFor(() => expect(app.settings.keymap?.other?.['field.edit']).toBeUndefined())
    expect(keysFor(SHORTCUT_BY_ID['field.edit'], 'other')).toEqual(['Mod+E'])
  })

  it('warns about a conflict and can reassign the key', async () => {
    render(<Harness />)
    fireEvent.click(within(row('field.edit')).getByRole('button', { name: '+ Add' }))
    fireEvent.keyDown(within(row('field.edit')).getByLabelText(/Record a shortcut/), { key: 'k', code: 'KeyK', ctrlKey: true })
    const alert = within(row('field.edit')).getByRole('alert')
    expect(alert).toHaveTextContent('Ctrl+K is used by Add field / value')
    expect(app.saveSettings).not.toHaveBeenCalled()
    fireEvent.click(within(alert).getByRole('button', { name: 'Reassign' }))
    await waitFor(() => expect(app.saveSettings).toHaveBeenCalled())
    expect(app.settings.keymap?.other?.['field.edit']).toEqual(['Mod+E', 'Mod+K'])
    expect(app.settings.keymap?.other?.['field.add']).toEqual([])
  })

  it('refuses keys that would type text or override text editing', () => {
    render(<Harness />)
    fireEvent.click(within(row('conn.manager')).getByRole('button', { name: '+ Add' }))
    fireEvent.keyDown(within(row('conn.manager')).getByLabelText(/Record a shortcut/), { key: 'x', code: 'KeyX' })
    expect(within(row('conn.manager')).getByText(/would type text/)).toBeInTheDocument()
    fireEvent.click(within(row('conn.manager')).getByRole('button', { name: '+ Add' }))
    fireEvent.keyDown(within(row('conn.manager')).getByLabelText(/Record a shortcut/), { key: 'c', code: 'KeyC', ctrlKey: true })
    expect(within(row('conn.manager')).getByText(/standard text-editing shortcut/)).toBeInTheDocument()
    expect(app.saveSettings).not.toHaveBeenCalled()
  })

  it('caps actions at three shortcuts and does not let built-in keys be changed', async () => {
    app.settings = { keymap: { other: { 'field.edit': ['Mod+E', 'F9', 'F10'] } } }
    setKeymapOverrides(app.settings.keymap)
    render(<Harness />)
    expect(within(row('field.edit')).getByRole('button', { name: '+ Add' })).toBeDisabled()
    expect(within(row('cell.save')).queryByRole('button', { name: '+ Add' })).toBeNull()
    expect(within(row('cell.save')).getByText(/not customizable/)).toBeInTheDocument()
  })

  it('exports and imports the keymap', async () => {
    app.settings = { keymap: { other: { 'doc.copy': ['Mod+Alt+C'] } } }
    setKeymapOverrides(app.settings.keymap)
    backend.exportTextFile.mockResolvedValue('/tmp/k.json')
    render(<Harness />)
    fireEvent.click(screen.getByRole('button', { name: 'Export…' }))
    await waitFor(() => expect(backend.exportTextFile).toHaveBeenCalled())
    const exported = JSON.parse(backend.exportTextFile.mock.calls[0][2])
    expect(exported).toEqual({ format: 'mango3s-keymap', version: 1, keymap: { other: { 'doc.copy': ['Mod+Alt+C'] } } })

    backend.importTextFile.mockResolvedValue(JSON.stringify({ format: 'mango3s-keymap', version: 1, keymap: { mac: { 'field.rename': ['F2'] }, other: { 'field.rename': ['F2'], 'no.such': ['F1'], 'cell.save': ['F4'] } } }))
    fireEvent.click(screen.getByRole('button', { name: 'Import…' }))
    await waitFor(() => expect(app.saveSettings).toHaveBeenCalled())
    expect(app.settings.keymap).toEqual({ mac: { 'field.rename': ['F2'] }, other: { 'field.rename': ['F2'] } })
    expect(app.toast).toHaveBeenCalledWith(expect.objectContaining({ message: '2 unknown action(s) were ignored.' }))
    expect(() => parseKeymapFile('{"format":"other"}')).toThrow(/not a Mango3S keymap/)
    expect(() => parseKeymapFile('nope')).toThrow(/not JSON/)
  })
})

describe('Update and Remove dialogs', () => {
  it('preview first, then the typed target, then run with the previewed count', async () => {
    backend.previewBulkWrite.mockResolvedValue({ affected: 2, sampleIds: ['1', '2'], confirmText: 'shop.orders', shell: 'db.getCollection("orders").deleteMany({...})' })
    backend.executeBulkWrite.mockResolvedValue({ matched: 2, modified: 0, deleted: 2, backupPath: '/b.ndjson' })
    const onDone = vi.fn()
    render(<BulkWriteDialog connId="c" db="shop" coll="orders" kind="remove" selectedIds={['1', '2']} queryFilter="{}" onDone={onDone} onClose={vi.fn()} />)
    const run = screen.getByRole('button', { name: /^Remove/ })
    expect(run).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: /Preview/ }))
    expect(await screen.findByText(/will be deleted/)).toBeInTheDocument()
    expect(backend.previewBulkWrite).toHaveBeenCalledWith(expect.objectContaining({ kind: 'delete', scope: 'selected', ids: ['1', '2'] }))
    expect(screen.getByRole('button', { name: /^Remove 2 documents/ })).toBeDisabled()
    fireEvent.change(screen.getByLabelText(/type shop.orders to confirm/), { target: { value: 'shop.orders' } })
    fireEvent.click(screen.getByRole('button', { name: /^Remove 2 documents/ }))
    await waitFor(() => expect(onDone).toHaveBeenCalled())
    expect(backend.executeBulkWrite).toHaveBeenCalledWith(expect.objectContaining({ kind: 'delete' }), 'shop.orders', 2)
  })

  it('a changed count means preview again; nothing runs', async () => {
    backend.previewBulkWrite.mockResolvedValue({ affected: 3, sampleIds: [], confirmText: 'shop.orders', shell: 'x' })
    backend.executeBulkWrite.mockRejectedValue(new Error(JSON.stringify({ code: 'CONFLICT', message: '4 documents match now, but the preview showed 3. Preview again before running it.' })))
    const onDone = vi.fn()
    render(<BulkWriteDialog connId="c" db="shop" coll="orders" kind="update" selectedIds={[]} queryFilter='{ s: "A" }' onDone={onDone} onClose={vi.fn()} />)
    expect(screen.getByRole('radio', { name: /Selected \(none\)/ })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: /Preview/ }))
    fireEvent.change(await screen.findByLabelText(/type shop.orders to confirm/), { target: { value: 'shop.orders' } })
    fireEvent.click(screen.getByRole('button', { name: /^Update 3 documents/ }))
    expect(await screen.findByText(/Preview again/)).toBeInTheDocument()
    expect(onDone).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: /^Update/ })).toBeDisabled()
    expect(backend.executeBulkWrite.mock.calls[0][0]).toMatchObject({ scope: 'query', filter: '{ s: "A" }', kind: 'update' })
  })
})
