import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { BackupInspection, Job, SessionInfo } from '../lib/types'

const backend = vi.hoisted(() => ({
  backupToolsStatus: vi.fn(),
  listDatabases: vi.fn(),
  chooseFile: vi.fn(),
  chooseSaveFile: vi.fn(),
  startBackup: vi.fn(),
  inspectBackup: vi.fn(),
  startRestoreBackup: vi.fn(),
  cancelJob: vi.fn(),
}))
const app = vi.hoisted(() => ({ jobs: [] as Job[] }))

vi.mock('../lib/backend', () => ({ backend, newOpId: (p: string) => `${p}-1` }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { BackupDialog, RestoreDialog } from './BackupDialogs'
import { JobRow } from './JobsPanel'

const conn: SessionInfo = { id: 'c1', name: 'Prod', readOnly: false, serverVersion: '8.0.1', topology: 'standalone', connectedAt: '' }
const okTool = (name: string) => ({ name, purpose: '', path: `/bin/${name}`, source: 'PATH', version: '100.16.1', status: 'ok', message: '' })
const job = (over: Partial<Job> = {}): Job => ({
  id: 'j1',
  kind: 'backup',
  title: 'Back up Prod',
  state: 'running',
  done: 0,
  total: 0,
  unit: 'docs',
  message: '',
  createdAt: '',
  startedAt: '',
  finishedAt: '',
  ...over,
})

beforeEach(() => {
  vi.clearAllMocks()
  app.jobs = []
  backend.backupToolsStatus.mockResolvedValue({ dump: okTool('mongodump'), restore: okTool('mongorestore'), blocked: '' })
  backend.listDatabases.mockResolvedValue([{ name: 'shop' }, { name: 'crm' }, { name: 'local' }])
  backend.startBackup.mockResolvedValue(job())
  backend.startRestoreBackup.mockResolvedValue(job({ kind: 'restore' }))
})

describe('BackupDialog', () => {
  it('backs up the chosen databases into a new folder', async () => {
    backend.chooseFile.mockResolvedValue('/backups')
    render(<BackupDialog conn={conn} db="shop" onClose={vi.fn()} />)
    expect(await screen.findByRole('checkbox', { name: 'shop' })).toBeChecked()
    expect(screen.queryByRole('checkbox', { name: 'local' })).toBeNull()
    fireEvent.click(screen.getByRole('checkbox', { name: 'crm' }))
    expect(screen.getByRole('button', { name: 'Start backup' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Choose…' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Start backup' })).toBeEnabled())
    fireEvent.change(screen.getByRole('textbox', { name: 'Backup name' }), { target: { value: 'nightly' } })
    fireEvent.click(screen.getByRole('button', { name: 'Start backup' }))
    await waitFor(() => expect(backend.startBackup).toHaveBeenCalled())
    const r = backend.startBackup.mock.calls[0][0]
    expect(r.engine).toBe('builtin')
    expect(r.builtin).toMatchObject({ dir: '/backups/nightly', gzip: true, selection: { databases: ['shop', 'crm'], views: true, includeSystem: false } })
  })

  it('explains why the archive format is unavailable', async () => {
    backend.backupToolsStatus.mockResolvedValue({
      dump: { ...okTool('mongodump'), status: 'missing', path: '', message: 'Not found. Install the MongoDB Database Tools from https://example' },
      restore: okTool('mongorestore'),
      blocked: '',
    })
    render(<BackupDialog conn={conn} onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Archive file (mongodump)' }))
    expect(await screen.findByText(/Install the MongoDB Database Tools/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Start backup' })).toBeDisabled()
  })

  it('archives one database with its users and roles', async () => {
    backend.chooseSaveFile.mockResolvedValue('/b/shop.archive.gz')
    render(<BackupDialog conn={conn} db="shop" onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Archive file (mongodump)' }))
    await screen.findByText(/Using mongodump 100.16.1/)
    fireEvent.click(screen.getByRole('button', { name: 'Save as…' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Start backup' })).toBeEnabled())
    fireEvent.click(screen.getByRole('checkbox', { name: /users and roles/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Start backup' }))
    await waitFor(() => expect(backend.startBackup).toHaveBeenCalled())
    expect(backend.startBackup.mock.calls[0][0]).toMatchObject({ engine: 'tools', tool: { archive: '/b/shop.archive.gz', db: 'shop', usersAndRoles: true, gzip: true } })
  })
})

const inspection: BackupInspection = {
  dir: '/backups/nightly',
  manifest: { format: 'mango3s-backup', version: 1, created: '2026-10-19T08:00:00Z', tool: 'mango3s', source: 'Prod', gzip: true, namespaces: [] },
  entries: [
    { db: 'shop', coll: 'orders', type: 'collection', count: 10, bytes: 2048, indexes: 2, gzip: true, expected: 10 },
    { db: 'shop', coll: 'big', type: 'view', count: 0, bytes: 0, indexes: 0, gzip: true, expected: -1 },
  ],
  warnings: ['shop.x has no .metadata.json: indexes and collection options will not be restored.'],
}

describe('RestoreDialog', () => {
  it('restores selected namespaces into another database', async () => {
    backend.chooseFile.mockResolvedValue('/backups/nightly')
    backend.inspectBackup.mockResolvedValue(inspection)
    render(<RestoreDialog conn={conn} db="shop_copy" onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Choose…' }))
    expect(await screen.findByText(/has no .metadata.json/)).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Target database for shop' })).toHaveValue('shop_copy')
    fireEvent.click(screen.getByRole('checkbox', { name: 'shop.big' }))
    fireEvent.click(screen.getByRole('button', { name: 'Start restore' }))
    await waitFor(() => expect(backend.startRestoreBackup).toHaveBeenCalled())
    const [r, confirm] = backend.startRestoreBackup.mock.calls[0]
    expect(confirm).toBe('')
    expect(r.builtin).toMatchObject({ dir: '/backups/nightly', mode: 'insert', renames: [{ from: 'shop.*', to: 'shop_copy.*' }], selection: { include: ['shop.orders'] } })
  })

  it('needs the connection name typed before dropping collections', async () => {
    backend.chooseFile.mockResolvedValue('/backups/nightly')
    backend.inspectBackup.mockResolvedValue(inspection)
    render(<RestoreDialog conn={conn} onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Choose…' }))
    await screen.findByText('shop.orders')
    fireEvent.click(screen.getByRole('radio', { name: 'Drop collections first' }))
    const go = screen.getByRole('button', { name: 'Drop and restore' })
    expect(go).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox', { name: /type Prod to confirm/ }), { target: { value: 'Prod' } })
    expect(go).toBeEnabled()
    fireEvent.click(go)
    await waitFor(() => expect(backend.startRestoreBackup).toHaveBeenCalledWith(expect.objectContaining({ builtin: expect.objectContaining({ mode: 'drop' }) }), 'Prod'))
  })

  it('is disabled on read-only connections', async () => {
    render(<RestoreDialog conn={{ ...conn, readOnly: true }} onClose={vi.fn()} />)
    expect(screen.getByText(/This connection is read-only/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Start restore' })).toBeDisabled()
  })
})

describe('JobRow log', () => {
  it('shows and copies the log on demand', async () => {
    const writeText = vi.fn()
    Object.assign(navigator, { clipboard: { writeText } })
    render(
      <ul>
        <JobRow job={job({ log: ['08:00:00 Backing up 2 namespaces', '08:00:01 shop.orders: 10 documents, 2 indexes'] })} />
      </ul>,
    )
    expect(screen.queryByLabelText(/Log of/)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Show log (2)' }))
    expect(within(screen.getByLabelText('Log of Back up Prod')).getByText(/shop.orders: 10 documents/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    expect(writeText).toHaveBeenCalledWith('08:00:00 Backing up 2 namespaces\n08:00:01 shop.orders: 10 documents, 2 indexes')
  })
})
