import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ChangeEvent } from '../lib/types'

type Batch = { id: string; events: ChangeEvent[]; resumeToken: string }
const listeners = vi.hoisted(() => ({ events: [] as ((b: Batch) => void)[], state: [] as ((s: { id: string; running: boolean; resumeToken?: string }) => void)[] }))
const backend = vi.hoisted(() => ({ startChangeStream: vi.fn(), stopChangeStream: vi.fn(), exportTextFile: vi.fn() }))
const app = vi.hoisted(() => ({ toast: vi.fn() }))
vi.mock('../lib/backend', () => ({
  backend,
  onStreamEvents: (cb: (b: Batch) => void) => (listeners.events.push(cb), () => undefined),
  onStreamState: (cb: (s: { id: string; running: boolean }) => void) => (listeners.state.push(cb), () => undefined),
}))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { ChangesView, MAX_EVENTS } from './ChangesView'

const tab = { id: 't9', kind: 'changes' as const, title: '', connId: 'c1', connName: 'RS', db: 'shop', coll: 'orders' }
const ev = (op: string, key: string, extra: Partial<ChangeEvent> = {}): ChangeEvent => ({ op, ns: 'shop.orders', key, time: '2026-10-22T10:00:00Z', raw: `{op: "${op}"}`, ...extra })
const push = (events: ChangeEvent[], resumeToken = '{"_data": "T1"}') => act(() => listeners.events.forEach((l) => l({ id: 'changes-t9', events, resumeToken })))

beforeEach(() => {
  vi.clearAllMocks()
  listeners.events.length = 0
  listeners.state.length = 0
  backend.startChangeStream.mockResolvedValue(undefined)
  backend.stopChangeStream.mockResolvedValue(undefined)
})

describe('change stream viewer', () => {
  it('shows live changes, pauses and resumes from the token', async () => {
    render(<ChangesView tab={tab} />)
    fireEvent.change(screen.getByLabelText('Change filter'), { target: { value: '{operationType: "update"}' } })
    fireEvent.click(screen.getByRole('button', { name: 'Start watching' }))
    await waitFor(() =>
      expect(backend.startChangeStream).toHaveBeenCalledWith('changes-t9', { connId: 'c1', db: 'shop', coll: 'orders', match: '{operationType: "update"}', fullDocument: true, resumeToken: '' }),
    )
    push([ev('insert', '1'), ev('update', '1', { updated: ['status'], removed: ['note'] })])
    expect(screen.getByText('~status')).toHaveClass('change-better')
    expect(screen.getByText('−note')).toHaveClass('change-worse')

    fireEvent.click(screen.getByRole('button', { name: 'Pause' }))
    await waitFor(() => expect(backend.stopChangeStream).toHaveBeenCalledWith('changes-t9'))
    fireEvent.click(screen.getByRole('button', { name: 'Resume' }))
    await waitFor(() => expect(backend.startChangeStream).toHaveBeenLastCalledWith('changes-t9', expect.objectContaining({ resumeToken: '{"_data": "T1"}' })))
  })

  it(`keeps at most ${MAX_EVENTS} events`, async () => {
    render(<ChangesView tab={tab} />)
    fireEvent.click(screen.getByRole('button', { name: 'Start watching' }))
    await waitFor(() => expect(backend.startChangeStream).toHaveBeenCalled())
    push(Array.from({ length: MAX_EVENTS + 5 }, (_, i) => ev('insert', String(i))))
    expect(screen.getByText(`Showing the last ${MAX_EVENTS.toLocaleString()} changes.`)).toBeInTheDocument()
    expect(screen.queryByText('4', { selector: 'td' })).toBeNull()
    expect(screen.getByText(String(MAX_EVENTS + 4), { selector: 'td' })).toBeInTheDocument()
  })
})
