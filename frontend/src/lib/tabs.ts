import type { QueryPayload, Stage } from './types'

export type TabKind =
  | 'welcome' | 'collection' | 'aggregation' | 'shell' | 'settings' | 'schema' | 'sql' | 'library'
  | 'compare' | 'reschema' | 'mask' | 'migrate' | 'tasks' | 'connection'

/** Content a tab opens with, e.g. from a saved item or the history. */
export interface TabInit {
  query?: QueryPayload
  stages?: Stage[]
  text?: string
  savedId?: string
  savedName?: string
  /** Library tab: which section to show. */
  section?: 'saved' | 'history' | 'backups'
  /** Settings tab: section to scroll to. */
  settingsSection?: 'keymap' | 'platform'
}

export interface Tab {
  id: string
  kind: TabKind
  title: string
  connId?: string
  connName?: string
  db?: string
  coll?: string
  init?: TabInit
}

export interface TabsState {
  tabs: Tab[]
  activeId: string | null
}

export type TabsAction =
  | { type: 'open'; tab: Omit<Tab, 'id' | 'title'> & { title?: string }; reuse?: boolean }
  | { type: 'close'; id: string }
  | { type: 'closeOthers'; id: string }
  | { type: 'closeConnection'; connId: string }
  | { type: 'activate'; id: string }
  | { type: 'move'; from: number; to: number }
  | { type: 'step'; delta: number }

let seq = 0
function tabId(): string {
  seq += 1
  return `t${seq}`
}

export function tabTitle(t: Omit<Tab, 'id' | 'title'>): string {
  switch (t.kind) {
    case 'welcome':
      return 'Welcome'
    case 'settings':
      return 'Settings'
    case 'collection':
      return t.coll ?? 'Collection'
    case 'aggregation':
      return `${t.coll ?? 'Aggregate'} · pipeline`
    case 'shell':
      return `Shell · ${t.connName ?? t.connId ?? ''}`
    case 'schema':
      return `${t.coll ?? ''} · schema`
    case 'sql':
      return `SQL · ${t.db ?? ''}`
    case 'library':
      return 'Library'
    case 'compare':
      return `Compare · ${t.coll ?? t.db ?? ''}`
    case 'reschema':
      return `${t.coll ?? ''} · reschema`
    case 'mask':
      return `${t.coll ?? ''} · mask`
    case 'migrate':
      return `Migrate → ${t.db ?? ''}`
    case 'tasks':
      return 'Tasks'
    case 'connection':
      return t.connName ?? 'Connection'
  }
}

function sameTarget(a: Omit<Tab, 'id' | 'title'>, b: Tab): boolean {
  if (a.kind === 'connection') return b.kind === 'connection' && a.connId === b.connId
  return a.kind === b.kind && a.connId === b.connId && a.db === b.db && a.coll === b.coll
}

/** Kinds that exist at most once (per connection for workspaces). */
const SINGLETONS = new Set<TabKind>(['settings', 'welcome', 'library', 'tasks', 'connection'])

export function tabsReducer(state: TabsState, action: TabsAction): TabsState {
  switch (action.type) {
    case 'open': {
      if (SINGLETONS.has(action.tab.kind) || action.reuse) {
        const existing = state.tabs.find((t) => sameTarget(action.tab, t))
        if (existing) {
          // Re-opening may carry new content (e.g. the library section).
          const tabs = action.tab.init ? state.tabs.map((t) => (t.id === existing.id ? { ...t, init: action.tab.init } : t)) : state.tabs
          return { tabs, activeId: existing.id }
        }
      }
      const tab: Tab = { ...action.tab, id: tabId(), title: action.tab.title ?? tabTitle(action.tab) }
      const tabs = [...state.tabs]
      if (tab.kind === 'welcome') {
        tabs.unshift(tab) // Welcome is always the first tab
      } else {
        const idx = state.tabs.findIndex((t) => t.id === state.activeId)
        tabs.splice(idx < 0 ? tabs.length : idx + 1, 0, tab)
      }
      return { tabs, activeId: tab.id }
    }
    case 'close': {
      const idx = state.tabs.findIndex((t) => t.id === action.id)
      if (idx < 0) return state
      const tabs = state.tabs.filter((t) => t.id !== action.id)
      let activeId = state.activeId
      if (activeId === action.id) {
        activeId = tabs.length ? tabs[Math.min(idx, tabs.length - 1)].id : null
      }
      return { tabs, activeId }
    }
    case 'closeOthers':
      return { tabs: state.tabs.filter((t) => t.id === action.id), activeId: action.id }
    case 'closeConnection': {
      const tabs = state.tabs.filter((t) => t.connId !== action.connId)
      const activeStill = tabs.some((t) => t.id === state.activeId)
      return { tabs, activeId: activeStill ? state.activeId : (tabs[tabs.length - 1]?.id ?? null) }
    }
    case 'step': {
      if (state.tabs.length === 0) return state
      const idx = Math.max(0, state.tabs.findIndex((t) => t.id === state.activeId))
      const n = state.tabs.length
      return { ...state, activeId: state.tabs[(((idx + action.delta) % n) + n) % n].id }
    }
    case 'activate':
      return state.tabs.some((t) => t.id === action.id) ? { ...state, activeId: action.id } : state
    case 'move': {
      const { from, to } = action
      // Welcome stays first.
      if (state.tabs[from]?.kind === 'welcome' || (to === 0 && state.tabs[0]?.kind === 'welcome')) return state
      if (from === to || from < 0 || to < 0 || from >= state.tabs.length || to >= state.tabs.length) return state
      const tabs = [...state.tabs]
      const [t] = tabs.splice(from, 1)
      tabs.splice(to, 0, t)
      return { ...state, tabs }
    }
  }
}

/**
 * Moves id next to target (before or after it) in an ordered list of ids.
 * Returns the list unchanged when id is target or either is missing.
 */
export function moveId(ids: string[], id: string, target: string, before: boolean): string[] {
  if (id === target || !ids.includes(id) || !ids.includes(target)) return ids
  const rest = ids.filter((x) => x !== id)
  const at = rest.indexOf(target) + (before ? 0 : 1)
  return [...rest.slice(0, at), id, ...rest.slice(at)]
}
