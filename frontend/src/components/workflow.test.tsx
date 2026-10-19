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
}))
const app = vi.hoisted(() => ({ toast: vi.fn(), confirm: vi.fn(), jobs: [] }))
vi.mock('../lib/backend', () => ({ backend }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { ProfilerDialog, SqlExportDialog } from './WorkflowDialogs'

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
