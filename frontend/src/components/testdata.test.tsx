import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SessionInfo } from '../lib/types'

const backend = vi.hoisted(() => ({ testDataTemplateFromSchema: vi.fn(), previewTestData: vi.fn(), startTestData: vi.fn() }))
const app = vi.hoisted(() => ({ confirm: vi.fn(), toast: vi.fn() }))
vi.mock('../lib/backend', () => ({ backend }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { TestDataDialog } from './TestDataDialog'

const conn: SessionInfo = { id: 'c1', name: 'Local', readOnly: false, serverVersion: '8.0.1', topology: 'standalone', connectedAt: '' }

beforeEach(() => {
  vi.clearAllMocks()
  backend.testDataTemplateFromSchema.mockResolvedValue('{ name: $name }')
  backend.previewTestData.mockResolvedValue({ documents: [{ id: '', text: '{ "name": "Ada Lovelace" }', json: '' }], existing: 120, generators: [] })
  backend.startTestData.mockResolvedValue({ id: 'j1' })
})

describe('Generate test data', () => {
  it('previews from the schema template and asks before adding to a non-empty collection', async () => {
    app.confirm.mockResolvedValue(true)
    const onClose = vi.fn()
    render(<TestDataDialog conn={conn} db="shop" coll="people" onClose={onClose} />)
    expect(await screen.findByRole('textbox', { name: 'Template' })).toHaveValue('{ name: $name }')
    expect(await screen.findByText('{ "name": "Ada Lovelace" }')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Seed'), { target: { value: '42' } })
    fireEvent.change(screen.getByLabelText('Number of documents'), { target: { value: '500' } })
    await waitFor(() => expect(backend.previewTestData).toHaveBeenLastCalledWith({ connId: 'c1', db: 'shop', coll: 'people', template: '{ name: $name }', seed: 42, count: 500 }))
    fireEvent.click(screen.getByRole('button', { name: /Generate 500 documents/ }))
    await waitFor(() => expect(backend.startTestData).toHaveBeenCalledWith({ connId: 'c1', db: 'shop', coll: 'people', template: '{ name: $name }', seed: 42, count: 500 }))
    expect(app.confirm).toHaveBeenCalledWith(expect.objectContaining({ danger: true }))
    expect(onClose).toHaveBeenCalled()
  })
})
