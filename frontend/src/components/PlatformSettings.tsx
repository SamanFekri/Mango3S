import { useState } from 'react'
import { Spinner } from './common'
import { backend } from '../lib/backend'
import type { Settings } from '../lib/types'
import { useApp } from '../state/AppContext'

const stateLabel: Record<string, string> = {
  idle: 'Not checked yet',
  checking: 'Checking…',
  up_to_date: 'Up to date',
  update_available: 'Update available',
  downloading: 'Downloading an update…',
  ready_to_install: 'Update ready to install',
  installing: 'Installing…',
  error: 'Last check failed',
}

/** "Updates" settings: the automatic-update toggle and an on-demand check.
 * Mango3S registers with its update server by itself — there's nothing to
 * activate — and keeps working normally whether or not that server is reachable. */
export function PlatformSettings({ form, set }: { form: Settings; set: <K extends keyof Settings>(k: K, v: Settings[K]) => void }) {
  const { updateStatus, toast, toastError } = useApp()
  const [checking, setChecking] = useState(false)

  const checkNow = async () => {
    setChecking(true)
    try {
      const status = await backend.checkForUpdatesNow()
      if (status.state === 'up_to_date') {
        toast({ kind: 'info', title: 'You are up to date', message: `Version ${status.currentVersion}` })
      }
    } catch (err) {
      toastError(err, 'Could not check for updates')
    } finally {
      setChecking(false)
    }
  }

  return (
    <section className="settings-group" id="settings-platform">
      <h2>Updates</h2>
      <label className="check">
        <input type="checkbox" checked={form.autoUpdateEnabled} onChange={(e) => set('autoUpdateEnabled', e.target.checked)} />
        Automatically check for and download updates
      </label>
      <p className="muted small">
        Checks every 10 minutes while the app is open; you're always asked before an update is installed.
        "Check now" works even when this is off.
      </p>

      <div className="field">
        <span>Status</span>
        {updateStatus?.revoked ? (
          <div className="muted">
            Updates and announcements are paused for this installation by the Mango3S administrator.
            Everything else works as usual.
          </div>
        ) : (
          <div>
            {stateLabel[updateStatus?.state ?? 'idle'] ?? updateStatus?.state}
            {updateStatus?.latestVersion && (updateStatus.state === 'update_available' || updateStatus.state === 'ready_to_install')
              ? ` — version ${updateStatus.latestVersion}`
              : ''}
            {updateStatus?.lastCheckedAt && (
              <span className="muted small"> · last checked {new Date(updateStatus.lastCheckedAt).toLocaleString()}</span>
            )}
            {updateStatus?.error && (
              <div className="muted small">Couldn't reach the update server — it will retry automatically.</div>
            )}
          </div>
        )}
      </div>
      <div className="form-actions">
        <button onClick={checkNow} disabled={checking}>
          {checking && <Spinner small />} Check for updates now
        </button>
      </div>
    </section>
  )
}
