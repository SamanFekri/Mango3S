// Central list of keyboard shortcuts. Everything that shows a shortcut (the
// reference dialog, tooltips) reads it from here, and the dispatcher in
// state/shortcuts.ts matches key events against it, so displayed and
// implemented bindings cannot drift apart.
//
// Key syntax: "Mod+Shift+N". Mod is ⌘ on macOS and Ctrl elsewhere; Ctrl is
// always the Control key. Keys are layout-independent for letters, digits
// and punctuation (KeyboardEvent.code), so Alt combinations work on macOS.

export type Platform = 'mac' | 'other'

export type ShortcutScope =
  /** Anywhere in the app. */
  | 'global'
  /** The active tab (registered by the tab's view while it is active). */
  | 'tab'
  /** Focus inside the explorer tree. */
  | 'explorer'
  /** Focus inside a results panel (not in a text field). */
  | 'results'
  /** Focus inside a query or script editor. */
  | 'editor'

/**
 * Who implements the shortcut:
 * - app: the dispatcher calls a handler registered with useShortcut.
 * - component: a component handles the key itself (the grid's cell editor, the tree).
 * - editor: the code editor (CodeMirror) handles it.
 */
export type Handler = 'app' | 'component' | 'editor'

/** studio3t: same as Studio 3T's documented default; default: chosen by Mango3S. */
export type Source = 'studio3t' | 'default'

export interface ShortcutDef {
  id: string
  label: string
  category: string
  mac: string[]
  other: string[]
  scope: ShortcutScope
  handler: Handler
  source: Source
  /** Also fires while typing in a text field or editor (only for combinations that don't type text). */
  allowInInputs?: boolean
  note?: string
}

const S = (
  id: string,
  label: string,
  category: string,
  keys: string[] | { mac: string[]; other: string[] },
  scope: ShortcutScope,
  source: Source,
  extra: Partial<ShortcutDef> = {},
): ShortcutDef => ({
  id,
  label,
  category,
  mac: Array.isArray(keys) ? keys : keys.mac,
  other: Array.isArray(keys) ? keys : keys.other,
  scope,
  handler: 'app',
  source,
  ...extra,
})

const digits = (prefix: string) => Array.from({ length: 9 }, (_, i) => `${prefix}${i + 1}`)

export const SHORTCUTS: ShortcutDef[] = [
  // Connections
  S('conn.manager', 'Open the connection manager', 'Connections', ['Mod+N'], 'global', 'studio3t', { allowInInputs: true }),
  S('conn.new', 'New connection', 'Connections', ['Mod+Alt+N'], 'global', 'default', { allowInInputs: true }),
  S('conn.quick', 'Connect to recent connection 1–9', 'Connections', digits('Alt+'), 'global', 'studio3t', {
    note: 'In the order shown under Recent connections on the Welcome tab.',
  }),
  S('conn.disconnect', 'Disconnect the current connection', 'Connections', { mac: ['Ctrl+Alt+D'], other: ['Ctrl+Alt+D'] }, 'global', 'studio3t', {
    allowInInputs: true,
  }),

  // Tabs
  S('tab.close', 'Close tab', 'Tabs', { mac: ['Mod+W'], other: ['Ctrl+W', 'Ctrl+F4'] }, 'global', 'studio3t', { allowInInputs: true }),
  S('tab.closeNoPrompt', 'Close tab without asking (discards unsaved changes)', 'Tabs', { mac: ['Mod+Shift+W'], other: ['Ctrl+Shift+F4'] }, 'global', 'studio3t', {
    allowInInputs: true,
  }),
  S('tab.next', 'Next tab', 'Tabs', ['Ctrl+PageDown', 'Ctrl+Tab'], 'global', 'studio3t', { allowInInputs: true }),
  S('tab.prev', 'Previous tab', 'Tabs', ['Ctrl+PageUp', 'Ctrl+Shift+Tab'], 'global', 'studio3t', { allowInInputs: true }),
  S('tab.goto', 'Go to tab 1–9', 'Tabs', digits('Mod+'), 'global', 'default', { allowInInputs: true }),
  S('tab.welcome', 'Open the Welcome tab', 'Tabs', ['Shift+F1'], 'global', 'default', { allowInInputs: true }),
  S('tab.shell', 'Open IntelliShell for this connection', 'Tabs', ['Mod+L'], 'global', 'studio3t', { allowInInputs: true }),
  S('tab.sql', 'Open SQL query', 'Tabs', ['Mod+Shift+L'], 'global', 'studio3t', { allowInInputs: true }),
  S('tab.aggregate', 'Open the aggregation editor for this collection', 'Tabs', ['F4'], 'tab', 'studio3t', { allowInInputs: true }),
  S('app.settings', 'Open settings', 'Tabs', { mac: ['Mod+P', 'Mod+Comma'], other: ['Ctrl+P'] }, 'global', 'studio3t', { allowInInputs: true }),
  S('app.library', 'Open query history', 'Tabs', ['Mod+Shift+H'], 'global', 'default', { allowInInputs: true }),

  // Help
  S('help.shortcuts', 'Show keyboard shortcuts', 'Help', ['F1', 'Mod+Shift+Slash'], 'global', 'default', { allowInInputs: true }),

  // Queries and scripts (active tab)
  S('query.run', 'Run the query, pipeline, script or SQL', 'Queries', ['Mod+Enter', 'F5'], 'tab', 'studio3t', {
    allowInInputs: true,
    note: 'F5 is Studio 3T’s “Execute”; Mod+Enter also works inside every editor.',
  }),
  S('query.runSelection', 'Run the selected text (shell)', 'Queries', ['F9', 'Mod+Shift+Enter'], 'tab', 'studio3t', { allowInInputs: true }),
  S('query.runStatement', 'Run the statement at the cursor (shell)', 'Queries', ['F6'], 'tab', 'studio3t', {
    allowInInputs: true,
    note: 'A statement is the block of lines around the cursor, up to blank lines.',
  }),
  S('query.format', 'Format the query or pipeline', 'Queries', { mac: ['Mod+Alt+L'], other: ['Ctrl+Alt+L', 'Alt+L'] }, 'tab', 'studio3t', {
    allowInInputs: true,
    note: 'Collection (filter, projection, sort) and aggregation tabs.',
  }),
  S('query.save', 'Save to the library', 'Queries', ['Mod+S'], 'tab', 'studio3t', { allowInInputs: true }),
  S('query.saveAs', 'Save as a new library item', 'Queries', ['Mod+Shift+S'], 'tab', 'studio3t', { allowInInputs: true }),
  S('op.cancel', 'Cancel the running operation', 'Queries', ['Mod+Period'], 'tab', 'default', { allowInInputs: true }),
  S('view.refresh', 'Refresh results (or the selected explorer item)', 'Queries', ['Mod+R'], 'tab', 'studio3t', { allowInInputs: true }),
  S('query.builder', 'Show or hide the visual query builder', 'Queries', ['Mod+B'], 'tab', 'studio3t', { allowInInputs: true }),
  S('query.code', 'Query code', 'Queries', ['Mod+G'], 'tab', 'studio3t', { allowInInputs: true }),
  S('query.explain', 'Explain', 'Queries', ['F8'], 'tab', 'studio3t', { allowInInputs: true }),
  S('agg.addStage', 'Add a pipeline stage', 'Queries', ['Mod+Shift+N'], 'tab', 'studio3t', { allowInInputs: true }),

  // Results
  S('results.tree', 'Tree view', 'Results', ['Mod+Alt+1'], 'tab', 'studio3t', { allowInInputs: true }),
  S('results.table', 'Table view', 'Results', ['Mod+Alt+2'], 'tab', 'studio3t', { allowInInputs: true }),
  S('results.json', 'JSON view', 'Results', ['Mod+Alt+3'], 'tab', 'studio3t', { allowInInputs: true }),
  S('cell.move', 'Select the next / previous cell', 'Results', ['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'], 'results', 'default', { handler: 'component' }),
  S('cell.edit', 'Edit the field with the mouse', 'Results', [], 'results', 'studio3t', {
    handler: 'component',
    note: 'Double-click — a single click only selects. In the table and tree views.',
  }),
  S('cell.save', 'Save the field being edited', 'Results', ['Enter'], 'results', 'default', { handler: 'component' }),
  S('cell.cancel', 'Cancel editing the field', 'Results', ['Escape'], 'results', 'default', { handler: 'component' }),
  S('cell.tab', 'Next / previous editable cell (saves an edit first)', 'Results', ['Tab', 'Shift+Tab'], 'results', 'default', { handler: 'component' }),
  S('docs.selectAll', 'Select all documents on the page', 'Results', ['Mod+A'], 'results', 'default'),

  S('edit.undo', 'Undo the last field edit', 'Results', ['Mod+Z'], 'results', 'default'),
  S('edit.redo', 'Redo the field edit', 'Results', { mac: ['Mod+Shift+Z'], other: ['Ctrl+Y', 'Ctrl+Shift+Z'] }, 'results', 'default'),

  // Documents (Studio 3T: Collection Tab › Documents). On macOS Studio 3T's
  // "Fn+Del" is the forward-delete key, which browsers report as Delete.
  S('doc.copy', 'Copy document', 'Documents', ['Mod+Shift+C'], 'results', 'studio3t'),
  S('doc.view', 'View document (JSON)', 'Documents', ['F3'], 'results', 'studio3t'),
  S('doc.insert', 'Insert document', 'Documents', ['Mod+D'], 'tab', 'studio3t', { note: 'Not while typing in an editor (Mod+D selects the next match there).' }),
  S('doc.editJson', 'Edit document (JSON)', 'Documents', ['Mod+J'], 'results', 'studio3t'),
  S('doc.delete', 'Remove document (asks first)', 'Documents', ['Shift+Delete'], 'results', 'studio3t', { note: 'macOS: Shift+Fn+Delete.' }),
  S('doc.refresh', 'Refresh document', 'Documents', ['Mod+Shift+R'], 'results', 'studio3t'),
  S('doc.followRef', 'Follow reference', 'Documents', ['Shift+F7'], 'results', 'studio3t', {
    note: 'Opens the document a DBRef points to; for other values, asks which collection to look in.',
  }),
  S('docs.paste', 'Paste documents', 'Documents', ['Mod+V'], 'results', 'default', {
    handler: 'component',
    note: 'Opens Insert document with the clipboard’s JSON.',
  }),

  // Fields (Studio 3T: Collection Tab › Fields)
  S('cell.copy', 'Copy value as text', 'Fields', ['Mod+C'], 'results', 'studio3t'),
  S('cell.copyValue', 'Copy value (exact shell syntax)', 'Fields', ['Alt+Shift+C'], 'results', 'studio3t'),
  S('field.add', 'Add field / value', 'Fields', ['Mod+K'], 'results', 'studio3t', { note: 'Adds a field to the selected document, or a value to the selected array.' }),
  S('field.edit', 'Edit field', 'Fields', ['Mod+E'], 'results', 'studio3t', { note: 'Keyboard alternative to double-clicking the value.' }),
  S('field.rename', 'Rename field', 'Fields', ['Shift+F2'], 'results', 'studio3t'),
  S('field.remove', 'Remove field (asks first)', 'Fields', ['Delete'], 'results', 'studio3t', { note: 'macOS: Fn+Delete.' }),
  S('dialog.update', 'Update dialog (updateMany)', 'Fields', ['Mod+U'], 'results', 'studio3t'),
  S('dialog.remove', 'Remove dialog (deleteMany)', 'Fields', ['F8'], 'results', 'studio3t'),

  // Explorer
  S('explorer.move', 'Move between items', 'Explorer', ['ArrowUp', 'ArrowDown', 'Home', 'End'], 'explorer', 'default', { handler: 'component' }),
  S('explorer.expand', 'Expand / collapse', 'Explorer', ['ArrowRight', 'ArrowLeft'], 'explorer', 'studio3t', { handler: 'component' }),
  S('explorer.open', 'Open the collection / connect', 'Explorer', ['Enter'], 'explorer', 'studio3t', { handler: 'component' }),
  S('explorer.drop', 'Drop the selected collection or database (asks first)', 'Explorer', ['Delete'], 'explorer', 'studio3t', {
    handler: 'component',
    note: 'macOS: Fn+Delete.',
  }),
  S('explorer.refreshAll', 'Refresh all connections', 'Explorer', ['Mod+Shift+R'], 'explorer', 'studio3t'),
  S('explorer.filter', 'Filter collections', 'Explorer', ['Mod+Shift+F'], 'global', 'default', { allowInInputs: true }),

  // Focus
  S('focus.explorer', 'Focus the explorer', 'Focus', ['Ctrl+Shift+1'], 'global', 'default', { allowInInputs: true }),
  S('focus.editor', 'Focus the query editor', 'Focus', ['Ctrl+Shift+2'], 'global', 'default', { allowInInputs: true }),
  S('focus.results', 'Focus the results', 'Focus', ['Ctrl+Shift+3'], 'global', 'default', { allowInInputs: true }),

  // Text editing (inside editors and text fields)
  S('text.undo', 'Undo / redo typing', 'Text editing', { mac: ['Mod+Z', 'Mod+Shift+Z'], other: ['Ctrl+Z', 'Ctrl+Y'] }, 'editor', 'studio3t', { handler: 'editor' }),
  S('text.clipboard', 'Cut / copy / paste', 'Text editing', ['Mod+X', 'Mod+C', 'Mod+V'], 'editor', 'studio3t', { handler: 'editor' }),
  S('text.selectAll', 'Select all', 'Text editing', ['Mod+A'], 'editor', 'default', { handler: 'editor' }),
  S('text.find', 'Find in the editor', 'Text editing', ['Mod+F'], 'editor', 'studio3t', { handler: 'editor', note: 'Script, pipeline and SQL editors.' }),
  S('text.comment', 'Toggle line comment', 'Text editing', ['Mod+Slash'], 'editor', 'studio3t', { handler: 'editor' }),
  S('text.complete', 'Autocomplete', 'Text editing', ['Ctrl+Space'], 'editor', 'studio3t', { handler: 'editor' }),
  S('shell.history', 'Previous / next script from history (shell)', 'Text editing', ['Alt+ArrowUp', 'Alt+ArrowDown'], 'editor', 'default', { handler: 'component' }),
]

export const SHORTCUT_BY_ID: Record<string, ShortcutDef> = Object.fromEntries(SHORTCUTS.map((s) => [s.id, s]))

export function detectPlatform(): Platform {
  const nav = typeof navigator !== 'undefined' ? navigator : undefined
  const p = (nav as { userAgentData?: { platform?: string } } | undefined)?.userAgentData?.platform ?? nav?.platform ?? nav?.userAgent ?? ''
  return /mac|iphone|ipad/i.test(p) ? 'mac' : 'other'
}

/** Customized bindings (Preferences → Keymap): platform → action id → bindings. */
export type KeymapOverrides = Partial<Record<Platform, Record<string, string[]>>>
let overrides: KeymapOverrides = {}

/** Applies the user's keymap; called when settings load or change. */
export function setKeymapOverrides(k: KeymapOverrides | undefined | null) {
  overrides = k ?? {}
}

export function getKeymapOverrides(): KeymapOverrides {
  return overrides
}

/** The documented default bindings of an action. */
export function defaultKeysFor(def: ShortcutDef, platform: Platform): string[] {
  return platform === 'mac' ? def.mac : def.other
}

/** The bindings in effect: customized ones if any, else the defaults. */
export function keysFor(def: ShortcutDef, platform: Platform): string[] {
  return overrides[platform]?.[def.id] ?? defaultKeysFor(def, platform)
}

export function isCustomized(def: ShortcutDef, platform: Platform): boolean {
  return overrides[platform]?.[def.id] !== undefined
}

/** Scopes whose shortcuts can fire for the same key press compete for a binding. */
export function scopeBucket(scope: ShortcutScope): string {
  return scope === 'global' || scope === 'tab' ? 'app' : scope
}

export interface Conflict {
  binding: string
  ids: string[]
}

/** Bindings used by more than one action that can fire in the same place. */
export function findConflicts(platform: Platform): Conflict[] {
  const by = new Map<string, string[]>()
  for (const s of SHORTCUTS) {
    for (const k of keysFor(s, platform)) {
      const key = `${scopeBucket(s.scope)}|${normalizeBinding(k, platform)}`
      by.set(key, [...(by.get(key) ?? []), s.id])
    }
  }
  return [...by.entries()].filter(([, ids]) => ids.length > 1).map(([k, ids]) => ({ binding: k.split('|')[1], ids }))
}

/** Actions other than id whose bindings would clash with binding in id's scope. */
export function conflictsWith(id: string, binding: string, platform: Platform): string[] {
  const def = SHORTCUT_BY_ID[id]
  const n = normalizeBinding(binding, platform)
  return SHORTCUTS.filter(
    (s) => s.id !== id && scopeBucket(s.scope) === scopeBucket(def.scope) && keysFor(s, platform).some((k) => normalizeBinding(k, platform) === n),
  ).map((s) => s.id)
}

/**
 * Turns a key press into a binding for the keymap editor, using Mod for the
 * platform's command key (⌘ on macOS, Ctrl elsewhere). Returns null for a
 * lone modifier.
 */
export function bindingFromEvent(e: Pick<KeyboardEvent, 'key' | 'code' | 'metaKey' | 'ctrlKey' | 'altKey' | 'shiftKey'>, platform: Platform): string | null {
  if (['Meta', 'Control', 'Alt', 'Shift', 'OS', 'AltGraph'].includes(e.key)) return null
  const parts: string[] = []
  const mod = platform === 'mac' ? e.metaKey : e.ctrlKey
  if (mod) parts.push('Mod')
  if (platform === 'mac' && e.ctrlKey) parts.push('Ctrl')
  if (platform !== 'mac' && e.metaKey) parts.push('Meta')
  if (e.altKey) parts.push('Alt')
  if (e.shiftKey) parts.push('Shift')
  parts.push(eventKey(e))
  return parts.join('+')
}

/** Plain typing keys can't be shortcuts that fire while editing text. */
export function isTypingKey(binding: string): boolean {
  const parts = binding.split('+')
  const key = parts[parts.length - 1]
  const mods = parts.slice(0, -1).filter((m) => m !== 'Shift')
  return mods.length === 0 && (key.length === 1 || ['Space', 'Enter', 'Tab', 'Backspace', 'Delete', 'Escape'].includes(key))
}

const CODE_KEYS: Record<string, string> = {
  Slash: 'Slash',
  Period: 'Period',
  Comma: 'Comma',
  Space: 'Space',
  Minus: 'Minus',
  Equal: 'Equal',
  BracketLeft: 'BracketLeft',
  BracketRight: 'BracketRight',
}

/** The canonical key name of an event: letters/digits/punctuation from e.code, others from e.key. */
export function eventKey(e: Pick<KeyboardEvent, 'key' | 'code'>): string {
  const code = e.code ?? ''
  if (/^Key[A-Z]$/.test(code)) return code.slice(3)
  if (/^Digit[0-9]$/.test(code)) return code.slice(5)
  if (/^Numpad[0-9]$/.test(code)) return code.slice(6)
  if (CODE_KEYS[code]) return CODE_KEYS[code]
  if (code === 'NumpadEnter') return 'Enter'
  const k = e.key
  if (k === ' ') return 'Space'
  if (k === 'Esc') return 'Escape'
  if (k.length === 1) return k.toUpperCase()
  return k
}

type Mods = Pick<KeyboardEvent, 'metaKey' | 'ctrlKey' | 'altKey' | 'shiftKey'>

/** Normalizes a binding like "Mod+Shift+n" for the platform into "Ctrl+Shift+N" or "Meta+Shift+N". */
export function normalizeBinding(binding: string, platform: Platform): string {
  const parts = binding.split('+')
  const key = parts.pop()!
  const mods = new Set(parts.map((p) => (p === 'Mod' ? (platform === 'mac' ? 'Meta' : 'Ctrl') : p)))
  const order = ['Ctrl', 'Alt', 'Shift', 'Meta'].filter((m) => mods.has(m))
  return [...order, key.length === 1 ? key.toUpperCase() : key].join('+')
}

export function eventCombo(e: Mods & Pick<KeyboardEvent, 'key' | 'code'>): string {
  const mods = [e.ctrlKey && 'Ctrl', e.altKey && 'Alt', e.shiftKey && 'Shift', e.metaKey && 'Meta'].filter(Boolean)
  return [...mods, eventKey(e)].join('+')
}

/** Returns the index of the binding of def that the event matches, or -1. */
export function matchIndex(def: ShortcutDef, e: Mods & Pick<KeyboardEvent, 'key' | 'code'>, platform: Platform): number {
  const combo = eventCombo(e)
  return keysFor(def, platform).findIndex((b) => normalizeBinding(b, platform) === combo)
}

const MAC_SYMBOLS: Record<string, string> = { Meta: '⌘', Ctrl: '⌃', Alt: '⌥', Shift: '⇧' }
const KEY_LABELS: Record<string, string> = {
  Slash: '/',
  Period: '.',
  Comma: ',',
  Space: 'Space',
  ArrowUp: '↑',
  ArrowDown: '↓',
  ArrowLeft: '←',
  ArrowRight: '→',
  Escape: 'Esc',
  Delete: 'Del',
  PageUp: 'PgUp',
  PageDown: 'PgDn',
  Backspace: '⌫',
  Enter: 'Enter',
}

/** Human-readable binding, e.g. "⌘⇧N" on macOS or "Ctrl+Shift+N" elsewhere. */
export function formatBinding(binding: string, platform: Platform): string {
  const parts = normalizeBinding(binding, platform).split('+')
  const key = parts.pop()!
  const label = KEY_LABELS[key] ?? key
  if (platform === 'mac') return parts.map((m) => MAC_SYMBOLS[m] ?? m).join('') + (label === 'Enter' ? '↩' : label)
  return [...parts, label].join('+')
}

/** Compact label for a definition's bindings, collapsing 1–9 ranges. */
export function formatShortcut(def: ShortcutDef, platform: Platform): string {
  const keys = keysFor(def, platform)
  if (keys.length === 9 && keys.every((k, i) => k.endsWith(`+${i + 1}`))) {
    return `${formatBinding(keys[0], platform)}…9`
  }
  return keys.map((k) => formatBinding(k, platform)).join('  /  ')
}

export function shortcutHint(id: string, platform: Platform = detectPlatform()): string {
  const def = SHORTCUT_BY_ID[id]
  if (!def) return ''
  const k = keysFor(def, platform)[0]
  return k ? formatBinding(k, platform) : ''
}

/** True for elements where typing happens: inputs, textareas, selects, contenteditable (CodeMirror). */
export function isTextTarget(el: Element | null): boolean {
  if (!el) return false
  if (el instanceof HTMLInputElement) {
    return !['checkbox', 'radio', 'button', 'submit', 'reset', 'range', 'color', 'file'].includes(el.type)
  }
  if (el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement) return true
  return (el as HTMLElement).isContentEditable || !!el.closest?.('[contenteditable="true"], .cm-content')
}
