import { useCallback, useRef, useState } from 'react'
import { backend } from '../../lib/backend'
import { toAppError, type AppError } from '../../lib/errors'
import type { Doc, Page, QueryRequest } from '../../lib/types'
import { useApp } from '../../state/AppContext'

/**
 * Runs a collection query page by page. Paging, refresh, page size and count
 * reuse the last request that produced the results (or `fallback` before the
 * first one), so they show exactly what was run, not what is being edited.
 */
export function usePagedQuery(opId: string, countId: string, fallback: () => QueryRequest) {
  const { toastError } = useApp()
  const [page, setPage] = useState<Page | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [total, setTotal] = useState<number | null>(null)
  const [counting, setCounting] = useState(false)
  const lastReq = useRef<QueryRequest | null>(null)

  const execute = useCallback(
    async (req: QueryRequest) => {
      setLoading(true)
      setError(null)
      try {
        const p = await backend.runQuery(opId, req)
        if (lastReq.current?.filter !== req.filter || lastReq.current?.skip !== req.skip || lastReq.current?.limit !== req.limit) {
          setTotal(null)
        }
        lastReq.current = req
        setPage(p)
      } catch (err) {
        const e = toAppError(err)
        if (e.code !== 'CANCELED') setError(e)
      } finally {
        setLoading(false)
      }
    },
    [opId],
  )

  const current = () => lastReq.current ?? fallback()
  const goto = (n: number) => execute({ ...current(), page: n })
  const refresh = () => execute(current())
  const resize = (pageSize: number) => execute({ ...current(), page: 0, pageSize })

  const count = async () => {
    setCounting(true)
    try {
      setTotal(await backend.countQuery(countId, current()))
    } catch (err) {
      toastError(err, 'Count failed')
    } finally {
      setCounting(false)
    }
  }

  /** Puts a re-read document in place of its old version. */
  const replaceDocument = useCallback((nd: Doc) => setPage((p) => (p ? { ...p, documents: p.documents.map((d) => (d.id === nd.id ? nd : d)) } : p)), [])
  const removeDocument = useCallback((id: string) => setPage((p) => (p ? { ...p, documents: p.documents.filter((d) => d.id !== id) } : p)), [])
  /** After a write: the count is stale and the page is reloaded. */
  const changed = () => {
    setTotal(null)
    void refresh()
  }

  return { page, loading, error, setError, total, counting, lastReq, execute, goto, refresh, resize, count, replaceDocument, removeDocument, changed }
}
