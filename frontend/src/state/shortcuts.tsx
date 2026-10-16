// The shortcut dispatcher: one window-level keydown listener that matches
// events against lib/shortcuts.ts and calls the handlers components register
// with useShortcut. Native menu items run the same handlers via runShortcut.
import { createContext, useContext, useEffect, useRef } from 'react'
import type { RefObject } from 'react'
import { SHORTCUT_BY_ID, SHORTCUTS, detectPlatform, isTextTarget, matchIndex, type Platform, type ShortcutDef } from '../lib/shortcuts'

export type ShortcutHandler = (ctx: { index: number; event?: KeyboardEvent }) => void | boolean

interface Registration {
  id: string
  handler: { current: ShortcutHandler }
  enabled: { current: boolean }
  /** For explorer/results/editor scopes: focus must be inside this element. */
  within?: RefObject<HTMLElement | null>
  order: number
}

const registry = new Map<string, Registration[]>()
let seq = 0
let platform: Platform = detectPlatform()

/** Overrides platform detection (tests). */
export function setShortcutPlatform(p: Platform) {
  platform = p
}

export function getShortcutPlatform(): Platform {
  return platform
}

function register(r: Registration) {
  const list = registry.get(r.id) ?? []
  list.push(r)
  registry.set(r.id, list)
  return () => {
    const l = registry.get(r.id) ?? []
    registry.set(
      r.id,
      l.filter((x) => x !== r),
    )
  }
}

/** Picks the most specific enabled handler for a shortcut (focused element scope > tab > global). */
function resolve(def: ShortcutDef, active: Element | null): Registration | undefined {
  const regs = (registry.get(def.id) ?? []).filter((r) => r.enabled.current)
  const candidates = regs.filter((r) => !r.within || (r.within.current && active && r.within.current.contains(active)))
  if (def.scope === 'results' || def.scope === 'explorer') {
    // Element-scoped shortcuts need focus inside their element.
    return candidates.filter((r) => r.within).sort((a, b) => b.order - a.order)[0]
  }
  return candidates.sort((a, b) => (b.within ? 1 : 0) - (a.within ? 1 : 0) || b.order - a.order)[0]
}

/** True while a modal dialog other than the shortcut reference is open. */
function dialogOpen(target: Element | null): boolean {
  const dialogs = Array.from(document.querySelectorAll('[role="dialog"]'))
  return dialogs.length > 0 && !(target && dialogs.some((d) => d.contains(target) && d.hasAttribute('data-shortcuts-pass')))
}

export function handleKeyDown(e: KeyboardEvent): boolean {
  if (e.defaultPrevented || e.repeat && !['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(e.key)) return false
  // AltGr (Ctrl+Alt on Windows layouts) types characters such as @ and €.
  if (e.ctrlKey && e.altKey && e.getModifierState?.('AltGraph')) return false
  const target = (e.target as Element | null) ?? document.activeElement
  const typing = isTextTarget(target)
  const inDialog = dialogOpen(target)
  for (const def of SHORTCUTS) {
    if (def.handler !== 'app') continue
    const index = matchIndex(def, e, platform)
    if (index < 0) continue
    if (typing && !def.allowInInputs) continue
    if (inDialog && def.id !== 'help.shortcuts') continue
    const reg = resolve(def, target)
    if (!reg) continue
    const res = reg.handler.current({ index, event: e })
    if (res === false) continue // handler declined (e.g. nothing selected)
    e.preventDefault()
    e.stopPropagation()
    return true
  }
  return false
}

/** Runs a shortcut's handler without a key event (native menu, buttons). */
export function runShortcut(id: string): boolean {
  const def = SHORTCUT_BY_ID[id]
  if (!def) return false
  const reg = resolve(def, document.activeElement) ?? (registry.get(id) ?? []).find((r) => r.enabled.current && !r.within)
  if (!reg) return false
  return reg.handler.current({ index: 0 }) !== false
}

/** Installs the window listener (once, in the app shell). */
export function useShortcutDispatcher() {
  useEffect(() => {
    // Capture phase so app shortcuts win over focused widgets' defaults,
    // except where a component already handled the key (defaultPrevented).
    const onKey = (e: KeyboardEvent) => handleKeyDown(e)
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
}

/** Whether the tab a component lives in is the active one. */
export const TabActiveContext = createContext(true)
export const useTabActive = () => useContext(TabActiveContext)

/**
 * Registers a handler for a shortcut id. For tab-scoped shortcuts it is
 * enabled only while the surrounding tab is active; `within` restricts it to
 * focus inside an element (explorer, results).
 */
export function useShortcut(id: string, handler: ShortcutHandler, opts: { enabled?: boolean; within?: RefObject<HTMLElement | null> } = {}) {
  if (!SHORTCUT_BY_ID[id]) throw new Error(`Unknown shortcut ${id}`)
  const tabActive = useTabActive()
  const h = useRef(handler)
  h.current = handler
  const enabled = useRef(true)
  const def = SHORTCUT_BY_ID[id]
  enabled.current = (opts.enabled ?? true) && (def?.scope === 'global' || tabActive)
  const within = opts.within
  useEffect(() => {
    seq += 1
    return register({ id, handler: h, enabled, within, order: seq })
  }, [id, within])
}

/** For tests: drops every registration. */
export function resetShortcuts() {
  registry.clear()
}

/** Ids that currently have a handler (tests, diagnostics). */
export function registeredShortcutIds(): string[] {
  return [...registry.entries()].filter(([, l]) => l.length > 0).map(([id]) => id)
}
