import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Tab } from '../lib/tabs'
import type { CollectionInfo } from '../lib/types'

const backend = vi.hoisted(() => ({
  listGridFSBuckets: vi.fn(),
  listGridFSFiles: vi.fn(),
  previewGridFSFile: vi.fn(),
  chooseGridFSUploadFile: vi.fn(),
  startGridFSUpload: vi.fn(),
  startGridFSDownload: vi.fn(),
  renameGridFSFile: vi.fn(),
  deleteGridFSFile: vi.fn(),
}))
const app = vi.hoisted(() => ({
  confirm: vi.fn(),
  prompt: vi.fn(),
  toast: vi.fn(),
  jobs: [] as unknown[],
  sessions: [{ id: 'c1', name: 'Local', readOnly: false, serverVersion: '8.0.1', topology: 'standalone', connectedAt: '' }],
}))
vi.mock('../lib/backend', () => ({ backend }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { GridFSView } from './GridFSView'
import { gridfsBuckets } from '../components/Sidebar'

const tab = { id: 't1', kind: 'gridfs', connId: 'c1', connName: 'Local', db: 'media', coll: 'images' } as Tab
const file = { id: 'ObjectId("65a1f0c2e4b0a1b2c3d4e5f6")', filename: 'logo.png', length: 2048, chunkSize: 261120, uploadDate: '2026-10-01T10:00:00Z', contentType: 'image/png' }

beforeEach(() => {
  vi.clearAllMocks()
  app.sessions = [{ ...app.sessions[0]!, readOnly: false }]
  backend.listGridFSBuckets.mockResolvedValue([
    { name: 'fs', files: 0, chunkBytes: 0 },
    { name: 'images', files: 1, chunkBytes: 2048 },
  ])
  backend.listGridFSFiles.mockResolvedValue({ files: [file], total: 1, page: 0 })
  backend.previewGridFSFile.mockResolvedValue({ kind: 'image', contentType: 'image/png', dataUrl: 'data:image/png;base64,AAAA', truncated: false })
  backend.chooseGridFSUploadFile.mockResolvedValue('/tmp/new.pdf')
  backend.startGridFSUpload.mockResolvedValue({ id: 'j1' })
  backend.deleteGridFSFile.mockResolvedValue(undefined)
})

describe('GridFS', () => {
  it('finds buckets from .files/.chunks pairs', () => {
    const c = (name: string) => ({ name }) as CollectionInfo
    expect(gridfsBuckets([c('fs.files'), c('fs.chunks'), c('photos.files'), c('orders'), c('a.chunks'), c('a.files')])).toEqual(['a', 'fs'])
  })

  it('lists files, previews, uploads as a job and deletes after typed confirmation', async () => {
    app.confirm.mockResolvedValue(true)
    render(<GridFSView tab={tab} />)
    fireEvent.click(await screen.findByText('logo.png'))
    expect(backend.listGridFSFiles).toHaveBeenCalledWith({ connId: 'c1', db: 'media', bucket: 'images', name: '', sort: 'uploadDate', page: 0, pageSize: 50 })
    expect(await screen.findByRole('img', { name: 'logo.png' })).toHaveAttribute('src', 'data:image/png;base64,AAAA')

    fireEvent.click(screen.getByRole('button', { name: 'Upload…' }))
    await waitFor(() => expect(backend.startGridFSUpload).toHaveBeenCalledWith('c1', 'media', 'images', '/tmp/new.pdf'))

    fireEvent.click(screen.getByRole('button', { name: 'Delete logo.png' }))
    await waitFor(() => expect(backend.deleteGridFSFile).toHaveBeenCalledWith('c1', 'media', 'images', file.id))
    expect(app.confirm).toHaveBeenCalledWith(expect.objectContaining({ typeToConfirm: 'logo.png', danger: true }))
  })

  it('offers no changes on read-only connections', async () => {
    app.sessions = [{ ...app.sessions[0]!, readOnly: true }]
    render(<GridFSView tab={tab} />)
    expect(await screen.findByText('logo.png')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Download logo.png' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Upload…' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Delete logo.png' })).toBeNull()
  })
})
