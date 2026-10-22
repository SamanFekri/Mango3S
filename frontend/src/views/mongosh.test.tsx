import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const listeners = vi.hoisted(() => [] as ((l: { opId: string; stream: string; text: string }) => void)[])
const finish = vi.hoisted(() => ({ resolve: null as null | ((v: { exitCode: number; durationMs: number }) => void) }))
const backend = vi.hoisted(() => ({
  listCollections: vi.fn(async () => []),
  listHistory: vi.fn(async () => []),
  cancel: vi.fn(async () => true),
  runShell: vi.fn(),
  runShellPage: vi.fn(),
  sampleFields: vi.fn(async () => []),
  getMongoshAvailability: vi.fn(),
  runMongosh: vi.fn(),
  runJavaScript: vi.fn(),
}))
const app = vi.hoisted(() => ({ sessions: [], profiles: [], prompt: vi.fn(), toast: vi.fn(), toastError: vi.fn() }))
vi.mock('../lib/backend', () => ({
  backend,
  onMongoshLine: (cb: (l: { opId: string; stream: string; text: string }) => void) => {
    listeners.push(cb)
    return () => undefined
  },
}))
vi.mock('../state/AppContext', () => ({ useApp: () => app, useTabDirty: () => undefined }))

import { ShellView } from './ShellView'

const tab = { id: 't1', kind: 'shell' as const, title: 'Shell', connId: 'c1', connName: 'Local', db: 'shop', init: { text: 'for (let i = 0; i < 2; i++) print(i)' } }

beforeEach(() => {
  vi.clearAllMocks()
  listeners.length = 0
  backend.getMongoshAvailability.mockResolvedValue({ available: true, path: '/usr/bin/mongosh' })
  backend.runMongosh.mockImplementation(() => new Promise((r) => (finish.resolve = r)))
})

describe('shell › mongosh mode', () => {
  it('streams the output of a script run through mongosh', async () => {
    render(<ShellView tab={tab} />)
    fireEvent.click(screen.getByRole('radio', { name: 'mongosh' }))
    expect(await screen.findByText('/usr/bin/mongosh')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Run/ }))
    await waitFor(() => expect(backend.runMongosh).toHaveBeenCalledWith('shell-t1', 'c1', 'shop', 'for (let i = 0; i < 2; i++) print(i)'))
    expect(backend.runShell).not.toHaveBeenCalled()
    act(() => {
      listeners.forEach((l) => l({ opId: 'shell-t1', stream: 'stdout', text: '0' }))
      listeners.forEach((l) => l({ opId: 'shell-t1', stream: 'stdout', text: '1' }))
      listeners.forEach((l) => l({ opId: 'other-tab', stream: 'stdout', text: 'not mine' }))
    })
    expect(screen.getByLabelText('mongosh output')).toHaveTextContent(/0\s*1/)
    expect(screen.getByLabelText('mongosh output')).not.toHaveTextContent('not mine')
    await act(async () => finish.resolve!({ exitCode: 1, durationMs: 1200 }))
    expect(await screen.findByText(/exit code 1/)).toBeInTheDocument()
  })

  it('explains why mongosh can\'t be used and disables Run', async () => {
    backend.getMongoshAvailability.mockResolvedValue({ available: false, problem: 'This connection uses an SSH tunnel, which mongosh can\'t use.' })
    render(<ShellView tab={tab} />)
    fireEvent.click(screen.getByRole('radio', { name: 'mongosh' }))
    expect(await screen.findByText(/SSH tunnel/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Run/ })).toBeDisabled()
  })
  it('runs scripts in the built-in JavaScript engine', async () => {
    backend.runJavaScript.mockResolvedValue({ outputs: [{ statement: 'print', kind: 'text', text: '0', hasMore: false, durationMs: 1 }], currentDb: 'shop' })
    render(<ShellView tab={tab} />)
    fireEvent.click(screen.getByRole('radio', { name: 'JavaScript' }))
    fireEvent.click(screen.getByRole('button', { name: /Run/ }))
    await waitFor(() => expect(backend.runJavaScript).toHaveBeenCalledWith('shell-t1', 'c1', 'shop', 'for (let i = 0; i < 2; i++) print(i)'))
    expect(backend.runShell).not.toHaveBeenCalled()
    expect(backend.getMongoshAvailability).not.toHaveBeenCalled()
  })
})
