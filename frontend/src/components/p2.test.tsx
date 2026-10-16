import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AIProposal, Task } from '../lib/types'

const backend = vi.hoisted(() => ({
  getAIStatus: vi.fn(),
  previewAIContext: vi.fn(),
  generateAI: vi.fn(),
  previewAIWrite: vi.fn(),
  executeAIWrite: vi.fn(),
  listTasks: vi.fn(),
  listTaskRuns: vi.fn(),
  listTaskTypes: vi.fn(),
  approveTask: vi.fn(),
  runTaskNow: vi.fn(),
}))
const app = vi.hoisted(() => ({
  toast: vi.fn(),
  toastError: vi.fn(),
  dispatchTabs: vi.fn(),
  confirm: vi.fn(),
  sessions: [],
  settings: { schedulerEnabled: true },
  jobs: [],
  profiles: [],
}))

vi.mock('../lib/backend', () => ({ backend, newOpId: (p: string) => `${p}-1` }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { TasksView } from '../views/TasksView'
import { AIDialog } from './AIDialog'

const base: AIProposal = {
  id: 'prop1',
  kind: 'find',
  db: 'shop',
  coll: 'orders',
  filter: '{ status: "A" }',
  projection: '',
  sort: '{ _id: -1 }',
  limit: 20,
  pipeline: [],
  update: '',
  explanation: 'Active orders, newest first.',
  warnings: [],
  isWrite: false,
  requiresConfirmation: false,
  valid: true,
  validation: [{ name: 'parses', ok: true, detail: '' }],
  shell: 'db.orders.find({ status: "A" })',
  sent: { collection: 'orders', fields: [], samples: [] },
  model: 'claude-opus-5-5',
  inputTokens: 10,
  outputTokens: 20,
  durationMs: 5,
}

beforeEach(() => {
  vi.clearAllMocks()
  backend.getAIStatus.mockResolvedValue({ enabled: true, ready: true, provider: 'anthropic', providerName: 'Anthropic API', model: 'claude-opus-5-5', hasKey: true, keySource: 'stored', allowSamples: false, leavesDevice: true })
})

async function generate(p: AIProposal, onUse = vi.fn()) {
  backend.generateAI.mockResolvedValue(p)
  render(<AIDialog connId="c1" db="shop" coll="orders" onUse={onUse} onClose={vi.fn()} />)
  fireEvent.change(screen.getByRole('textbox'), { target: { value: 'active orders' } })
  await waitFor(() => expect(screen.getByRole('button', { name: /Generate/ })).toBeEnabled())
  fireEvent.click(screen.getByRole('button', { name: /Generate/ }))
  await screen.findByText(p.explanation)
  return onUse
}

describe('AIDialog', () => {
  it('hands a read proposal to the editor without running anything', async () => {
    const onUse = await generate(base)
    expect(backend.generateAI).toHaveBeenCalledWith('ai-1', expect.objectContaining({ prompt: 'active orders', allowWrites: false }))
    fireEvent.click(screen.getByRole('button', { name: 'Load into query editor' }))
    expect(onUse).toHaveBeenCalledWith(base)
    expect(backend.previewAIWrite).not.toHaveBeenCalled()
    expect(backend.executeAIWrite).not.toHaveBeenCalled()
  })

  it('does not offer to use an invalid proposal', async () => {
    await generate({ ...base, valid: false, validation: [{ name: 'server validation', ok: false, detail: 'unknown operator $foo' }] })
    expect(screen.queryByRole('button', { name: 'Load into query editor' })).toBeNull()
    expect(screen.getByText('unknown operator $foo')).toBeInTheDocument()
  })

  it('runs a write only after the target is typed', async () => {
    const write = { ...base, kind: 'deleteMany' as const, isWrite: true, requiresConfirmation: true, shell: 'db.orders.deleteMany({ status: "X" })' }
    await generate(write)
    expect(screen.queryByRole('button', { name: 'Load into query editor' })).toBeNull()
    expect(backend.executeAIWrite).not.toHaveBeenCalled()

    backend.previewAIWrite.mockResolvedValue({ proposalId: 'prop1', shell: write.shell, affected: 3, sampleIds: ['1'], confirmText: 'shop.orders' })
    fireEvent.click(screen.getByRole('button', { name: 'Review write…' }))
    const execute = await screen.findByRole('button', { name: 'Execute' })
    expect(execute).toBeDisabled()

    const confirmBox = screen.getByLabelText(/type shop.orders to confirm/)
    fireEvent.change(confirmBox, { target: { value: 'shop.order' } })
    expect(execute).toBeDisabled()
    fireEvent.change(confirmBox, { target: { value: 'shop.orders' } })
    expect(execute).toBeEnabled()

    backend.executeAIWrite.mockResolvedValue({ matched: 3, modified: 0, deleted: 3, backupPath: '/b/1.ndjson' })
    fireEvent.click(execute)
    await waitFor(() => expect(backend.executeAIWrite).toHaveBeenCalledWith('prop1', 'shop.orders'))
    expect(await screen.findByText('/b/1.ndjson')).toBeInTheDocument()
  })

  it('cannot generate while AI is disabled', async () => {
    backend.getAIStatus.mockResolvedValue({ enabled: false, ready: true, provider: 'anthropic', providerName: 'Anthropic API', model: 'm', hasKey: true, keySource: 'stored', allowSamples: false })
    render(<AIDialog connId="c1" db="shop" coll="orders" onUse={vi.fn()} onClose={vi.fn()} />)
    expect(await screen.findByText(/AI assistance is off/)).toBeInTheDocument()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'x' } })
    expect(screen.getByRole('button', { name: /Generate/ })).toBeDisabled()
  })
})

describe('TasksView', () => {
  const task: Task = {
    id: 't1',
    name: 'Nightly export',
    type: 'export',
    params: { db: 'shop' },
    schedule: { kind: 'cron', everyMinutes: 0, cron: '0 2 * * *' },
    enabled: true,
    retry: { maxAttempts: 2, backoffSeconds: 60 },
    misfire: 'skip',
    approvedHash: '',
    nextRun: new Date().toISOString(),
    createdAt: '',
    updatedAt: '',
  }

  beforeEach(() => {
    backend.listTasks.mockResolvedValue([task])
    backend.listTaskRuns.mockResolvedValue([])
    backend.listTaskTypes.mockResolvedValue([{ id: 'export', label: 'Export', description: '' }])
    backend.approveTask.mockResolvedValue(task)
  })

  it('requires approval before a task can run', async () => {
    render(<TasksView />)
    expect(await screen.findByText('needs approval')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Run now' })).toBeNull()

    app.confirm.mockResolvedValueOnce(false)
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() => expect(app.confirm).toHaveBeenCalled())
    expect(backend.approveTask).not.toHaveBeenCalled()

    app.confirm.mockResolvedValueOnce(true)
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() => expect(backend.approveTask).toHaveBeenCalledWith('t1'))
  })

  it('offers Run now once approved', async () => {
    backend.listTasks.mockResolvedValue([{ ...task, approvedHash: 'h', approvedAt: new Date().toISOString() }])
    render(<TasksView />)
    expect(await screen.findByText('approved')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Run now' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
  })
})

describe('AIDialog with other providers', () => {
  it('explains what is missing for the chosen provider and blocks generating', async () => {
    backend.getAIStatus.mockResolvedValue({ enabled: true, ready: false, provider: 'claudeCode', providerName: 'Claude Code', detail: 'Installed but not signed in. Run `claude` once and sign in.', model: '', hasKey: false, keySource: 'none', allowSamples: false, leavesDevice: true })
    render(<AIDialog connId="c1" db="shop" coll="orders" onUse={vi.fn()} onClose={vi.fn()} />)
    expect(await screen.findByText(/Claude Code is not ready: Installed but not signed in/)).toBeInTheDocument()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'x' } })
    expect(screen.getByRole('button', { name: /Generate/ })).toBeDisabled()
  })

  it('uses a ready local model like any other provider', async () => {
    backend.getAIStatus.mockResolvedValue({ enabled: true, ready: true, provider: 'local', providerName: 'Local model', model: 'qwen2.5-coder:7b', hasKey: false, keySource: 'none', allowSamples: false, leavesDevice: false })
    await generate({ ...base, model: 'qwen2.5-coder:7b' })
    expect(screen.getByRole('button', { name: 'Load into query editor' })).toBeInTheDocument()
  })
})
