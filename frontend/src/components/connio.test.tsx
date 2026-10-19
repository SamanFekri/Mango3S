import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ConnectionImportPreview, Profile } from '../lib/types'

const backend = vi.hoisted(() => ({
  exportConnections: vi.fn(),
  chooseConnectionsFile: vi.fn(),
  previewConnectionImport: vi.fn(),
  applyConnectionImport: vi.fn(),
}))
const app = vi.hoisted(() => ({ profiles: [] as Profile[], toast: vi.fn(), reloadProfiles: vi.fn() }))

vi.mock('../lib/backend', () => ({ backend }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { ConnectionExportDialog, ConnectionImportDialog } from './ConnectionIODialogs'

const prof = (id: string, name: string) => ({ id, name, uri: 'mongodb://h' }) as Profile

beforeEach(() => {
  vi.clearAllMocks()
  app.profiles = [prof('p1', 'Prod'), prof('p2', 'Local')]
  backend.exportConnections.mockResolvedValue({ path: '/x.json', count: 1, secretsIncluded: false })
})

describe('ConnectionExportDialog', () => {
  it('exports without secrets by default', async () => {
    render(<ConnectionExportDialog selected={app.profiles[0]} onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Export…' }))
    await waitFor(() => expect(backend.exportConnections).toHaveBeenCalledWith(['p1'], false, ''))
  })

  it('needs a matching passphrase of 10+ characters to include secrets', async () => {
    render(<ConnectionExportDialog selected={null} onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('checkbox', { name: /Include passwords/ }))
    expect(screen.getByText(/Anyone with this file and its passphrase/)).toBeInTheDocument()
    const go = screen.getByRole('button', { name: 'Export…' })
    fireEvent.change(screen.getByLabelText('Passphrase'), { target: { value: 'short' } })
    expect(go).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Passphrase'), { target: { value: 'long enough pass' } })
    fireEvent.change(screen.getByLabelText('Repeat passphrase'), { target: { value: 'long enough pas' } })
    expect(go).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Repeat passphrase'), { target: { value: 'long enough pass' } })
    fireEvent.click(go)
    await waitFor(() => expect(backend.exportConnections).toHaveBeenCalledWith([], true, 'long enough pass'))
  })
})

const preview = (over: Partial<ConnectionImportPreview> = {}): ConnectionImportPreview => ({
  path: '/in.json',
  format: 'mango3s',
  hasSecrets: true,
  secretsUnlocked: false,
  warnings: ['The file contains encrypted passwords.'],
  items: [
    { index: 0, name: 'Prod', folder: 'Clients', hosts: 'db1:27017', via: 'SSH bastion', secretKinds: [], conflict: 'id', existingId: 'p1', notes: [], error: '' },
    { index: 1, name: 'Local', folder: '', hosts: 'localhost', via: '', secretKinds: [], conflict: 'name', existingId: 'p2', notes: [], error: '' },
    { index: 2, name: 'Broken', folder: '', hosts: '', via: '', secretKinds: [], conflict: '', existingId: '', notes: [], error: 'Server: bad' },
  ],
  ...over,
})

describe('ConnectionImportDialog', () => {
  it('previews, unlocks secrets and applies the chosen actions', async () => {
    backend.chooseConnectionsFile.mockResolvedValue('/in.json')
    backend.previewConnectionImport.mockResolvedValueOnce(preview())
    backend.applyConnectionImport.mockResolvedValue({ added: 1, replaced: 1, skipped: 1, errors: [] })
    const onClose = vi.fn()
    render(<ConnectionImportDialog onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Choose file…' }))
    await screen.findByText('Server: bad')
    // Same connection: skipped by default; a name clash gets a new name.
    expect(screen.getByRole('combobox', { name: 'Action for Prod' })).toHaveValue('skip')
    expect(screen.getByRole('textbox', { name: 'Name for Local' })).toHaveValue('Local (imported)')
    expect(screen.getByRole('combobox', { name: 'Action for Broken' })).toBeDisabled()
    fireEvent.change(screen.getByRole('combobox', { name: 'Action for Prod' }), { target: { value: 'replace' } })

    backend.previewConnectionImport.mockResolvedValueOnce(preview({ secretsUnlocked: true }))
    fireEvent.change(screen.getByLabelText('Passphrase'), { target: { value: 'the passphrase' } })
    fireEvent.click(screen.getByRole('button', { name: 'Unlock' }))
    await screen.findByRole('button', { name: 'Unlocked' })
    expect(backend.previewConnectionImport).toHaveBeenLastCalledWith('/in.json', 'the passphrase')
    expect(screen.getByRole('combobox', { name: 'Action for Prod' })).toHaveValue('replace')
    fireEvent.click(screen.getByRole('button', { name: /^Import/ }))
    await waitFor(() =>
      expect(backend.applyConnectionImport).toHaveBeenCalledWith('/in.json', 'the passphrase', [
        { index: 0, action: 'replace', name: 'Prod', folder: 'Clients' },
        { index: 1, action: 'add', name: 'Local (imported)', folder: '' },
        { index: 2, action: 'skip', name: 'Broken', folder: '' },
      ]),
    )
    expect(app.reloadProfiles).toHaveBeenCalled()
    expect(onClose).toHaveBeenCalled()
  })
})
