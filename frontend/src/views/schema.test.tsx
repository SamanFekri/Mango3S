import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Tab } from '../lib/tabs'

const backend = vi.hoisted(() => ({
  analyzeSchema: vi.fn(),
  cancel: vi.fn(),
  suggestValidator: vi.fn(),
  checkValidator: vi.fn(),
  setValidation: vi.fn(),
}))
const app = vi.hoisted(() => ({
  toast: vi.fn(),
  toastError: vi.fn(),
  confirm: vi.fn(),
  settings: { schemaValidation: false } as { schemaValidation: boolean },
  sessions: [{ id: 'c1', name: 'Local', readOnly: false, serverVersion: '8.0.1', topology: 'standalone', connectedAt: '' }],
}))
vi.mock('../lib/backend', () => ({ backend, newOpId: (p: string) => `${p}-1` }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { SchemaView } from './SchemaView'

const tab = { id: 't1', kind: 'schema', connId: 'c1', connName: 'Local', db: 'shop', coll: 'orders' } as Tab
const report = {
  sampled: 2,
  estimatedDocs: 2,
  durationMs: 3,
  fields: [{ path: 'name', name: 'name', depth: 0, count: 2, presence: 100, types: [{ type: 'string', count: 2, percent: 100 }], distinct: 2, distinctCapped: false, unique: true, samples: ['"Ada"'] }],
}

beforeEach(() => {
  vi.clearAllMocks()
  app.settings = { schemaValidation: false }
  backend.analyzeSchema.mockResolvedValue(report)
  backend.suggestValidator.mockResolvedValue('{ $jsonSchema: { bsonType: "object", required: ["name"] } }')
  backend.checkValidator.mockResolvedValue({ checked: 2, failing: 0, sample: [] })
  backend.setValidation.mockResolvedValue(undefined)
  backend.cancel.mockResolvedValue(true)
})

describe('validation rules from the schema', () => {
  it('are hidden while the setting is off (the default)', async () => {
    render(<SchemaView tab={tab} />)
    await screen.findByText('field paths')
    expect(screen.queryByRole('button', { name: /Create validation rules/ })).toBeNull()
  })

  it('build, check and save rules when turned on', async () => {
    app.settings = { schemaValidation: true }
    render(<SchemaView tab={tab} />)
    fireEvent.click(await screen.findByRole('button', { name: /Create validation rules/ }))
    await waitFor(() => expect(backend.suggestValidator).toHaveBeenCalledWith(report, { requiredPercent: 100, rejectUnknown: false }))
    expect(await screen.findByLabelText('Validator')).toHaveValue('{ $jsonSchema: { bsonType: "object", required: ["name"] } }')

    fireEvent.click(screen.getByLabelText(/Reject fields that weren't seen/))
    await waitFor(() => expect(backend.suggestValidator).toHaveBeenLastCalledWith(report, { requiredPercent: 100, rejectUnknown: true }))

    expect(screen.getByRole('button', { name: 'Save to orders' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: /Check existing documents/ }))
    expect(await screen.findByText('All 2 existing documents match.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Save to orders' }))
    await waitFor(() =>
      expect(backend.setValidation).toHaveBeenCalledWith('c1', 'shop', 'orders', {
        validator: '{ $jsonSchema: { bsonType: "object", required: ["name"] } }',
        level: 'strict',
        action: 'error',
      }),
    )
  })
})
