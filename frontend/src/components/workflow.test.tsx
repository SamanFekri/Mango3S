import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SessionInfo } from '../lib/types'

const backend = vi.hoisted(() => ({
  getProfiling: vi.fn(),
  setProfiling: vi.fn(),
  listProfile: vi.fn(),
  clearProfile: vi.fn(),
  chooseSaveFile: vi.fn(),
  startSQLExport: vi.fn(),
  analyzeIndexes: vi.fn(),
  createIndex: vi.fn(),
  modifyIndex: vi.fn(),
  dropIndex: vi.fn(),
}))
const app = vi.hoisted(() => ({ toast: vi.fn(), confirm: vi.fn(), jobs: [] }))
vi.mock('../lib/backend', () => ({ backend, newOpId: (p: string) => `${p}-1` }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { IndexAdvisorDialog, ProfilerDialog, SqlExportDialog } from './WorkflowDialogs'

const conn: SessionInfo = { id: 'c1', name: 'Prod', readOnly: false, serverVersion: '8.0.1', topology: 'standalone', connectedAt: '' }

beforeEach(() => {
  vi.clearAllMocks()
  backend.getProfiling.mockResolvedValue({ level: 0, slowms: 100, sampleRate: 1 })
  backend.listProfile.mockResolvedValue([
    { ts: '2026-10-19T08:00:00Z', op: 'query', ns: 'shop.orders', millis: 250, planSummary: 'COLLSCAN', keysExamined: 0, docsExamined: 5000, nreturned: 3, command: '{"find": "orders"}', client: '', user: '', appName: 'api' },
  ])
  backend.startSQLExport.mockResolvedValue({ id: 'j1' })
})

describe('ProfilerDialog', () => {
  it('lists slow operations and shows the command', async () => {
    render(<ProfilerDialog conn={conn} db="shop" onClose={vi.fn()} />)
    expect(await screen.findByText('COLLSCAN')).toHaveClass('warning-text')
    fireEvent.click(screen.getByText('shop.orders'))
    expect(screen.getByLabelText('Profiled command')).toHaveTextContent('{"find": "orders"}')
  })

  it('asks before profiling every operation', async () => {
    app.confirm.mockResolvedValue(true)
    render(<ProfilerDialog conn={conn} db="shop" onClose={vi.fn()} />)
    fireEvent.click(await screen.findByRole('radio', { name: 'All operations' }))
    await waitFor(() => expect(backend.setProfiling).toHaveBeenCalledWith('c1', 'shop', { level: 2, slowms: 100, sampleRate: 1 }))
    expect(app.confirm).toHaveBeenCalled()
  })

  it('filters by collection and duration', async () => {
    render(<ProfilerDialog conn={conn} db="shop" onClose={vi.fn()} />)
    await screen.findByText('COLLSCAN')
    fireEvent.change(screen.getByLabelText('Collection filter'), { target: { value: 'orders' } })
    fireEvent.change(screen.getByLabelText('Minimum duration'), { target: { value: '100' } })
    await waitFor(() => expect(backend.listProfile).toHaveBeenLastCalledWith('c1', 'shop', { coll: 'orders', op: '', minMillis: 100, limit: 200 }))
  })
})

describe('SqlExportDialog', () => {
  it('exports with the chosen dialect and options', async () => {
    backend.chooseSaveFile.mockResolvedValue('/out/orders.sql')
    render(<SqlExportDialog conn={conn} db="shop" coll="orders" onClose={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Export' })).toBeDisabled()
    fireEvent.click(screen.getByRole('radio', { name: 'MySQL' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save as…' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Export' })).toBeEnabled())
    expect(backend.chooseSaveFile).toHaveBeenCalledWith('Save SQL script', 'orders.mysql.sql')
    fireEvent.click(screen.getByRole('checkbox', { name: /DROP TABLE/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    await waitFor(() =>
      expect(backend.startSQLExport).toHaveBeenCalledWith({
        connId: 'c1',
        db: 'shop',
        coll: 'orders',
        path: '/out/orders.sql',
        options: { dialect: 'mysql', table: 'orders', filter: '', sampleSize: 1000, batchRows: 200, flatten: true, dropTable: true },
      }),
    )
  })
})

const advice = {
  profilingLevel: 1,
  analyzed: 12,
  since: '2026-10-19T08:00:00Z',
  notes: [],
  shapes: [
    { coll: 'orders', op: 'query', shape: '{status: =, age: range} sort {created: -1}', count: 8, totalMillis: 4000, avgMillis: 500, maxMillis: 900, docsExamined: 80000, keysExamined: 0, returned: 40, plans: ['COLLSCAN'], example: '{"find": "orders"}', lastSeen: '', status: 'missing', suggested: '{ status: 1, created: -1, age: 1 }', name: 'status_1_created_-1_age_1', reason: 'No index serves this query: it scans the whole collection.' },
    { coll: 'orders', op: 'query', shape: '{sku: =}', count: 2, totalMillis: 300, avgMillis: 150, maxMillis: 200, docsExamined: 2, keysExamined: 2, returned: 2, plans: [], example: '', lastSeen: '', status: 'hidden', servedBy: 'sku_1', reason: '' },
  ],
  unused: [{ coll: 'orders', name: 'c_1', keys: '{ c: 1 }', reason: 'Not used since 2026-10-19.', size: 4096, hidden: false }],
  redundant: [{ coll: 'orders', name: 'a_1', keys: '{ a: 1 }', reason: 'Its keys are the start of a_1_b_1.', size: 4096, hidden: false }],
}

describe('IndexAdvisorDialog', () => {
  it('suggests indexes for slow query shapes and acts on findings', async () => {
    backend.analyzeIndexes.mockResolvedValue(advice)
    app.confirm.mockResolvedValue(true)
    render(<IndexAdvisorDialog conn={conn} db="shop" coll="orders" onClose={vi.fn()} />)
    expect(await screen.findByText('{ status: 1, created: -1, age: 1 }')).toBeInTheDocument()
    expect(backend.analyzeIndexes).toHaveBeenCalledWith('advisor-1', 'c1', 'shop', { coll: 'orders', minMillis: 0, limit: 5000 })
    expect(screen.getByText('no index')).toHaveClass('warning-text')

    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(backend.createIndex).toHaveBeenCalledWith('index-1', 'c1', 'shop', 'orders', expect.objectContaining({ keys: '{ status: 1, created: -1, age: 1 }' })))
    fireEvent.click(screen.getByRole('button', { name: 'Unhide' }))
    await waitFor(() => expect(backend.modifyIndex).toHaveBeenCalledWith('c1', 'shop', 'orders', 'sku_1', { hidden: false }))

    fireEvent.click(screen.getByText('{status: =, age: range} sort {created: -1}'))
    expect(screen.getByLabelText('Shape details')).toHaveTextContent('Plans seen: COLLSCAN')

    fireEvent.click(screen.getAllByRole('button', { name: 'Hide' })[0])
    await waitFor(() => expect(backend.modifyIndex).toHaveBeenCalledWith('c1', 'shop', 'orders', 'c_1', { hidden: true }))
    fireEvent.click(screen.getByRole('button', { name: 'Drop orders.a_1' }))
    await waitFor(() => expect(backend.dropIndex).toHaveBeenCalledWith('c1', 'shop', 'orders', 'a_1'))
    expect(app.confirm).toHaveBeenLastCalledWith(expect.objectContaining({ typeToConfirm: 'a_1', danger: true }))
  })

  it('offers to turn the profiler on when it is off', async () => {
    backend.analyzeIndexes.mockResolvedValue({ ...advice, profilingLevel: 0, analyzed: 0, shapes: [], unused: [], redundant: [], notes: ['The profiler is off, so there are no queries to analyze.'] })
    render(<IndexAdvisorDialog conn={conn} db="shop" onClose={vi.fn()} />)
    fireEvent.click(await screen.findByRole('button', { name: 'Turn on the profiler (slow operations)' }))
    await waitFor(() => expect(backend.setProfiling).toHaveBeenCalledWith('c1', 'shop', { level: 1, slowms: 0, sampleRate: 0 }))
  })
})
