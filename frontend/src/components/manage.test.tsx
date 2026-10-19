import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SessionInfo } from '../lib/types'

const backend = vi.hoisted(() => ({
  listCollections: vi.fn(),
  createCollection: vi.fn(),
  createCollectionWithOptions: vi.fn(),
  createView: vi.fn(),
  collectionDetails: vi.fn(),
  listIndexDetails: vi.fn(),
  sampleFields: vi.fn(),
  createIndex: vi.fn(),
  dropIndex: vi.fn(),
  modifyIndex: vi.fn(),
  getValidation: vi.fn(),
  checkValidator: vi.fn(),
  setValidation: vi.fn(),
  renameCollection: vi.fn(),
  startCopy: vi.fn(),
}))
const conn: SessionInfo = { id: 'c1', name: 'Prod', readOnly: false, serverVersion: '8.0.1', topology: 'standalone', connectedAt: '' }
const app = vi.hoisted(() => ({
  toast: vi.fn(),
  confirm: vi.fn(),
  prompt: vi.fn(),
  jobs: [],
  sessions: [] as SessionInfo[],
}))

vi.mock('../lib/backend', () => ({ backend, newOpId: (p: string) => `${p}-1` }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { addIndexKey, CollectionInfoDialog, CopyDialog, CreateCollectionDialog, indexKeyFields, RenameCollectionDialog } from './ManageDialogs'

beforeEach(() => {
  vi.clearAllMocks()
  app.sessions = [conn, { ...conn, id: 'c2', name: 'Staging' }, { ...conn, id: 'c3', name: 'Ro', readOnly: true }]
  backend.listCollections.mockResolvedValue([{ name: 'people', type: 'collection' }])
  backend.collectionDetails.mockResolvedValue({ type: 'collection', options: '{}', readOnly: false, stats: {}, count: 2, size: 100, storageSize: 4096, totalIndexSize: 4096, avgObjSize: 50, capped: false, sharded: false, shell: 'db.createCollection("people", {})' })
  backend.listIndexDetails.mockResolvedValue([
    { name: '_id_', keys: '{_id: 1}', spec: '', unique: false, sparse: false, hidden: false, size: 4096, accesses: 3 },
    { name: 'email_1', keys: '{email: 1}', spec: '', unique: true, sparse: false, hidden: false, ttl: 60, size: 4096, accesses: 0 },
  ])
  backend.sampleFields.mockResolvedValue(['_id', 'email', 'name', 'address.city'])
  backend.getValidation.mockResolvedValue({ validator: '', level: 'strict', action: 'error' })
  backend.startCopy.mockResolvedValue({ id: 'j1' })
})

describe('CreateCollectionDialog', () => {
  it('creates a plain collection with just a name', async () => {
    const onCreated = vi.fn()
    render(<CreateCollectionDialog conn={conn} db="shop" onClose={vi.fn()} onCreated={onCreated} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'Collection name' }), { target: { value: 'orders' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('shop', 'orders'))
    expect(backend.createCollection).toHaveBeenCalledWith('c1', 'shop', 'orders')
    expect(backend.createCollectionWithOptions).not.toHaveBeenCalled()
  })

  it('passes capped, collation and validator options', async () => {
    render(<CreateCollectionDialog conn={conn} db="shop" onClose={vi.fn()} onCreated={vi.fn()} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'Collection name' }), { target: { value: 'log' } })
    fireEvent.click(screen.getByRole('button', { name: /Options: capped/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: /Capped/ }))
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Maximum size' }), { target: { value: '1048576' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Validator' }), { target: { value: '{ $jsonSchema: { required: ["msg"] } }' } })
    fireEvent.change(screen.getByRole('combobox', { name: 'Validation action' }), { target: { value: 'warn' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() =>
      expect(backend.createCollectionWithOptions).toHaveBeenCalledWith('c1', 'shop', expect.objectContaining({ name: 'log', capped: true, sizeBytes: 1048576, validationAction: 'warn', validator: '{ $jsonSchema: { required: ["msg"] } }' })),
    )
  })

  it('creates a view on an existing collection', async () => {
    render(<CreateCollectionDialog conn={conn} db="shop" onClose={vi.fn()} onCreated={vi.fn()} />)
    fireEvent.click(screen.getByRole('radio', { name: 'View' }))
    fireEvent.change(screen.getByRole('textbox', { name: 'View name' }), { target: { value: 'adults' } })
    await screen.findByRole('option', { name: 'people' })
    fireEvent.change(screen.getByRole('combobox', { name: 'Reads from' }), { target: { value: 'people' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(backend.createView).toHaveBeenCalledWith('c1', 'shop', 'adults', 'people', expect.stringContaining('$match'), ''))
  })
})

describe('CollectionInfoDialog', () => {
  it('shows statistics and the recreate command', async () => {
    render(<CollectionInfoDialog conn={conn} db="shop" coll="people" onClose={vi.fn()} />)
    expect(await screen.findByText('db.createCollection("people", {})')).toBeInTheDocument()
    expect(screen.getByText('Documents').closest('tr')).toHaveTextContent('2')
  })

  it('manages indexes: hide, TTL, drop with confirmation, create', async () => {
    app.confirm.mockResolvedValue(true)
    app.prompt.mockResolvedValue('120')
    render(<CollectionInfoDialog conn={conn} db="shop" coll="people" tab="indexes" onClose={vi.fn()} />)
    const row = (await screen.findByText('email_1')).closest('tr') as HTMLElement
    expect(row).toHaveTextContent('unique · TTL 60s')
    expect(within(screen.getByText('_id_').closest('tr') as HTMLElement).queryByRole('button')).toBeNull()
    fireEvent.click(within(row).getByRole('button', { name: 'Hide' }))
    await waitFor(() => expect(backend.modifyIndex).toHaveBeenCalledWith('c1', 'shop', 'people', 'email_1', { hidden: true }))
    fireEvent.click(within(row).getByRole('button', { name: 'TTL…' }))
    await waitFor(() => expect(backend.modifyIndex).toHaveBeenCalledWith('c1', 'shop', 'people', 'email_1', { ttl: 120 }))
    fireEvent.click(within(row).getByRole('button', { name: 'Drop email_1' }))
    await waitFor(() => expect(backend.dropIndex).toHaveBeenCalledWith('c1', 'shop', 'people', 'email_1'))
    expect(app.confirm).toHaveBeenCalledWith(expect.objectContaining({ typeToConfirm: 'email_1', danger: true }))

    fireEvent.click(screen.getByRole('button', { name: 'New index…' }))
    // Suggested fields: email already leads an index; clicking adds keys with the chosen direction.
    const chips = within(await screen.findByRole('group', { name: 'Suggested index fields' }))
    expect(chips.queryByRole('button', { name: /^_id/ })).toBeNull()
    expect(chips.getByRole('button', { name: /email/ })).toHaveTextContent('indexed')
    fireEvent.click(chips.getByRole('button', { name: /^name/ }))
    expect(screen.getByRole('textbox', { name: 'Keys' })).toHaveValue('{ name: 1 }')
    expect(chips.getByRole('button', { name: /^name/ })).toBeDisabled()
    fireEvent.change(screen.getByRole('combobox', { name: 'Key type' }), { target: { value: '-1' } })
    fireEvent.click(chips.getByRole('button', { name: /address.city/ }))
    expect(screen.getByRole('textbox', { name: 'Keys' })).toHaveValue('{ name: 1, "address.city": -1 }')
    fireEvent.change(screen.getByRole('textbox', { name: 'Keys' }), { target: { value: '{ name: 1 }' } })
    fireEvent.click(screen.getByRole('checkbox', { name: 'Unique' }))
    fireEvent.click(screen.getByRole('button', { name: 'Create index' }))
    await waitFor(() => expect(backend.createIndex).toHaveBeenCalledWith('index-1', 'c1', 'shop', 'people', expect.objectContaining({ keys: '{ name: 1 }', unique: true })))
  })

  it('checks a validator against existing documents before saving', async () => {
    backend.checkValidator.mockResolvedValue({ checked: 10, failing: 2, sample: ['1', '7'] })
    render(<CollectionInfoDialog conn={conn} db="shop" coll="people" tab="validation" onClose={vi.fn()} />)
    const editor = await screen.findByRole('textbox', { name: /Validator/ })
    fireEvent.change(editor, { target: { value: '{ age: { $exists: true } }' } })
    fireEvent.click(screen.getByRole('button', { name: 'Check existing documents' }))
    expect(await screen.findByText(/2 of 10 existing documents don't match \(e.g. _id 1, 7\)/)).toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: 'Validation level' }), { target: { value: 'moderate' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save rules' }))
    await waitFor(() => expect(backend.setValidation).toHaveBeenCalledWith('c1', 'shop', 'people', { validator: '{ age: { $exists: true } }', level: 'moderate', action: 'error' }))
  })
})

describe('Rename and copy', () => {
  it('needs the target typed to replace an existing collection', async () => {
    const onDone = vi.fn()
    render(<RenameCollectionDialog conn={conn} db="shop" coll="people" onClose={vi.fn()} onDone={onDone} />)
    const go = screen.getByRole('button', { name: 'Rename' })
    expect(go).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox', { name: 'New name' }), { target: { value: 'customers' } })
    fireEvent.click(screen.getByRole('checkbox', { name: /Replace an existing/ }))
    expect(go).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox', { name: /type shop.customers to confirm/ }), { target: { value: 'shop.customers' } })
    fireEvent.click(go)
    await waitFor(() => expect(backend.renameCollection).toHaveBeenCalledWith('c1', 'shop', 'people', 'shop', 'customers', true, 'shop.customers'))
    expect(onDone).toHaveBeenCalled()
  })

  it('copies a collection to another connection and refuses copying onto itself', async () => {
    render(<CopyDialog conn={conn} db="shop" coll="people" onClose={vi.fn()} />)
    const go = screen.getByRole('button', { name: 'Copy' })
    fireEvent.change(screen.getByRole('textbox', { name: 'Target collection' }), { target: { value: 'people' } })
    expect(go).toBeDisabled()
    expect(screen.getByText(/can't be copied onto itself/)).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Ro (read-only)' })).toBeDisabled()
    fireEvent.change(screen.getByRole('combobox', { name: 'Target connection' }), { target: { value: 'c2' } })
    fireEvent.click(go)
    await waitFor(() =>
      expect(backend.startCopy).toHaveBeenCalledWith(
        { srcConn: 'c1', dstConn: 'c2', options: expect.objectContaining({ srcDb: 'shop', collections: ['people'], dstDb: 'shop', dstColl: 'people', mode: 'insert' }) },
        '',
      ),
    )
  })
})

describe('index key helpers', () => {
  it('adds keys, quoting dotted paths and skipping duplicates', () => {
    expect(addIndexKey('{ field: 1 }', 'email', '1')).toBe('{ email: 1 }')
    expect(addIndexKey('{}', 'a.b', '-1')).toBe('{ "a.b": -1 }')
    expect(addIndexKey('{ a: 1 }', 'title', '"text"')).toBe('{ a: 1, title: "text" }')
    expect(addIndexKey('{ a: 1 }', 'a', '-1')).toBe('{ a: 1 }')
    expect(indexKeyFields('{"email": 1, "a.b": -1}')).toEqual(['email', 'a.b'])
    expect(indexKeyFields("{ x: 1, 'y z': 1 }")).toEqual(['x', 'y z'])
  })
})
