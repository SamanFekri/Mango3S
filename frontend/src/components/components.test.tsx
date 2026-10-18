import { fireEvent, render, screen, within } from '@testing-library/react'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { emptyBuilder, type BuilderState } from '../lib/queryBuilder'
import type { Page } from '../lib/types'
import { ErrorBanner } from './common'
import { highlight } from './highlight'
import { QueryBuilder } from './QueryBuilder'
import { ResultsView } from './ResultsView'

const page: Page = {
  page: 0,
  pageSize: 2,
  hasMore: true,
  durationMs: 12,
  documents: [
    { id: 'ObjectId("a")', text: '{\n  "_id": ObjectId("a"),\n  "name": "Ada"\n}', json: '{"_id":{"$oid":"a"},"name":"Ada"}' },
    { id: '"b"', text: '{\n  "_id": "b",\n  "age": 3\n}', json: '{"_id":"b","age":3}' },
  ],
}

function renderResults(extra: Partial<Parameters<typeof ResultsView>[0]> = {}) {
  const props = {
    page,
    loading: false,
    error: null,
    view: 'json' as const,
    onViewChange: vi.fn(),
    onPage: vi.fn(),
    pageSize: 2,
    onEdit: vi.fn(),
    onDelete: vi.fn(),
    ...extra,
  }
  render(<ResultsView {...props} />)
  return props
}

describe('ResultsView', () => {
  it('shows documents, range and pages forward', () => {
    const p = renderResults()
    expect(screen.getByText('ObjectId("a")')).toBeInTheDocument()
    expect(screen.getAllByText(/1–2/)).toHaveLength(2) // above and below the results
    const top = screen.getByRole('navigation', { name: 'Pagination' })
    expect(within(top).getByLabelText('Previous page')).toBeDisabled()
    fireEvent.click(within(top).getByLabelText('Next page'))
    expect(p.onPage).toHaveBeenCalledWith(1)
  })

  it('bulk-deletes selected documents', () => {
    const p = renderResults()
    fireEvent.click(screen.getByLabelText('Select document 1'))
    fireEvent.click(screen.getByLabelText('Select document 2'))
    fireEvent.click(screen.getByText('Delete 2 selected'))
    expect(p.onDelete).toHaveBeenCalledWith(page.documents)
  })

  it('hides write actions on read-only connections', () => {
    renderResults({ readOnly: true })
    expect(screen.queryByText('Edit')).not.toBeInTheDocument()
    expect(screen.queryByText('Delete')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Select document 1')).not.toBeInTheDocument()
  })

  it('renders the table view with union columns', () => {
    renderResults({ view: 'table' })
    for (const h of ['_id', 'name', 'age']) expect(screen.getByRole('columnheader', { name: h })).toBeInTheDocument()
    expect(screen.getByText('Ada')).toBeInTheDocument()
  })

  it('shows errors and a cancel button while loading', () => {
    const onCancel = vi.fn()
    renderResults({ loading: true, onCancel, error: { code: 'TIMEOUT', message: 'too slow' } })
    expect(screen.getByRole('alert')).toHaveTextContent('Timed out')
    expect(screen.getByRole('alert')).toHaveTextContent('too slow')
    fireEvent.click(screen.getByText('Cancel'))
    expect(onCancel).toHaveBeenCalled()
  })
})

describe('QueryBuilder', () => {
  function Harness({ onApply }: { onApply: (q: unknown, run: boolean) => void }) {
    const [s, setS] = useState<BuilderState>(emptyBuilder)
    return <QueryBuilder state={s} onChange={setS} fields={['age', 'name']} onApply={onApply} />
  }

  it('builds a filter from the form and applies it', () => {
    const onApply = vi.fn()
    render(<Harness onApply={onApply} />)
    fireEvent.change(screen.getByLabelText('Field'), { target: { value: 'age' } })
    fireEvent.change(screen.getByLabelText('Operator'), { target: { value: 'gt' } })
    fireEvent.change(screen.getByLabelText('Value'), { target: { value: '30' } })
    expect(screen.getByTitle('Generated filter')).toHaveTextContent('{ age: { $gt: 30 } }')
    fireEvent.click(screen.getByText('Apply & run'))
    expect(onApply).toHaveBeenCalledWith({ filter: '{ age: { $gt: 30 } }', sort: '', projection: '' }, true)
  })

  it('reports invalid input instead of producing a filter', () => {
    render(<Harness onApply={vi.fn()} />)
    fireEvent.change(screen.getByLabelText('Field'), { target: { value: 'age' } })
    fireEvent.change(screen.getByLabelText('Value type'), { target: { value: 'number' } })
    fireEvent.change(screen.getByLabelText('Value'), { target: { value: 'abc' } })
    expect(screen.getByText(/is not a number/)).toBeInTheDocument()
    expect(screen.getByText('Apply & run')).toBeDisabled()
  })
})

describe('ErrorBanner', () => {
  it('shows a friendly title and expandable detail', () => {
    render(<ErrorBanner error={{ code: 'UNREACHABLE', message: 'Could not reach', detail: 'dial tcp: refused' }} />)
    expect(screen.getByText('Server unreachable')).toBeInTheDocument()
    expect(screen.getByText('dial tcp: refused')).toBeInTheDocument()
  })
})

describe('highlight', () => {
  it('tokenizes keys, strings, numbers and helpers', () => {
    const { container } = render(<pre>{highlight('{"a": ObjectId("x"), "n": 1.5, "s": "v", "b": true}')}</pre>)
    expect(container.querySelectorAll('.tok-key')).toHaveLength(4)
    expect(container.querySelector('.tok-fn')).toHaveTextContent('ObjectId')
    expect(container.querySelector('.tok-num')).toHaveTextContent('1.5')
    expect(container.querySelector('.tok-kw')).toHaveTextContent('true')
    expect(container.textContent).toBe('{"a": ObjectId("x"), "n": 1.5, "s": "v", "b": true}')
  })
})

describe('pagination under the results', () => {
  it('repeats the pager below the results; both page the same way', () => {
    const p = renderResults()
    const bottom = screen.getByRole('navigation', { name: 'Pagination (bottom)' })
    expect(within(bottom).getByLabelText('Previous page')).toBeDisabled()
    expect(within(bottom).getByLabelText('Page')).toHaveTextContent('1')
    fireEvent.click(within(bottom).getByLabelText('Next page'))
    expect(p.onPage).toHaveBeenCalledWith(1)
    expect(screen.getByText('1–2', { selector: '.results-footer span' })).toBeInTheDocument()
  })

  it('is hidden when there are no documents', () => {
    renderResults({ page: { ...page, documents: [], hasMore: false } })
    expect(screen.queryByRole('navigation', { name: 'Pagination (bottom)' })).toBeNull()
  })
})
