import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { formatNumber, yTicks } from '../components/Chart'
import type { Tab } from '../lib/tabs'
import type { MonitorSample } from '../lib/types'

const listeners = vi.hoisted(() => [] as ((e: { connId: string; sample: MonitorSample }) => void)[])
const backend = vi.hoisted(() => ({
  startMonitor: vi.fn(),
  stopMonitor: vi.fn(),
  getServerOverview: vi.fn(),
  listCurrentOps: vi.fn(),
  killOperation: vi.fn(),
}))
const app = vi.hoisted(() => ({
  confirm: vi.fn(),
  toast: vi.fn(),
  sessions: [{ id: 'c1', name: 'Local', readOnly: false, serverVersion: '8.0.1', topology: 'replicaSet', connectedAt: '' }],
}))
vi.mock('../lib/backend', () => ({
  backend,
  onMonitorSample: (cb: (e: { connId: string; sample: MonitorSample }) => void) => {
    listeners.push(cb)
    return () => undefined
  },
}))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { MonitorView } from './MonitorView'

const tab = { id: 't1', kind: 'monitor', connId: 'c1', connName: 'Local' } as Tab
const sample = (t: number, query: number, first = false): MonitorSample => ({
  time: new Date(Date.UTC(2026, 0, 1, 12, 0, t)).toISOString(),
  insert: 1, query, update: 0, delete: 0, getmore: 0, command: 4, netIn: 1000, netOut: 2000, requests: 5,
  connections: 7, connectionsAvailable: 100, queueReaders: 0, queueWriters: 0, activeReaders: 1, activeWriters: 0,
  cacheBytes: 512, cacheMax: 1024, cacheDirty: 10, residentMb: 80, first,
})

beforeEach(() => {
  vi.clearAllMocks()
  listeners.length = 0
  backend.startMonitor.mockResolvedValue({ intervalSeconds: 2, samples: [sample(0, 0, true), sample(2, 10)] })
  backend.stopMonitor.mockResolvedValue(undefined)
  backend.getServerOverview.mockResolvedValue({
    host: 'db1:27017', version: '8.0.4', process: 'mongod', uptime: 7200, storageEngine: 'wiredTiger',
    replication: { set: 'rs0', oplogHours: 30.5, oplogSizeMb: 990, members: [
      { name: 'db1:27017', state: 'PRIMARY', health: true, self: true, lagSeconds: 0, uptime: 7200 },
      { name: 'db2:27017', state: 'SECONDARY', health: true, self: false, lagSeconds: 42, pingMs: 3, uptime: 7100, syncSource: 'db1:27017' },
    ] },
  })
  backend.listCurrentOps.mockResolvedValue([
    { opid: '12345', op: 'query', ns: 'shop.orders', seconds: 31.5, client: '10.0.0.5:5123', appName: 'api', user: '', desc: 'conn9', command: '{find: "orders", filter: {$where: "sleep(99999)"}}', planSummary: 'COLLSCAN', waitingForLock: false, active: true },
  ])
  backend.killOperation.mockResolvedValue(undefined)
})

describe('MonitorView', () => {
  it('charts live samples, shows replication lag and kills an operation after typed confirmation', async () => {
    app.confirm.mockResolvedValue(true)
    render(<MonitorView tab={tab} />)
    expect(await screen.findByRole('img', { name: 'Operations per second' })).toBeInTheDocument()
    await waitFor(() => expect(backend.startMonitor).toHaveBeenCalledWith('c1', 2))
    act(() => listeners.forEach((l) => l({ connId: 'c1', sample: sample(4, 20) })))
    expect(screen.getByRole('img', { name: /^WiredTiger cache \(1(\.0+)? KB configured\)$/ })).toBeInTheDocument()

    expect(await screen.findByText('42 s')).toHaveClass('warning-text')
    expect(screen.getByText(/oplog window 30.5 h/)).toBeInTheDocument()

    expect(await screen.findByText('shop.orders')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Kill operation 12345' }))
    await waitFor(() => expect(app.confirm).toHaveBeenCalledWith(expect.objectContaining({ typeToConfirm: '12345', danger: true })))
    expect(backend.killOperation).toHaveBeenCalledWith('c1', '12345', '12345')
  })

  it('hides Kill on read-only connections', async () => {
    app.sessions = [{ ...app.sessions[0]!, readOnly: true }]
    render(<MonitorView tab={tab} />)
    expect(await screen.findByText('shop.orders')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Kill operation/ })).toBeNull()
    app.sessions = [{ ...app.sessions[0]!, readOnly: false }]
  })
})

describe('chart helpers', () => {
  it('pick nice ticks and short numbers', () => {
    expect(yTicks(0, 87)).toEqual([0, 20, 40, 60, 80, 100])
    expect(yTicks(0, 0)).toEqual([0, 0.2, 0.4, 0.6, 0.8, 1])
    expect(yTicks(0, 100)).toEqual([0, 20, 40, 60, 80, 100])
    expect(formatNumber(1534)).toBe('1.5k')
    expect(formatNumber(2_500_000)).toBe('2.5M')
  })
})
