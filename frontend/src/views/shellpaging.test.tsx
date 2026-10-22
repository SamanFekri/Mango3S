import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ShellOutput } from '../lib/types'

const backend = vi.hoisted(() => ({
  listCollections: vi.fn(async () => []),
  listHistory: vi.fn(async () => []),
  cancel: vi.fn(async () => true),
  runShell: vi.fn(),
  runShellPage: vi.fn(),
  sampleFields: vi.fn(async () => []),
}))
const app = vi.hoisted(() => ({ sessions: [], profiles: [], prompt: vi.fn(), toast: vi.fn(), toastError: vi.fn(), setTabDirty: vi.fn() }))
vi.mock('../lib/backend', () => ({ backend, onMongoshLine: () => () => undefined }))
vi.mock('../state/AppContext', () => ({ useApp: () => app, useTabDirty: () => undefined }))

import { ShellView } from './ShellView'

const doc = (n: number) => ({ id: String(n), text: `{ "_id": ${n} }`, json: `{"_id":{"$numberInt":"${n}"}}` })
const page = (p: number, ids: number[], hasMore: boolean): ShellOutput => ({
  statement: 'db.paged.find({})', kind: 'documents', documents: ids.map(doc), hasMore, durationMs: 1, pageable: true, page: p, pageSize: 2,
  text: `Documents ${p * 2 + 1}–${p * 2 + ids.length}. Use the page buttons to see more.`,
})

describe('shell results', () => {
  it('are paged: next and previous re-run the statement for that page', async () => {
    backend.runShell.mockResolvedValue({ outputs: [page(0, [0, 1], true)], currentDb: 'test' })
    backend.runShellPage.mockImplementation(async (_op: string, _c: string, _db: string, _st: string, p: number) => (p === 1 ? page(1, [2], false) : page(0, [0, 1], true)))
    render(<ShellView tab={{ id: 't1', kind: 'shell', title: 'Shell', connId: 'c1', connName: 'Local', db: 'test' }} />)
    fireEvent.click(screen.getByRole('button', { name: /Run/ }))
    expect(await screen.findByText(/Page 1/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Previous page' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await waitFor(() => expect(screen.getByText(/Page 2/)).toBeInTheDocument())
    expect(backend.runShellPage).toHaveBeenCalledWith(expect.any(String), 'c1', 'test', 'db.paged.find({})', 1)
    const shown = () => Array.from(document.querySelectorAll('.shell-entry pre.doc-text')).map((p) => p.textContent)
    expect(shown()).toEqual(['{ "_id": 2 }'])
    expect(screen.getByRole('button', { name: 'Next page' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Previous page' }))
    await waitFor(() => expect(screen.getByText(/Page 1/)).toBeInTheDocument())
  })
})
