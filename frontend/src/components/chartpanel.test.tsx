import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { openAction } from '../lib/library'
import type { SavedItem, SessionInfo } from '../lib/types'

const backend = vi.hoisted(() => ({ runChart: vi.fn(), saveItem: vi.fn() }))
const app = vi.hoisted(() => ({ prompt: vi.fn(), toast: vi.fn(), toastError: vi.fn() }))
vi.mock('../lib/backend', () => ({ backend, newOpId: (p: string) => `${p}-1` }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { ChartPanel } from './ChartPanel'

beforeEach(() => {
  vi.clearAllMocks()
  backend.runChart.mockResolvedValue({
    xKind: 'category',
    truncated: false,
    pipeline: '[{ $group: {} }]',
    points: [
      { x: 'Paris', y: 40 },
      { x: 'Rome', y: 25 },
    ],
  })
  backend.saveItem.mockResolvedValue({ id: 'i1' })
})

describe('Chart view', () => {
  it('groups on the server with the query filter, draws it and saves it', async () => {
    app.prompt.mockResolvedValue('Orders per city')
    render(<ChartPanel connId="c1" db="shop" coll="orders" filter='{status: "A"}' fields={['city', 'total', 'createdAt']} />)
    expect(screen.getByText(/Choose what to chart/)).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('X axis'), { target: { value: 'city' } })
    fireEvent.change(screen.getByLabelText('Aggregation'), { target: { value: 'sum' } })
    fireEvent.change(screen.getByLabelText('Field'), { target: { value: 'total' } })
    await waitFor(() =>
      expect(backend.runChart).toHaveBeenLastCalledWith('chart-1', {
        connId: 'c1', db: 'shop', coll: 'orders', filter: '{status: "A"}',
        spec: { kind: 'bar', x: 'city', y: 'total', agg: 'sum', dateUnit: '', series: '', limit: 0 },
      }),
    )
    expect(await screen.findByRole('img', { name: 'orders: sum of total by city' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Save chart…' }))
    await waitFor(() =>
      expect(backend.saveItem).toHaveBeenCalledWith(expect.objectContaining({ kind: 'chart', name: 'Orders per city', coll: 'orders', chart: expect.objectContaining({ x: 'city', agg: 'sum' }) })),
    )
  })

  it('reopens a saved chart from the library in the Chart view', () => {
    const session = { id: 'c1', name: 'Local' } as SessionInfo
    const item = { id: 'i1', kind: 'chart', name: 'x', description: '', db: 'shop', coll: 'orders', query: { filter: '{a: 1}' }, chart: { kind: 'pie', x: 'city' } } as unknown as SavedItem
    const action = openAction(item, session, item)
    expect(action).toMatchObject({ type: 'open', tab: { kind: 'collection', coll: 'orders', init: { chart: { kind: 'pie', x: 'city' }, query: { filter: '{a: 1}' } } } })
  })
})
