import { useEffect, useRef, useState } from 'react'
import { ErrorBanner, Spinner } from '../components/common'
import { backend, onStreamEvents, onStreamState } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { Tab } from '../lib/tabs'
import type { ChangeEvent } from '../lib/types'
import { useApp } from '../state/AppContext'

/** A tab keeps at most this many events (the oldest are dropped). */
export const MAX_EVENTS = 10000

/** Live change stream viewer for a collection, a database or the whole
 * deployment. Pausing keeps the resume token, so resuming continues from it. */
export function ChangesView({ tab }: { tab: Tab }) {
  const connId = tab.connId!
  const id = `changes-${tab.id}`
  const { toast } = useApp()
  const [match, setMatch] = useState('')
  const [fullDocument, setFullDocument] = useState(true)
  const [running, setRunning] = useState(false)
  const [starting, setStarting] = useState(false)
  const [events, setEvents] = useState<ChangeEvent[]>([])
  const [error, setError] = useState<AppError | null>(null)
  const [open, setOpen] = useState<number | null>(null)
  const token = useRef('')
  const seq = useRef(0)

  useEffect(() => {
    const offEvents = onStreamEvents((e) => {
      if (e.id !== id) return
      if (e.resumeToken) token.current = e.resumeToken
      setEvents((cur) => {
        const added = e.events.map((ev) => ({ ...ev, seq: ++seq.current }))
        const next = [...cur, ...added]
        return next.length > MAX_EVENTS ? next.slice(next.length - MAX_EVENTS) : next
      })
    })
    const offState = onStreamState((s) => {
      if (s.id !== id) return
      if (s.resumeToken) token.current = s.resumeToken
      setRunning(s.running)
      if (s.error) setError({ code: 'MONGO_ERROR', message: s.error })
    })
    return () => {
      offEvents()
      offState()
      void backend.stopChangeStream(id).catch(() => undefined)
    }
  }, [id])

  const start = async (resume: boolean) => {
    setStarting(true)
    setError(null)
    try {
      await backend.startChangeStream(id, {
        connId,
        db: tab.db ?? '',
        coll: tab.coll ?? '',
        match,
        fullDocument,
        resumeToken: resume ? token.current : '',
      })
      setRunning(true)
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setStarting(false)
    }
  }

  const pause = async () => {
    await backend.stopChangeStream(id).catch(() => undefined)
    setRunning(false)
  }

  const exportJSON = async () => {
    try {
      const path = await backend.exportTextFile('Export changes', `${tab.coll ?? tab.db ?? 'changes'}-changes.json`, JSON.stringify(events, null, 2))
      if (path) toast({ kind: 'success', title: `Exported ${events.length} changes` })
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const scope = tab.coll ? `${tab.db}.${tab.coll}` : tab.db ? `database ${tab.db}` : 'the whole deployment'
  const shown = [...events].reverse()
  return (
    <div className="view changes-view">
      <div className="view-header">
        <div className="breadcrumbs">
          <span>{tab.connName}</span>
          <span className="muted">· changes to {scope}</span>
        </div>
        <span className="spacer" />
        {running && <span className="badge">LIVE</span>}
        {running ? (
          <button onClick={() => void pause()}>Pause</button>
        ) : (
          <>
            {token.current && (
              <button className="primary" onClick={() => void start(true)} disabled={starting} title="Continue from where it stopped: nothing in between is missed">
                Resume
              </button>
            )}
            <button className={token.current ? '' : 'primary'} onClick={() => void start(false)} disabled={starting}>
              {starting ? <Spinner small /> : token.current ? 'Start fresh' : 'Start watching'}
            </button>
          </>
        )}
        <button onClick={() => setEvents([])} disabled={events.length === 0}>
          Clear
        </button>
        <button onClick={() => void exportJSON()} disabled={events.length === 0}>
          Export JSON…
        </button>
      </div>
      <div className="field-row changes-options">
        <label className="field">
          <span>Only changes matching</span>
          <input
            className="mono"
            value={match}
            onChange={(e) => setMatch(e.target.value)}
            placeholder={'{ operationType: "update", "fullDocument.status": "A" }'}
            disabled={running}
            aria-label="Change filter"
            spellCheck={false}
          />
        </label>
        <label className="check">
          <input type="checkbox" checked={fullDocument} onChange={(e) => setFullDocument(e.target.checked)} disabled={running} />
          Include the full document after updates
        </label>
      </div>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {events.length === 0 && !error && (
        <div className="empty-state">{running ? 'Waiting for changes…' : 'Start watching to see inserts, updates and deletes as they happen.'}</div>
      )}
      {events.length > 0 && (
        <div className="table-wrap">
          <table className="grid compact changes-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Operation</th>
                <th>Namespace</th>
                <th>Key</th>
                <th>Changed fields</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((ev) => (
                <tr key={ev.seq} className={open === ev.seq ? 'selected' : ''} onClick={() => setOpen(open === ev.seq ? null : ev.seq!)}>
                  <td className="small">{ev.time ? new Date(ev.time).toLocaleTimeString() : ''}</td>
                  <td>
                    <span className={`op-badge op-${ev.op}`}>{ev.op}</span>
                  </td>
                  <td className="mono small">{ev.ns}</td>
                  <td className="mono small">{ev.key}</td>
                  <td className="mono small">
                    {ev.updated?.map((f) => (
                      <span key={`u${f}`} className="change-better">
                        ~{f}{' '}
                      </span>
                    ))}
                    {ev.removed?.map((f) => (
                      <span key={`r${f}`} className="change-worse">
                        −{f}{' '}
                      </span>
                    ))}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {events.length >= MAX_EVENTS && <p className="muted small">Showing the last {MAX_EVENTS.toLocaleString()} changes.</p>}
        </div>
      )}
      {open !== null && events.find((e) => e.seq === open) && (
        <pre className="doc-text small changes-raw" aria-label="Change event">
          {events.find((e) => e.seq === open)!.raw}
        </pre>
      )}
    </div>
  )
}
