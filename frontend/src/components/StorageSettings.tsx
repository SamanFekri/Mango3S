import { useEffect, useState } from 'react'
import { Spinner } from './common'
import { backend } from '../lib/backend'
import { formatBytes } from '../lib/format'
import type { StorageUsage } from '../lib/types'
import { useApp } from '../state/AppContext'

/** Settings › Diagnostics › Storage: what Mango3S keeps on this computer, and
 * clearing the parts that are safe to lose. Connections, settings, saved
 * queries, tasks and backups are never removed from here. */
export function StorageSettings() {
  const { confirm, toast, toastError } = useApp()
  const [usage, setUsage] = useState<StorageUsage | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [busy, setBusy] = useState(false)

  const load = (preselect: boolean) =>
    backend
      .getStorageUsage()
      .then((u) => {
        setUsage(u)
        if (preselect) setSelected(new Set(u.items.filter((i) => i.recommended && i.bytes > 0).map((i) => i.id)))
      })
      .catch(() => undefined)

  useEffect(() => {
    load(true)
  }, [])

  if (!usage) return null
  const chosen = usage.items.filter((i) => selected.has(i.id))
  const chosenBytes = chosen.reduce((n, i) => n + i.bytes, 0)

  const toggle = (id: string) =>
    setSelected((s) => {
      const next = new Set(s)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const clear = async () => {
    const ok = await confirm({
      title: 'Clear selected data?',
      message: `This removes ${chosen.map((i) => i.label.toLowerCase()).join(', ')} (${formatBytes(chosenBytes)}). Your connections, settings, saved queries and backups are kept.`,
      confirmLabel: 'Clear',
      danger: chosen.some((i) => !i.recommended),
    })
    if (!ok) return
    setBusy(true)
    try {
      const res = await backend.clearStorage(chosen.map((i) => i.id))
      setUsage(res.usage)
      setSelected(new Set())
      toast({ kind: 'success', title: `Freed ${formatBytes(res.freedBytes)}`, message: res.skipped?.join(' ') || undefined })
    } catch (err) {
      toastError(err, 'Could not clear storage')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="settings-group storage-settings" id="settings-storage">
      <h2>Storage</h2>
      <p className="small muted">
        Mango3S keeps its data in <span className="mono">{usage.configDir}</span>. Clear logs, caches and temporary files here at any time; your
        connections, settings, saved queries and backups are never removed.
      </p>
      <div className="storage-list">
        {usage.items.map((it) => (
          <label key={it.id} className={`storage-item${it.clearable ? '' : ' info'}`}>
            <input type="checkbox" checked={selected.has(it.id)} disabled={!it.clearable || busy} onChange={() => toggle(it.id)} aria-label={it.label} />
            <span className="storage-text">
              <strong>{it.label}</strong>
              <span className="small muted">{it.description}</span>
            </span>
            <span className="mono small storage-size">{formatBytes(it.bytes)}</span>
          </label>
        ))}
      </div>
      <div className="form-actions left">
        <button className="danger" onClick={clear} disabled={busy || chosen.length === 0}>
          {busy && <Spinner small />} Clear selected{chosen.length > 0 ? ` (${formatBytes(chosenBytes)})` : ''}
        </button>
        <button onClick={() => load(false)} disabled={busy}>
          Refresh
        </button>
      </div>
    </section>
  )
}
