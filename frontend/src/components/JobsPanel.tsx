import { useEffect, useRef, useState } from 'react'
import { backend } from '../lib/backend'
import { formatBytes, formatCount, formatDuration } from '../lib/format'
import type { Job } from '../lib/types'
import { useApp } from '../state/AppContext'

export function jobProgress(j: Job): { percent: number | null; label: string } {
  const amount = (n: number) => (j.unit === 'bytes' ? formatBytes(n) : formatCount(n))
  if (j.total > 0) {
    const percent = Math.min(100, Math.round((100 * j.done) / j.total))
    return { percent, label: `${amount(j.done)} / ${amount(j.total)}` }
  }
  return { percent: null, label: j.done > 0 ? amount(j.done) : '' }
}

function elapsed(j: Job): string {
  if (!j.startedAt || j.startedAt.startsWith('0001')) return ''
  const end = j.finishedAt && !j.finishedAt.startsWith('0001') ? new Date(j.finishedAt) : new Date()
  return formatDuration(Math.max(0, end.getTime() - new Date(j.startedAt).getTime()))
}

/** A job's log: collapsed by default, follows new lines while open. */
function JobLog({ job }: { job: Job }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLPreElement>(null)
  const lines = job.log ?? []
  useEffect(() => {
    if (open && ref.current) ref.current.scrollTop = ref.current.scrollHeight
  }, [open, lines.length])
  return (
    <div>
      <div className="job-log-actions">
        <button className="small ghost" aria-expanded={open} onClick={() => setOpen((o) => !o)}>
          {open ? 'Hide log' : `Show log (${lines.length})`}
        </button>
        {open && (
          <button className="small ghost" onClick={() => void navigator.clipboard?.writeText(lines.join('\n'))}>
            Copy log
          </button>
        )}
      </div>
      {open && (
        <pre className="job-log" ref={ref} aria-label={`Log of ${job.title}`}>
          {lines.join('\n')}
        </pre>
      )}
    </div>
  )
}

export function JobRow({ job }: { job: Job }) {
  const { percent, label } = jobProgress(job)
  const active = job.state === 'running' || job.state === 'queued'
  return (
    <li className={`job job-${job.state}`}>
      <div className="job-head">
        <span className="job-title" title={job.title}>
          {job.title}
        </span>
        <span className={`job-state ${job.state}`}>{job.state}</span>
        {active && (
          <button className="small ghost" onClick={() => backend.cancelJob(job.id)}>
            Cancel
          </button>
        )}
      </div>
      {active && (
        <div className="progress" role="progressbar" aria-valuenow={percent ?? undefined} aria-valuemin={0} aria-valuemax={100}>
          <div className={percent === null ? 'progress-bar indeterminate' : 'progress-bar'} style={{ width: `${percent ?? 30}%` }} />
        </div>
      )}
      <div className="job-meta muted small">
        {[label, elapsed(job), active ? job.message : job.result].filter(Boolean).join(' · ')}
      </div>
      {job.error && job.state !== 'canceled' && <div className="error-text">{job.error}</div>}
      {!!job.log?.length && <JobLog job={job} />}
    </li>
  )
}

/** Status bar button with a popover listing background jobs. */
export function JobsIndicator() {
  const { jobs, refreshJobs } = useApp()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const running = jobs.filter((j) => j.state === 'running' || j.state === 'queued')

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    window.addEventListener('mousedown', onDown)
    return () => window.removeEventListener('mousedown', onDown)
  }, [open])

  // Elapsed times tick while the panel is open.
  const [, setTick] = useState(0)
  useEffect(() => {
    if (!open || running.length === 0) return
    const id = window.setInterval(() => setTick((t) => t + 1), 1000)
    return () => window.clearInterval(id)
  }, [open, running.length])

  if (jobs.length === 0) return null
  const first = running[0]
  const p = first ? jobProgress(first) : null
  return (
    <div className="jobs-indicator" ref={ref}>
      <button className="statusbar-btn" onClick={() => setOpen((o) => !o)} aria-expanded={open}>
        {running.length > 0 ? (
          <>
            <span className="spinner small" aria-hidden />
            {running.length} job{running.length === 1 ? '' : 's'} running{p?.percent != null ? ` · ${p.percent}%` : ''}
          </>
        ) : (
          'Jobs'
        )}
      </button>
      {open && (
        <div className="jobs-popover" role="dialog" aria-label="Background jobs">
          <div className="jobs-popover-head">
            <strong>Background jobs</strong>
            <span className="spacer" />
            <button
              className="small ghost"
              onClick={async () => {
                await backend.clearFinishedJobs()
                refreshJobs()
              }}
            >
              Clear finished
            </button>
          </div>
          <ul className="job-list">
            {jobs.map((j) => (
              <JobRow key={j.id} job={j} />
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
