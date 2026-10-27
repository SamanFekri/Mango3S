import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SessionInfo } from '../lib/types'

const backend = vi.hoisted(() => ({ listSearchIndexes: vi.fn(), createSearchIndex: vi.fn(), updateSearchIndex: vi.fn(), dropSearchIndex: vi.fn() }))
const app = vi.hoisted(() => ({ confirm: vi.fn(), toast: vi.fn() }))
vi.mock('../lib/backend', () => ({ backend }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { SEARCH_TEMPLATES, SearchIndexes } from './SearchIndexes'
import { STAGE_TEMPLATES } from '../lib/pipeline'

const conn: SessionInfo = { id: 'c1', name: 'Atlas', readOnly: false, serverVersion: '8.0.4', topology: 'replicaSet', connectedAt: '' }

beforeEach(() => {
  vi.clearAllMocks()
  backend.createSearchIndex.mockResolvedValue('vec')
})

describe('Search indexes', () => {
  it('explains when the server has no Atlas Search', async () => {
    backend.listSearchIndexes.mockResolvedValue({ supported: false, reason: 'This server has no Atlas Search.', indexes: [] })
    render(<SearchIndexes conn={conn} db="shop" coll="movies" />)
    expect(await screen.findByText('This server has no Atlas Search.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'New search index…' })).not.toBeInTheDocument()
  })

  it('lists indexes and creates a vector index from its template', async () => {
    backend.listSearchIndexes.mockResolvedValue({ supported: true, indexes: [{ name: 'default', type: 'search', status: 'READY', queryable: true, definition: '{}' }] })
    render(<SearchIndexes conn={conn} db="shop" coll="movies" />)
    expect(await screen.findByText('ready')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'New search index…' }))
    fireEvent.change(screen.getByLabelText('Search index name'), { target: { value: 'vec' } })
    fireEvent.change(screen.getByLabelText('Search index type'), { target: { value: 'vectorSearch' } })
    expect(screen.getByLabelText('Search index definition')).toHaveValue(SEARCH_TEMPLATES.vectorSearch)
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    await waitFor(() =>
      expect(backend.createSearchIndex).toHaveBeenCalledWith('c1', 'shop', 'movies', { name: 'vec', type: 'vectorSearch', definition: SEARCH_TEMPLATES.vectorSearch }),
    )
  })

  it('offers $search and $vectorSearch stage templates', () => {
    expect(STAGE_TEMPLATES.map((t) => t.operator)).toEqual(expect.arrayContaining(['$search', '$vectorSearch']))
  })
})
