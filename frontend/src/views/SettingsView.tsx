import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Spinner } from '../components/common'
import { AccountSettings } from '../components/AccountSettings'
import { KeymapSettings } from '../components/KeymapSettings'
import { AISettings, MCPSettings } from '../components/P2Settings'
import { PlatformSettings } from '../components/PlatformSettings'
import { PlatformConnection } from '../components/PlatformConnection'
import { StorageSettings } from '../components/StorageSettings'
import { APIConsole } from '../components/APIConsole'
import { backend } from '../lib/backend'
import type { SettingsSection, Tab } from '../lib/tabs'
import type { Settings } from '../lib/types'
import { useApp } from '../state/AppContext'

interface SectionDef {
  id: SettingsSection
  label: string
  /** The section edits the shared settings form, so it gets the Save / Revert bar. */
  form: boolean
  /** Shown only in test mode (`make dev`). */
  testOnly?: boolean
}

const ALL_SECTIONS: SectionDef[] = [
  { id: 'general', label: 'General', form: true },
  { id: 'ai', label: 'AI assistant', form: true },
  { id: 'mcp', label: 'MCP server', form: false },
  { id: 'keymap', label: 'Keymap', form: false },
  { id: 'account', label: 'Account & sync', form: false },
  { id: 'platform', label: 'Updates', form: true },
  { id: 'storage', label: 'Storage', form: false },
  { id: 'about', label: 'Diagnostics', form: true },
  { id: 'api', label: 'API console', form: false, testOnly: true },
]

/** Settings, split into tabs. Unsaved edits are kept while switching tabs and
 * saved together; MCP, Keymap and Account save on their own. */
export function SettingsView({ tab }: { tab?: Tab }) {
  const { settings, saveSettings, appInfo, toast, toastError } = useApp()
  const [form, setForm] = useState<Settings | null>(settings)
  const [busy, setBusy] = useState(false)
  const [section, setSection] = useState<SettingsSection>(tab?.init?.settingsSection ?? 'general')
  const tabRefs = useRef<Record<string, HTMLButtonElement | null>>({})
  const SECTIONS = ALL_SECTIONS.filter((s) => !s.testOnly || appInfo?.testMode)

  useEffect(() => setForm(settings), [settings])
  // App menu → Keymap… (and other deep links) open Settings at that tab.
  useEffect(() => {
    const want = tab?.init?.settingsSection
    if (!want) return
    setSection(want)
    document.getElementById(`settings-tab-${want}`)?.scrollIntoView?.({ block: 'nearest' })
  }, [tab?.init])
  if (!form) return <Spinner />

  const set = <K extends keyof Settings>(k: K, v: Settings[K]) => setForm({ ...form, [k]: v })
  const dirty = JSON.stringify(form) !== JSON.stringify(settings)
  const current = SECTIONS.find((s) => s.id === section) ?? SECTIONS[0]!

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

  const onTabKey = (e: KeyboardEvent) => {
    const i = SECTIONS.findIndex((s) => s.id === section)
    const next = e.key === 'ArrowDown' ? i + 1 : e.key === 'ArrowUp' ? i - 1 : e.key === 'Home' ? 0 : e.key === 'End' ? SECTIONS.length - 1 : null
    if (next === null) return
    e.preventDefault()
    const target = SECTIONS[(next + SECTIONS.length) % SECTIONS.length]!
    setSection(target.id)
    tabRefs.current[target.id]?.focus()
  }

  return (
    <div className="view settings-view">
      <h1>Settings</h1>
      <div className="settings-layout">
        <div className="settings-tabs" role="tablist" aria-label="Settings sections" aria-orientation="vertical" onKeyDown={onTabKey}>
          {SECTIONS.map((s) => (
            <button
              key={s.id}
              ref={(el) => {
                tabRefs.current[s.id] = el
              }}
              id={`settings-tab-${s.id}`}
              role="tab"
              aria-selected={section === s.id}
              aria-controls="settings-panel"
              tabIndex={section === s.id ? 0 : -1}
              className={section === s.id ? 'active' : ''}
              onClick={() => setSection(s.id)}
            >
              {s.label}
            </button>
          ))}
        </div>

        <div className="settings-panel" id="settings-panel" role="tabpanel" aria-labelledby={`settings-tab-${section}`}>
          {section === 'general' && (
            <>
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
                <label className="check">
                  <input type="checkbox" checked={form.showMascot !== false} onChange={(e) => set('showMascot', e.target.checked)} />
                  Show Mango, the mascot, in the explorer
                </label>
                <label className="check">
                  <input type="checkbox" checked={!!form.schemaValidation} onChange={(e) => set('schemaValidation', e.target.checked)} />
                  Suggest validation rules from the schema (adds "Create validation rules…" to the Schema tab)
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
            </>
          )}

          {section === 'ai' && <AISettings form={form} set={set} />}
          {section === 'mcp' && <MCPSettings />}
          {section === 'keymap' && (
            <div id="settings-keymap">
              <KeymapSettings />
            </div>
          )}
          {section === 'account' && <AccountSettings />}
          {section === 'platform' && <PlatformSettings form={form} set={set} />}
          {section === 'storage' && <StorageSettings />}
          {section === 'api' && appInfo?.testMode && <APIConsole />}

          {section === 'about' && (
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
              <PrivacyStatus />
              <PlatformConnection />
            </section>
          )}

          {current.form && (
            <div className="form-actions settings-save">
              {dirty && <span className="muted small">Unsaved changes</span>}
              <span className="spacer" />
              <button onClick={() => setForm(settings)} disabled={!dirty || busy}>
                Revert
              </button>
              <button className="primary" onClick={save} disabled={!dirty || busy}>
                {busy && <Spinner small />} Save settings
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

/** The privacy policy this installation accepted, from the Mango3S server.
 * Shown only once the server has answered. */
function PrivacyStatus() {
  const { privacyPolicy, openPrivacy, toastError } = useApp()
  const [policy, setPolicy] = useState(privacyPolicy)
  useEffect(() => {
    let live = true
    backend
      .getPrivacyPolicy()
      .then((p) => live && p && setPolicy(p))
      .catch(() => undefined)
    return () => {
      live = false
    }
  }, [privacyPolicy])
  if (!policy) return null
  return (
    <p className="small">
      Privacy policy{policy.version ? ` version ${policy.version}` : ''}
      {policy.acceptanceRequired ? (
        <>
          {' '}
          — waiting for you to accept it.{' '}
          <button className="link" onClick={() => openPrivacy(policy)}>
            Review
          </button>
        </>
      ) : policy.acceptedAt ? (
        ` — accepted ${new Date(policy.acceptedAt).toLocaleDateString()}.`
      ) : (
        '.'
      )}{' '}
      {policy.url && (
        <button className="link" onClick={() => backend.openPlatformLink(policy.url).catch((err) => toastError(err, 'Could not open the link'))}>
          Read it
        </button>
      )}
    </p>
  )
}
