import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SessionInfo } from '../lib/types'

const backend = vi.hoisted(() => ({
  collectionDetails: vi.fn(),
  databaseDetails: vi.fn(),
  listIndexDetails: vi.fn(),
  getValidation: vi.fn(),
}))
const app = vi.hoisted(() => ({ toast: vi.fn(), confirm: vi.fn(), prompt: vi.fn(), jobs: [], sessions: [] as SessionInfo[], connectToShard: vi.fn() }))
vi.mock('../lib/backend', () => ({ backend, newOpId: (p: string) => `${p}-1` }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { CollectionInfoDialog, DatabaseInfoDialog } from './ManageDialogs'

const conn: SessionInfo = { id: 'c1', name: 'Cluster', readOnly: false, serverVersion: '8.0.4', topology: 'sharded', connectedAt: '' }

beforeEach(() => {
  vi.clearAllMocks()
  app.connectToShard.mockResolvedValue({ id: 'c1~shard~shard1' })
  backend.collectionDetails.mockResolvedValue({
    type: 'collection', options: '{}', readOnly: false, stats: {}, count: 500, size: 100, storageSize: 4096, totalIndexSize: 4096, avgObjSize: 50,
    capped: false, sharded: true, shell: '',
    sharding: { key: '{userId: 1}', unique: false, balancing: true, chunks: 3, jumbo: 1, byShard: [{ shard: 'shard1', chunks: 2, jumbo: 1 }, { shard: 'shard2', chunks: 1, jumbo: 0 }] },
  })
  backend.databaseDetails.mockResolvedValue({
    collections: 2, views: 0, objects: 501, dataSize: 1000, storageSize: 2000, indexes: 3, indexSize: 100, avgObjSize: 2, fsUsedSize: 0, fsTotalSize: 0,
    sharding: {
      primary: 'shard1', shardedCollections: ['events'],
      shards: [{ id: 'shard1', host: 'shard1/a:27017', state: 1, draining: false }, { id: 'shard2', host: 'shard2/b:27017', state: 1, draining: true }],
      balancer: { mode: 'full', inBalancerRound: false, numBalancerRounds: 12 },
    },
  })
})

describe('sharding awareness', () => {
  it('shows the shard key, chunks per shard and jumbo chunks', async () => {
    render(<CollectionInfoDialog conn={conn} db="shop" coll="events" onClose={vi.fn()} />)
    expect(await screen.findByText('{userId: 1}')).toBeInTheDocument()
    expect(screen.getByText(/1 jumbo/)).toHaveClass('warning-text')
    expect(screen.getByRole('table', { name: 'Chunks per shard' })).toHaveTextContent('shard121shard210')
  })

  it('shows the primary shard and balancer, and connects to a shard', async () => {
    render(<DatabaseInfoDialog conn={conn} db="shop" onClose={vi.fn()} />)
    expect(await screen.findByText('on · 12 rounds')).toBeInTheDocument()
    expect(screen.getByText('events')).toBeInTheDocument()
    expect(screen.getByText('draining')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Connect to shard shard1' }))
    await waitFor(() => expect(app.connectToShard).toHaveBeenCalledWith('c1', 'shard1'))
  })
})
