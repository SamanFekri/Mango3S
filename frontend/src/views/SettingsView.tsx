import { useEffect, useState } from 'react'
import { Spinner } from '../components/common'
import { KeymapSettings } from '../components/KeymapSettings'
import { AISettings, MCPSettings } from '../components/P2Settings'
import type { Tab } from '../lib/tabs'
import type { Settings } from '../lib/types'
import { useApp } from '../state/AppContext'

export function SettingsView({ tab }: { tab?: Tab }) {
  const { settings, saveSettings, appInfo, toast, toastError } = useApp()
  const [form, setForm] = useState<Settings | null>(settings)
  const [busy, setBusy] = useState(false)

  useEffect(() => setForm(settings), [settings])
  // App menu → Keymap… opens Settings at the Keymap section.
  useEffect(() => {
    if (tab?.init?.settingsSection === 'keymap') document.getElementById('settings-keymap')?.scrollIntoView?.({ block: 'start' })
  }, [tab?.init, form === null])
  if (!form) return <Spinner />

  const set = <K extends keyof Settings>(k: K, v: Settings[K]) => setForm({ ...form, [k]: v })
  const dirty = JSON.stringify(form) !== JSON.stringify(settings)

  const save = async () => {
    setBusy(true)
    try {
      await saveSettings(form)
      toast({ kind: 'success', title: 'Settings saved' })
    } catch (err) {
      toastError(err, 'Could not save settings')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="view settings-view">
      <h1>Settings</h1>
      <section className="settings-group">
        <h2>Appearance</h2>
        <label className="field">
          <span>Theme</span>
          <select value={form.theme} onChange={(e) => set('theme', e.target.value as Settings['theme'])}>
            <option value="system">Follow system</option>
            <option value="light">Light</option>
            <option value="dark">Dark</option>
          </select>
        </label>
        <label className="field">
          <span>Editor font size</span>
          <input type="number" min={9} max={28} value={form.editorFontSize} onChange={(e) => set('editorFontSize', Number(e.target.value))} />
        </label>
        <label className="field">
          <span>Default result view</span>
          <select
            value={form.defaultView}
            onChange={(e) => setForm({ ...form, defaultView: e.target.value as Settings['defaultView'], defaultViewChosen: true })}
            aria-label="Default result view"
          >
            <option value="tree">Tree</option>
            <option value="table">Table</option>
            <option value="json">JSON</option>
          </select>
        </label>
      </section>
      <section className="settings-group">
        <h2>Queries</h2>
        <label className="field">
          <span>Documents per page (also caps shell output)</span>
          <input type="number" min={1} max={1000} value={form.pageSize} onChange={(e) => set('pageSize', Number(e.target.value))} />
        </label>
        <label className="field">
          <span>Operation timeout (seconds)</span>
          <input
            type="number"
            min={1}
            max={3600}
            value={form.queryTimeoutSeconds}
            onChange={(e) => set('queryTimeoutSeconds', Number(e.target.value))}
          />
        </label>
        <label className="check">
          <input type="checkbox" checked={form.confirmDeletes} onChange={(e) => set('confirmDeletes', e.target.checked)} />
          Ask before deleting documents
        </label>
      </section>
      <section className="settings-group">
        <h2>Diagnostics</h2>
        <label className="field">
          <span>Log level</span>
          <select value={form.logLevel} onChange={(e) => set('logLevel', e.target.value as Settings['logLevel'])}>
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warning</option>
            <option value="error">Error</option>
          </select>
        </label>
        {appInfo && (
          <dl className="stats">
            <dt>Version</dt>
            <dd>{appInfo.version}</dd>
            <dt>Config directory</dt>
            <dd className="mono">{appInfo.configDir}</dd>
            <dt>Log file</dt>
            <dd className="mono">{appInfo.logFile || '(stderr only)'}</dd>
            <dt>Password storage</dt>
            <dd>{appInfo.secretBackend === 'keyring' ? 'OS credential store' : appInfo.secretBackend === 'file' ? 'Encrypted file' : appInfo.secretBackend}</dd>
          </dl>
        )}
      </section>
      <AISettings form={form} set={set} />
      <div className="form-actions">
        <button onClick={() => setForm(settings)} disabled={!dirty || busy}>
          Revert
        </button>
        <button className="primary" onClick={save} disabled={!dirty || busy}>
          {busy && <Spinner small />} Save settings
        </button>
      </div>
      <MCPSettings />
      <div id="settings-keymap">
        <KeymapSettings />
      </div>
    </div>
  )
}
