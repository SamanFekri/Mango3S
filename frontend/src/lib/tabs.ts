export type TabKind = 'welcome' | 'collection' | 'aggregation' | 'shell' | 'settings'

export interface Tab {
  id: string
  kind: TabKind
  title: string
  connId?: string
  connName?: string
  db?: string
  coll?: string
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
  }
}

function sameTarget(a: Omit<Tab, 'id' | 'title'>, b: Tab): boolean {
  return a.kind === b.kind && a.connId === b.connId && a.db === b.db && a.coll === b.coll
}

export function tabsReducer(state: TabsState, action: TabsAction): TabsState {
  switch (action.type) {
    case 'open': {
      const singleton = action.tab.kind === 'settings' || action.tab.kind === 'welcome'
      if (singleton || action.reuse) {
        const existing = state.tabs.find((t) => sameTarget(action.tab, t))
        if (existing) return { ...state, activeId: existing.id }
      }
      const tab: Tab = { ...action.tab, id: tabId(), title: action.tab.title ?? tabTitle(action.tab) }
      const idx = state.tabs.findIndex((t) => t.id === state.activeId)
      const tabs = [...state.tabs]
      tabs.splice(idx < 0 ? tabs.length : idx + 1, 0, tab)
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
    case 'activate':
      return state.tabs.some((t) => t.id === action.id) ? { ...state, activeId: action.id } : state
    case 'move': {
      const { from, to } = action
      if (from === to || from < 0 || to < 0 || from >= state.tabs.length || to >= state.tabs.length) return state
      const tabs = [...state.tabs]
      const [t] = tabs.splice(from, 1)
      tabs.splice(to, 0, t)
      return { ...state, tabs }
    }
  }
}
