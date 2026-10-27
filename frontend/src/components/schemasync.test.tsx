import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { CompareOptions } from '../lib/types'

const backend = vi.hoisted(() => ({ compareSchema: vi.fn(), syncSchema: vi.fn() }))
const app = vi.hoisted(() => ({ toast: vi.fn() }))
vi.mock('../lib/backend', () => ({ backend }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { SchemaSync } from './SchemaSync'

const options: CompareOptions = {
  source: { connId: 'c1', db: 'shop', coll: 'users' },
  target: { connId: 'c2', db: 'shop_copy', coll: 'users' },
  filter: '',
  ignoreFields: [],
  ignoreFieldOrder: true,
  sampleSize: 50,
}

beforeEach(() => {
  vi.clearAllMocks()
  backend.compareSchema.mockResolvedValueOnce({
    fingerprint: 'fp1',
    changes: [
      { id: 'index:email_1', kind: 'createIndex', name: 'email_1', source: '{"key": {"email": 1}}' },
      { id: 'index:extra_1', kind: 'dropIndex', name: 'extra_1', target: '{"key": {"extra": 1}}' },
      { id: 'validator', kind: 'validator', source: '{}', target: '{}' },
    ],
  })
  backend.compareSchema.mockResolvedValue({ fingerprint: 'fp2', changes: [] })
  backend.syncSchema.mockResolvedValue({ applied: ['index:email_1', 'validator', 'index:extra_1'], failed: [] })
})

describe('Index and validation sync', () => {
  it('preselects additive changes and asks to type the target before dropping', async () => {
    render(<SchemaSync options={options} />)
    fireEvent.click(screen.getByRole('button', { name: /Compare indexes/ }))
    expect(await screen.findByRole('checkbox', { name: 'Create index email_1' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Set validation rules' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Drop index extra_1' })).not.toBeChecked()

    fireEvent.click(screen.getByRole('checkbox', { name: 'Drop index extra_1' }))
    const apply = screen.getByRole('button', { name: /Apply 3 changes/ })
    expect(apply).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Confirm target'), { target: { value: 'shop_copy.users' } })
    fireEvent.click(apply)
    await waitFor(() => expect(backend.syncSchema).toHaveBeenCalledWith(options, ['index:email_1', 'validator', 'index:extra_1'], 'fp1', 'shop_copy.users'))
    expect(await screen.findByText('Indexes and validation rules are the same.')).toBeInTheDocument()
  })
})
