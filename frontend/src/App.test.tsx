import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const profile = {
  id: 'p1',
  name: 'Local dev',
  uri: 'mongodb://localhost:27017',
  connectTimeoutSeconds: 10,
  readOnly: false,
  hasPassword: false,
  createdAt: '',
  updatedAt: '',
}
const session = { id: 'p1', name: 'Local dev', readOnly: false, serverVersion: '8.0.0', topology: 'standalone', connectedAt: '' }

const backend = vi.hoisted(() => ({
  getAppInfo: vi.fn(),
  getSettings: vi.fn(),
  saveSettings: vi.fn(),
  listProfiles: vi.fn(),
  saveProfile: vi.fn(),
  deleteProfile: vi.fn(),
  testConnection: vi.fn(),
  connect: vi.fn(),
  disconnect: vi.fn(),
  listConnections: vi.fn(),
  listDatabases: vi.fn(),
  listCollections: vi.fn(),
  collectionStats: vi.fn(),
  sampleFields: vi.fn(),
  runQuery: vi.fn(),
  countQuery: vi.fn(),
  cancel: vi.fn(),
  listJobs: vi.fn(),
  cancelJob: vi.fn(),
  clearFinishedJobs: vi.fn(),
  listSaved: vi.fn(),
  listHistory: vi.fn(),
}))

const jobListeners = vi.hoisted(() => [] as ((j: unknown) => void)[])

vi.mock('./lib/backend', () => ({
  backend,
  newOpId: (p: string) => `${p}-1`,
  onJobUpdate: (cb: (j: unknown) => void) => {
    jobListeners.push(cb)
    return () => undefined
  },
}))

import App from './App'

beforeEach(() => {
  vi.clearAllMocks()
  backend.getAppInfo.mockResolvedValue({ version: 'test', configDir: '/tmp', logFile: '', secretBackend: 'file' })
  backend.getSettings.mockResolvedValue({
    theme: 'dark', pageSize: 50, queryTimeoutSeconds: 60, confirmDeletes: true, defaultView: 'json', editorFontSize: 13, logLevel: 'info',
  })
  backend.listProfiles.mockResolvedValue([profile])
  backend.listConnections.mockResolvedValue([])
  backend.connect.mockResolvedValue(session)
  backend.listDatabases.mockResolvedValue([{ name: 'shop', sizeOnDisk: 4096, empty: false }])
  backend.listCollections.mockResolvedValue([{ name: 'orders', type: 'collection', readOnly: false }])
  backend.sampleFields.mockResolvedValue(['_id', 'status'])
  backend.runQuery.mockResolvedValue({
    documents: [{ id: '1', text: '{\n  "_id": 1,\n  "status": "A"\n}', json: '{"_id":1,"status":"A"}' }],
    page: 0, pageSize: 50, hasMore: false, durationMs: 3,
  })
  backend.cancel.mockResolvedValue(true)
  backend.listJobs.mockResolvedValue([])
  backend.cancelJob.mockResolvedValue(true)
  backend.listSaved.mockResolvedValue([
    { id: 's1', kind: 'sql', name: 'Big orders', description: '', db: 'shop', coll: '', text: 'SELECT * FROM orders', createdAt: '', updatedAt: new Date().toISOString() },
  ])
  backend.listHistory.mockResolvedValue([])
  jobListeners.length = 0
})

describe('App', () => {
  it('connects, browses and opens a collection', async () => {
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Mango3S' })).toBeInTheDocument()

    const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
    fireEvent.click(await within(sidebar).findByRole('button', { name: 'Connect' }))
    await waitFor(() => expect(backend.connect).toHaveBeenCalledWith('p1'))
    expect(await screen.findByText('Connected to Local dev')).toBeInTheDocument()

    fireEvent.click(await within(sidebar).findByText('shop'))
    fireEvent.click(await within(sidebar).findByText('orders'))

    expect(await screen.findByRole('tab', { name: /orders/ })).toHaveAttribute('aria-selected', 'true')
    await waitFor(() => expect(backend.runQuery).toHaveBeenCalled())
    const req = backend.runQuery.mock.calls[0][1]
    expect(req).toMatchObject({ connId: 'p1', db: 'shop', coll: 'orders', filter: '{}', page: 0, pageSize: 50 })
    expect(await screen.findByText('"A"')).toBeInTheDocument()
  })

  it('opens the connection manager and settings', async () => {
    render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'New connection' }))
    expect(await screen.findByRole('dialog', { name: 'Connections' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))

    fireEvent.click(screen.getByTitle('Settings'))
    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    expect(document.documentElement.dataset.theme).toBe('dark')
  })

  it('shows background job progress and lets the user cancel', async () => {
    render(<App />)
    await screen.findByRole('heading', { name: 'Mango3S' })
    const job = {
      id: 'j1', kind: 'export', title: 'Export shop.orders → orders.json', state: 'running', done: 250, total: 1000,
      unit: 'documents', message: '', createdAt: '2026-01-01T00:00:00Z', startedAt: '2026-01-01T00:00:00Z', finishedAt: '0001-01-01T00:00:00Z',
    }
    act(() => jobListeners.forEach((l) => l(job)))
    const indicator = await screen.findByRole('button', { name: /1 job running · 25%/ })
    fireEvent.click(indicator)
    expect(await screen.findByText('Export shop.orders → orders.json')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '25')
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(backend.cancelJob).toHaveBeenCalledWith('j1')

    act(() => jobListeners.forEach((l) => l({ ...job, state: 'succeeded', done: 1000, result: '1000 documents written' })))
    expect(await screen.findByText('1000 documents written')).toBeInTheDocument()
  })

  it('opens the library with saved items', async () => {
    render(<App />)
    fireEvent.click(await screen.findByTitle('Saved queries and history'))
    expect(await screen.findByText('Big orders')).toBeInTheDocument()
    expect(backend.listSaved).toHaveBeenCalled()
  })
})
