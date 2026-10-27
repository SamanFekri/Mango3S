import { useCallback, useEffect, useState } from 'react'
import { CodeEditor } from '../components/CodeEditor'
import { ErrorBanner, Modal, Segmented } from '../components/common'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { timeAgo } from '../lib/library'
import type { Task, TaskNotify, TaskRun, TaskSchedule, TaskType } from '../lib/types'
import { useApp } from '../state/AppContext'

const TEMPLATES: Record<string, object> = {
  export: { connId: '', db: '', coll: '', filter: '{}', path: '/path/to/exports/data-{date}.json', format: 'json', jsonMode: 'lines', canonical: true },
  compare: { source: { connId: '', db: '', coll: '' }, target: { connId: '', db: '', coll: '' }, filter: '', ignoreFields: [], ignoreFieldOrder: true, sampleSize: 10 },
  sync: {
    compare: { source: { connId: '', db: '', coll: '' }, target: { connId: '', db: '', coll: '' }, filter: '', ignoreFields: [], ignoreFieldOrder: true, sampleSize: 0 },
    sync: { insert: true, replace: true, delete: false },
  },
  maskCopy: { spec: { connId: '', db: '', coll: '', filter: '', mode: 'copy', targetColl: '', onError: 'fail' }, ruleSetId: '', replaceTarget: true },
  backup: {
    connId: '',
    backup: { dir: '/path/to/backups/nightly-{date}-{time}', gzip: true, selection: { databases: [], include: [], exclude: [], includeSystem: false, views: true }, query: '' },
  },
  chain: {
    steps: [
      { name: 'Export', type: 'export', params: { connId: '', db: '', coll: '', filter: '{}', path: '/path/to/exports/data-{date}.json', format: 'json', jsonMode: 'lines', canonical: true } },
      { name: 'Back up', type: 'backup', params: { connId: '', backup: { dir: '/path/to/backups/nightly-{date}-{time}', gzip: true, selection: { databases: [] } } } },
    ],
    continueOnError: false,
  },
}

interface Draft {
  id: string
  name: string
  type: string
  params: string
  schedule: TaskSchedule
  retry: { maxAttempts: number; backoffSeconds: number }
  misfire: 'skip' | 'runOnce'
  enabled: boolean
  notify: TaskNotify
  /** A new webhook URL to store; undefined keeps the stored one. */
  webhookUrl?: string
  /** Host of the stored webhook ('' = none). */
  webhookHost: string
}

function toDraft(t?: Task, type = 'export'): Draft {
  return {
    id: t?.id ?? '',
    name: t?.name ?? '',
    type: t?.type ?? type,
    params: JSON.stringify(t?.params ?? TEMPLATES[type], null, 2),
    schedule: t?.schedule ?? { kind: 'cron', everyMinutes: 60, cron: '0 2 * * *' },
    retry: t?.retry ?? { maxAttempts: 2, backoffSeconds: 60 },
    misfire: t?.misfire ?? 'skip',
    enabled: t?.enabled ?? true,
    notify: t?.notify ?? { on: 'never', desktop: true, webhook: false },
    webhookHost: '',
  }
}

const isApproved = (t: Task) => !!t.approvedAt && !!t.approvedHash

export function TasksView() {
  const { sessions, confirm, toast, toastError, settings } = useApp()
  const [tasks, setTasks] = useState<Task[]>([])
  const [types, setTypes] = useState<TaskType[]>([])
  const [runs, setRuns] = useState<TaskRun[]>([])
  const [draft, setDraft] = useState<Draft | null>(null)
  const [error, setError] = useState<AppError | null>(null)

  const load = useCallback(async () => {
    try {
      const [t, r] = await Promise.all([backend.listTasks(), backend.listTaskRuns('', 100)])
      setTasks(t)
      setRuns(r)
    } catch (err) {
      toastError(err, 'Could not load tasks')
    }
  }, [toastError])

  useEffect(() => {
    load()
    backend.listTaskTypes().then(setTypes).catch(() => undefined)
    const id = window.setInterval(load, 5000)
    return () => window.clearInterval(id)
  }, [load])

  // The stored webhook's host, for the task being edited.
  const editingId = draft?.id
  useEffect(() => {
    if (!editingId) return
    backend
      .taskWebhookHost(editingId)
      .then((host) => setDraft((d) => (d && d.id === editingId ? { ...d, webhookHost: host } : d)))
      .catch(() => undefined)
  }, [editingId])

  const save = async () => {
    if (!draft) return
    setError(null)
    let params: unknown
    try {
      params = JSON.parse(draft.params)
    } catch {
      setError({ code: 'INVALID_INPUT', message: 'Parameters must be valid JSON.' })
      return
    }
    try {
      const saved = await backend.saveTask({
        id: draft.id,
        name: draft.name,
        type: draft.type,
        params,
        schedule: draft.schedule,
        retry: draft.retry,
        misfire: draft.misfire,
        enabled: draft.enabled,
        notify: draft.notify,
      })
      if (draft.webhookUrl !== undefined) await backend.setTaskWebhook(saved.id, draft.webhookUrl)
      setDraft(null)
      load()
      toast({ kind: 'info', title: 'Task saved', message: 'Review and approve it before it can run.' })
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const act = async (f: () => Promise<unknown>, done?: string) => {
    try {
      await f()
      if (done) toast({ kind: 'success', title: done })
      load()
    } catch (err) {
      toastError(err)
    }
  }

  const approve = async (t: Task) => {
    const ok = await confirm({
      title: `Approve "${t.name}"`,
      message: (
        <>
          <p>
            Approving lets Mango3S run this <strong>{t.type}</strong> task on its schedule without asking again. Changing its parameters later revokes
            approval.
          </p>
          <pre className="doc-text small">{JSON.stringify(t.params, null, 2)}</pre>
        </>
      ),
      confirmLabel: 'Approve',
    })
    if (ok) act(() => backend.approveTask(t.id), 'Task approved')
  }

  const exportTasks = async () => {
    try {
      const json = await backend.exportTasks([])
      const path = await backend.exportTextFile('Export task definitions', 'mango3s-tasks.json', json)
      if (path) toast({ kind: 'success', title: `Exported ${tasks.length} task definition${tasks.length === 1 ? '' : 's'}`, message: 'Approvals are not included.' })
    } catch (err) {
      toastError(err, 'Export failed')
    }
  }
  const importTasks = async () => {
    try {
      const content = await backend.importTextFile('Import task definitions')
      if (!content) return
      const r = await backend.importTasks(content)
      await load()
      if (r.errors.length) toastError({ code: 'INVALID_INPUT', message: r.errors.join('\n') }, `Imported ${r.imported.length}; ${r.errors.length} failed`)
      else toast({ kind: 'success', title: `Imported ${r.imported.length} task${r.imported.length === 1 ? '' : 's'}`, message: 'Review and approve them before they run.' })
    } catch (err) {
      toastError(err, 'Import failed')
    }
  }

  return (
    <div className="view tasks-view">
      <div className="view-header">
        <h1>Scheduled tasks</h1>
        <span className="spacer" />
        {!settings?.schedulerEnabled && <span className="badge">scheduler off in Settings</span>}
        <button onClick={importTasks} title="Imported tasks start unapproved">
          Import…
        </button>
        <button onClick={exportTasks} disabled={tasks.length === 0}>
          Export…
        </button>
        <button className="primary" onClick={() => setDraft(toDraft())}>
          + New task
        </button>
      </div>
      <p className="muted small">
        Tasks run while Mango3S is open, only after you approve them. Failed runs retry with backoff; runs missed while the app was closed are skipped or run
        once at startup, per task. Connection IDs: {sessions.map((s) => `${s.name} = ${s.id}`).join(', ') || 'connect first to see IDs'}.
      </p>

      <table className="grid">
        <thead>
          <tr>
            <th>Task</th>
            <th>Type</th>
            <th>Schedule</th>
            <th>Next run</th>
            <th>Status</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {tasks.map((t) => (
            <tr key={t.id}>
              <td>{t.name}</td>
              <td>{t.type}</td>
              <td className="mono">{t.schedule.kind === 'cron' ? t.schedule.cron : `every ${t.schedule.everyMinutes} min`}</td>
              <td>{t.enabled ? new Date(t.nextRun).toLocaleString() : '—'}</td>
              <td>
                {isApproved(t) ? <span className="job-state succeeded">approved</span> : <span className="job-state failed">needs approval</span>}{' '}
                {!t.enabled && <span className="badge">disabled</span>}
              </td>
              <td className="row-buttons">
                {!isApproved(t) && (
                  <button className="small primary" onClick={() => approve(t)}>
                    Approve
                  </button>
                )}
                {isApproved(t) && (
                  <button className="small" onClick={() => act(() => backend.runTaskNow(t.id), 'Task started')}>
                    Run now
                  </button>
                )}
                <button className="small ghost" onClick={() => act(() => backend.setTaskEnabled(t.id, !t.enabled))}>
                  {t.enabled ? 'Disable' : 'Enable'}
                </button>
                <button className="small ghost" onClick={() => setDraft(toDraft(t))}>
                  Edit
                </button>
                {isApproved(t) && (
                  <button className="small ghost" onClick={() => act(() => backend.revokeTask(t.id), 'Approval revoked')}>
                    Revoke
                  </button>
                )}
                <button
                  className="small ghost danger-text"
                  onClick={async () => {
                    if (await confirm({ title: 'Delete task', message: `Delete "${t.name}"?`, danger: true, confirmLabel: 'Delete' })) act(() => backend.deleteTask(t.id))
                  }}
                >
                  Delete
                </button>
              </td>
            </tr>
          ))}
          {tasks.length === 0 && (
            <tr>
              <td colSpan={6} className="muted">
                No tasks yet.
              </td>
            </tr>
          )}
        </tbody>
      </table>

      <h3>Run history</h3>
      <table className="grid compact">
        <thead>
          <tr>
            <th>Task</th>
            <th>Trigger</th>
            <th>State</th>
            <th>Attempts</th>
            <th>When</th>
            <th>Result / errors</th>
          </tr>
        </thead>
        <tbody>
          {runs.map((r) => (
            <tr key={r.id}>
              <td>{r.taskName}</td>
              <td>{r.trigger}</td>
              <td>
                <span className={`job-state ${r.state}`}>{r.state}</span>
              </td>
              <td>{r.attempts}</td>
              <td>{timeAgo(r.startedAt)}</td>
              <td className="small">{r.result ?? r.errors.join(' · ')}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {draft && (
        <Modal
          title={draft.id ? 'Edit task' : 'New task'}
          onClose={() => setDraft(null)}
          wide
          footer={
            <>
              <span className="muted small">Saving never approves a task.</span>
              <span className="spacer" />
              <button onClick={() => setDraft(null)}>Cancel</button>
              <button className="primary" onClick={save}>
                Save
              </button>
            </>
          }
        >
          <ErrorBanner error={error} onDismiss={() => setError(null)} />
          <div className="field-row">
            <label className="field">
              <span>Name</span>
              <input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} autoFocus />
            </label>
            <label className="field">
              <span>Type</span>
              <select value={draft.type} onChange={(e) => setDraft({ ...draft, type: e.target.value, params: draft.id ? draft.params : JSON.stringify(TEMPLATES[e.target.value], null, 2) })}>
                {types.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.label}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <p className="muted small">{types.find((t) => t.id === draft.type)?.description}</p>
          <label className="field">
            <span>Parameters (JSON)</span>
            <CodeEditor value={draft.params} onChange={(v) => setDraft({ ...draft, params: v })} minHeight={200} maxHeight={320} ariaLabel="Task parameters" />
          </label>
          <div className="field-row">
            <div className="field">
              <span>Schedule</span>
              <Segmented
                label="Schedule kind"
                value={draft.schedule.kind}
                onChange={(kind) => setDraft({ ...draft, schedule: { ...draft.schedule, kind } })}
                options={[
                  { value: 'cron', label: 'Cron' },
                  { value: 'interval', label: 'Interval' },
                ]}
              />
            </div>
            {draft.schedule.kind === 'cron' ? (
              <label className="field">
                <span>Cron (minute hour day month weekday, local time)</span>
                <input className="mono" value={draft.schedule.cron} onChange={(e) => setDraft({ ...draft, schedule: { ...draft.schedule, cron: e.target.value } })} />
              </label>
            ) : (
              <label className="field">
                <span>Every N minutes</span>
                <input type="number" min={1} value={draft.schedule.everyMinutes} onChange={(e) => setDraft({ ...draft, schedule: { ...draft.schedule, everyMinutes: Number(e.target.value) } })} />
              </label>
            )}
          </div>
          <div className="field-row">
            <label className="field narrow">
              <span>Attempts</span>
              <input type="number" min={1} max={10} value={draft.retry.maxAttempts} onChange={(e) => setDraft({ ...draft, retry: { ...draft.retry, maxAttempts: Number(e.target.value) } })} />
            </label>
            <label className="field narrow">
              <span>Backoff (s)</span>
              <input type="number" min={0} value={draft.retry.backoffSeconds} onChange={(e) => setDraft({ ...draft, retry: { ...draft.retry, backoffSeconds: Number(e.target.value) } })} />
            </label>
            <label className="field">
              <span>If runs were missed while closed</span>
              <select value={draft.misfire} onChange={(e) => setDraft({ ...draft, misfire: e.target.value as Draft['misfire'] })}>
                <option value="skip">skip them (record as missed)</option>
                <option value="runOnce">run once at startup</option>
              </select>
            </label>
            <label className="check">
              <input type="checkbox" checked={draft.enabled} onChange={(e) => setDraft({ ...draft, enabled: e.target.checked })} />
              Enabled
            </label>
          </div>
          <h4>Notifications</h4>
          <div className="field-row">
            <label className="field">
              <span>Tell me</span>
              <select
                value={draft.notify.on}
                onChange={(e) => setDraft({ ...draft, notify: { ...draft.notify, on: e.target.value as TaskNotify['on'] } })}
                aria-label="When to notify"
              >
                <option value="never">never</option>
                <option value="failure">when a run fails</option>
                <option value="always">after every run</option>
              </select>
            </label>
            <label className="check">
              <input type="checkbox" checked={draft.notify.desktop} onChange={(e) => setDraft({ ...draft, notify: { ...draft.notify, desktop: e.target.checked } })} />
              Desktop notification
            </label>
            <label className="check">
              <input type="checkbox" checked={draft.notify.webhook} onChange={(e) => setDraft({ ...draft, notify: { ...draft.notify, webhook: e.target.checked } })} />
              Webhook
            </label>
          </div>
          {draft.notify.webhook && (
            <div className="field-row">
              <label className="field">
                <span>Webhook URL (Mango3S POSTs the run as JSON; kept in your system's credential store)</span>
                <input
                  className="mono"
                  value={draft.webhookUrl ?? ''}
                  onChange={(e) => setDraft({ ...draft, webhookUrl: e.target.value })}
                  placeholder={draft.webhookHost ? `stored: ${draft.webhookHost}/… (type to replace)` : 'https://hooks.example.com/…'}
                  aria-label="Webhook URL"
                  spellCheck={false}
                  autoComplete="off"
                />
              </label>
              {draft.webhookHost && draft.webhookUrl === undefined && (
                <button className="small ghost" onClick={() => setDraft({ ...draft, webhookUrl: '', webhookHost: '' })}>
                  Remove
                </button>
              )}
            </div>
          )}
          {draft.id && (draft.notify.desktop || draft.notify.webhook) && (
            <button
              className="small"
              title="Uses the saved settings: save first if you changed them"
              onClick={() => act(() => backend.testTaskNotification(draft.id), 'Test notification sent')}
            >
              Send a test notification
            </button>
          )}
        </Modal>
      )}
    </div>
  )
}
