import { useEffect, useRef, useState } from 'react'
import { backend } from '../lib/backend'
import type { Endpoint, Job, ValidationCheck } from '../lib/types'
import { useApp } from '../state/AppContext'
import { JobRow } from './JobsPanel'

/** The live state of a background job, or undefined until it is known. */
export function useJob(jobId: string | null | undefined): Job | undefined {
  const { jobs } = useApp()
  return jobId ? jobs.find((j) => j.id === jobId) : undefined
}

/** Calls onDone once when the job reaches a final state. */
export function useJobDone(jobId: string | null | undefined, onDone: (j: Job) => void) {
  const job = useJob(jobId)
  const fired = useRef<string | null>(null)
  const cb = useRef(onDone)
  cb.current = onDone
  useEffect(() => {
    if (job && ['succeeded', 'failed', 'canceled'].includes(job.state) && fired.current !== job.id) {
      fired.current = job.id
      cb.current(job)
    }
  }, [job])
  return job
}

export function JobStatus({ jobId }: { jobId: string | null }) {
  const job = useJob(jobId)
  if (!job) return null
  return (
    <ul className="job-list inline-job">
      <JobRow job={job} />
    </ul>
  )
}

/** Connection + database + collection picker with server-side suggestions. */
export function EndpointPicker({
  label,
  value,
  onChange,
  collection = true,
}: {
  label: string
  value: Endpoint
  onChange: (e: Endpoint) => void
  collection?: boolean
}) {
  const { sessions } = useApp()
  const [dbs, setDbs] = useState<string[]>([])
  const [colls, setColls] = useState<string[]>([])
  const id = useRef(`ep-${Math.random().toString(36).slice(2)}`).current

  useEffect(() => {
    if (!value.connId) return
    backend
      .listDatabases(value.connId)
      .then((l) => setDbs(l.map((d) => d.name)))
      .catch(() => setDbs([]))
  }, [value.connId])
  useEffect(() => {
    if (!value.connId || !value.db || !collection) return
    backend
      .listCollections(value.connId, value.db)
      .then((l) => setColls(l.map((c) => c.name)))
      .catch(() => setColls([]))
  }, [value.connId, value.db, collection])

  return (
    <fieldset className="endpoint">
      <legend>{label}</legend>
      <label className="field">
        <span>Connection</span>
        <select value={value.connId} onChange={(e) => onChange({ ...value, connId: e.target.value })}>
          <option value="">Choose…</option>
          {sessions.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
              {s.readOnly ? ' (read-only)' : ''}
            </option>
          ))}
        </select>
      </label>
      <label className="field">
        <span>Database</span>
        <input list={`${id}-dbs`} value={value.db} onChange={(e) => onChange({ ...value, db: e.target.value })} spellCheck={false} />
        <datalist id={`${id}-dbs`}>
          {dbs.map((d) => (
            <option key={d} value={d} />
          ))}
        </datalist>
      </label>
      {collection && (
        <label className="field">
          <span>Collection</span>
          <input list={`${id}-colls`} value={value.coll} onChange={(e) => onChange({ ...value, coll: e.target.value })} spellCheck={false} />
          <datalist id={`${id}-colls`}>
            {colls.map((c) => (
              <option key={c} value={c} />
            ))}
          </datalist>
        </label>
      )}
    </fieldset>
  )
}

/** Type-to-confirm input for destructive actions. */
export function ConfirmInput({ text, value, onChange, action }: { text: string; value: string; onChange: (v: string) => void; action: string }) {
  if (!text) return null
  return (
    <label className="field confirm-field">
      <span>
        {action} — type <code>{text}</code> to confirm
      </span>
      <input value={value} onChange={(e) => onChange(e.target.value)} spellCheck={false} autoComplete="off" />
    </label>
  )
}

export function Checks({ checks }: { checks: ValidationCheck[] }) {
  return (
    <ul className="checks">
      {checks.map((c, i) => (
        <li key={i} className={c.ok ? 'check-ok' : 'check-fail'}>
          <span aria-hidden>{c.ok ? '✓' : '✗'}</span> <strong>{c.name}</strong> <span className="muted">{c.detail}</span>
        </li>
      ))}
    </ul>
  )
}

export function Stat({ label, value, tone }: { label: string; value: number | string; tone?: 'good' | 'warn' | 'bad' }) {
  return (
    <div className={`metric ${tone ? 'metric-' + tone : ''}`}>
      <div className="metric-value">{value}</div>
      <div className="metric-label">{label}</div>
    </div>
  )
}
