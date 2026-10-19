import { useMemo, useState } from 'react'
import { backend } from '../lib/backend'
import {
  SHORTCUTS,
  SHORTCUT_BY_ID,
  bindingFromEvent,
  conflictsWith,
  defaultKeysFor,
  findConflicts,
  formatBinding,
  isCustomized,
  isTypingKey,
  keysFor,
  normalizeBinding,
  type ShortcutDef,
} from '../lib/shortcuts'
import type { Settings } from '../lib/types'
import { useApp } from '../state/AppContext'
import { getShortcutPlatform } from '../state/shortcuts'

const MAX = 3 // shortcuts per action
const TEXT_EDITING = ['Mod+C', 'Mod+V', 'Mod+X', 'Mod+Z', 'Mod+A', 'Mod+Y', 'Mod+Shift+Z']

type Keymap = NonNullable<Settings['keymap']>

/** Preferences → Keymap: view, search, customize, reset, import and export shortcuts. */
export function KeymapSettings() {
  const { settings, saveSettings, toast, toastError } = useApp()
  const platform = getShortcutPlatform()
  const [filter, setFilter] = useState('')
  const [recording, setRecording] = useState<string | null>(null)
  const [pending, setPending] = useState<{ id: string; binding: string; conflicts: string[] } | null>(null)
  const [problem, setProblem] = useState<{ id: string; text: string } | null>(null)

  const keymap: Keymap = settings?.keymap ?? {}
  const save = async (next: Keymap) => {
    if (!settings) return
    try {
      await saveSettings({ ...settings, keymap: next })
    } catch (err) {
      toastError(err, 'Could not save the keymap')
    }
  }
  const setBindings = (k: Keymap, id: string, bindings: string[]): Keymap => ({ ...k, [platform]: { ...(k[platform] ?? {}), [id]: bindings } })
  const reset = (id: string) => {
    const mine = { ...(keymap[platform] ?? {}) }
    delete mine[id]
    void save({ ...keymap, [platform]: mine })
  }

  const assign = (id: string, binding: string, steal: string[]) => {
    let next = setBindings(keymap, id, [...keysFor(SHORTCUT_BY_ID[id], platform), binding])
    const n = normalizeBinding(binding, platform)
    for (const other of steal) {
      next = setBindings(next, other, keysFor(SHORTCUT_BY_ID[other], platform).filter((k) => normalizeBinding(k, platform) !== n))
    }
    setPending(null)
    void save(next)
  }

  const onRecordKey = (def: ShortcutDef, e: React.KeyboardEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (e.key === 'Escape') return setRecording(null)
    const b = bindingFromEvent(e, platform)
    if (!b) return // a lone modifier: keep listening
    setRecording(null)
    if (keysFor(def, platform).some((k) => normalizeBinding(k, platform) === normalizeBinding(b, platform))) {
      return setProblem({ id: def.id, text: `${formatBinding(b, platform)} is already assigned to this action.` })
    }
    if (def.allowInInputs && isTypingKey(b)) {
      return setProblem({ id: def.id, text: `${formatBinding(b, platform)} would type text in editors; add a modifier such as ${platform === 'mac' ? '⌘' : 'Ctrl'}.` })
    }
    if (def.scope !== 'results' && def.scope !== 'editor' && def.scope !== 'explorer' && TEXT_EDITING.includes(b)) {
      return setProblem({ id: def.id, text: `${formatBinding(b, platform)} is a standard text-editing shortcut and can't be reassigned here.` })
    }
    setProblem(null)
    const conflicts = conflictsWith(def.id, b, platform)
    if (conflicts.length) return setPending({ id: def.id, binding: b, conflicts })
    assign(def.id, b, [])
  }

  const exportKeymap = async () => {
    const body = JSON.stringify({ format: 'mango3s-keymap', version: 1, keymap }, null, 2)
    try {
      const path = await backend.exportTextFile('Export keymap', 'mango3s-keymap.json', body)
      if (path) toast({ kind: 'success', title: 'Keymap exported', message: path })
    } catch (err) {
      toastError(err, 'Export failed')
    }
  }
  const importKeymap = async () => {
    try {
      const text = await backend.importTextFile('Import keymap')
      if (!text) return
      const parsed = parseKeymapFile(text)
      await save(parsed.keymap)
      toast({ kind: 'success', title: 'Keymap imported', message: parsed.skipped ? `${parsed.skipped} unknown action(s) were ignored.` : undefined })
    } catch (err) {
      toastError(err, 'Import failed')
    }
  }

  const conflicts = findConflicts(platform)
  const groups = useMemo(() => {
    const f = filter.trim().toLowerCase()
    const out = new Map<string, ShortcutDef[]>()
    for (const s of SHORTCUTS) {
      const keys = keysFor(s, platform).map((k) => formatBinding(k, platform)).join(' ')
      if (f && !`${s.label} ${s.category} ${keys} ${s.id}`.toLowerCase().includes(f)) continue
      out.set(s.category, [...(out.get(s.category) ?? []), s])
    }
    return [...out.entries()]
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filter, platform, settings?.keymap])

  return (
    <section className="settings-group keymap">
      <h2>Keymap</h2>
      <p className="muted small">
        {platform === 'mac' ? 'macOS' : 'Windows / Linux'} shortcuts. Each action can have up to {MAX}{' '}
        shortcuts. Changes apply immediately.
      </p>
      <div className="keymap-toolbar">
        <input placeholder="Search actions or keys" value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Search keymap" />
        <span className="spacer" />
        <button className="small" onClick={() => void importKeymap()}>
          Import…
        </button>
        <button className="small" onClick={() => void exportKeymap()}>
          Export…
        </button>
        <button className="small ghost danger-text" disabled={!keymap[platform] || Object.keys(keymap[platform] ?? {}).length === 0} onClick={() => void save({ ...keymap, [platform]: {} })}>
          Reset all to defaults
        </button>
      </div>
      {conflicts.length > 0 && (
        <div className="info-banner small" role="alert">
          Conflicting shortcuts:{' '}
          {conflicts.map((c) => `${formatBinding(c.binding, platform)} (${c.ids.map((i) => SHORTCUT_BY_ID[i].label).join(' / ')})`).join('; ')}
        </div>
      )}
      {groups.map(([cat, list]) => (
        <div key={cat} className="keymap-group">
          <h3>{cat}</h3>
          <table className="shortcut-table">
            <tbody>
              {list.map((s) => {
                const keys = keysFor(s, platform)
                const fixed = s.handler !== 'app'
                return (
                  <tr key={s.id} data-action={s.id}>
                    <td>
                      {s.label}
                      {isCustomized(s, platform) && <span className="badge">custom</span>}
                      {fixed && <div className="muted small">{s.handler === 'editor' ? 'Handled by the text editor' : 'Built into the view'}; not customizable.</div>}
                      {problem?.id === s.id && <div className="error-text small">{problem.text}</div>}
                      {pending?.id === s.id && (
                        <div className="small keymap-conflict" role="alert">
                          {formatBinding(pending.binding, platform)} is used by {pending.conflicts.map((c) => SHORTCUT_BY_ID[c].label).join(', ')}.{' '}
                          <button className="small" onClick={() => assign(s.id, pending.binding, pending.conflicts)}>
                            Reassign
                          </button>{' '}
                          <button className="small ghost" onClick={() => setPending(null)}>
                            Cancel
                          </button>
                        </div>
                      )}
                    </td>
                    <td className="keys">
                      {s.id === 'cell.edit' && <kbd>Double-click</kbd>}
                      {keys.map((k) => (
                        <span key={k} className="key-chip">
                          <kbd>{formatBinding(k, platform)}</kbd>
                          {!fixed && (
                            <button
                              className="icon-btn"
                              aria-label={`Remove ${formatBinding(k, platform)} from ${s.label}`}
                              onClick={() => void save(setBindings(keymap, s.id, keys.filter((x) => x !== k)))}
                            >
                              ×
                            </button>
                          )}
                        </span>
                      ))}
                      {!fixed && keys.length === 0 && <span className="muted small">none</span>}
                    </td>
                    <td className="keymap-actions">
                      {!fixed &&
                        (recording === s.id ? (
                          <input
                            className="recorder"
                            autoFocus
                            readOnly
                            value="Press keys…"
                            aria-label={`Record a shortcut for ${s.label}`}
                            onKeyDown={(e) => onRecordKey(s, e)}
                            onBlur={() => setRecording(null)}
                          />
                        ) : (
                          <button className="small ghost" disabled={keys.length >= MAX} onClick={() => setRecording(s.id)} title={keys.length >= MAX ? `At most ${MAX} shortcuts` : undefined}>
                            + Add
                          </button>
                        ))}
                      {isCustomized(s, platform) && (
                        <button className="small ghost" onClick={() => reset(s.id)} title={`Default: ${defaultKeysFor(s, platform).map((k) => formatBinding(k, platform)).join(', ') || 'none'}`}>
                          Reset
                        </button>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      ))}
    </section>
  )
}

/** Reads an exported keymap file, dropping unknown actions and invalid entries. */
export function parseKeymapFile(text: string): { keymap: Keymap; skipped: number } {
  let raw: unknown
  try {
    raw = JSON.parse(text)
  } catch {
    throw new Error(JSON.stringify({ code: 'INVALID_INPUT', message: 'This is not a Mango3S keymap file (not JSON).' }))
  }
  const obj = raw as { format?: string; keymap?: Record<string, Record<string, unknown>> }
  if (!obj || obj.format !== 'mango3s-keymap' || typeof obj.keymap !== 'object') {
    throw new Error(JSON.stringify({ code: 'INVALID_INPUT', message: 'This is not a Mango3S keymap file.' }))
  }
  const out: Keymap = {}
  let skipped = 0
  for (const plat of ['mac', 'other'] as const) {
    const m = obj.keymap?.[plat]
    if (!m || typeof m !== 'object') continue
    const clean: Record<string, string[]> = {}
    for (const [id, v] of Object.entries(m)) {
      const def = SHORTCUT_BY_ID[id]
      if (!def || def.handler !== 'app' || !Array.isArray(v)) {
        skipped++
        continue
      }
      clean[id] = v.filter((b): b is string => typeof b === 'string' && b.length > 0 && b.length <= 40).slice(0, MAX)
    }
    out[plat] = clean
  }
  return { keymap: out, skipped }
}
