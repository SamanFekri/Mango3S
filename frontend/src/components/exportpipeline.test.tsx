import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Stage } from '../lib/types'

const backend = vi.hoisted(() => ({ sampleFields: vi.fn(), chooseExportFile: vi.fn(), startExport: vi.fn() }))
const app = vi.hoisted(() => ({ toast: vi.fn() }))
vi.mock('../lib/backend', () => ({ backend }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { ExportWizard } from './ExportWizard'

const query = { connId: 'c1', db: 'shop', coll: 'orders', filter: '', projection: '', sort: '', skip: 0, limit: 0, page: 0, pageSize: 50 }
const stages: Stage[] = [
  { operator: '$match', body: '{ status: "paid" }', enabled: true },
  { operator: '$limit', body: '5', enabled: false },
  { operator: '$group', body: '{ _id: "$city" }', enabled: true },
]

beforeEach(() => {
  vi.clearAllMocks()
  backend.sampleFields.mockResolvedValue(['_id', 'status'])
  backend.chooseExportFile.mockResolvedValue('/tmp/orders-results.json')
  backend.startExport.mockResolvedValue({ id: 'j1' })
})

describe('Export aggregation results', () => {
  it('exports the pipeline instead of a query, without the query fields', async () => {
    const onClose = vi.fn()
    render(<ExportWizard query={query} pipeline={stages} allowDiskUse onClose={onClose} />)
    expect(screen.getByText(/Export pipeline results: shop\.orders/)).toBeInTheDocument()
    expect(screen.getByText(/2 enabled stages/)).toBeInTheDocument()
    expect(screen.queryByText('Filter')).not.toBeInTheDocument()
    expect(screen.queryByPlaceholderText('Filter fields')).not.toBeInTheDocument()
    expect(backend.sampleFields).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Browse…' }))
    await waitFor(() => expect(backend.chooseExportFile).toHaveBeenCalledWith('orders-results.json'))
    fireEvent.click(await screen.findByRole('button', { name: 'Start export' }))
    await waitFor(() =>
      expect(backend.startExport).toHaveBeenCalledWith(expect.objectContaining({ pipeline: stages, allowDiskUse: true, path: '/tmp/orders-results.json', format: 'json' })),
    )
    expect(onClose).toHaveBeenCalled()
  })
})

describe('Excel export', () => {
  it('names the file .xlsx and sends the Excel format', async () => {
    backend.chooseExportFile.mockResolvedValue('/tmp/orders.xlsx')
    render(<ExportWizard query={query} onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Excel' }))
    expect(screen.getByText(/One sheet named after the collection/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Browse…' }))
    await waitFor(() => expect(backend.chooseExportFile).toHaveBeenCalledWith('orders.xlsx'))
    fireEvent.click(await screen.findByRole('button', { name: 'Start export' }))
    await waitFor(() => expect(backend.startExport).toHaveBeenCalledWith(expect.objectContaining({ format: 'xlsx', path: '/tmp/orders.xlsx' })))
  })
})
