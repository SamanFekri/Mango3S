import type { TabsAction } from './tabs'
import type { HistoryEntry, ItemKind, SavedItem, SessionInfo } from './types'

export const KIND_LABELS: Record<ItemKind, string> = {
  query: 'Query',
  aggregation: 'Pipeline',
  script: 'Shell script',
  sql: 'SQL',
}

/** Kinds that need a collection to open. */
export function needsCollection(kind: ItemKind): boolean {
  return kind === 'query' || kind === 'aggregation'
}

/**
 * Builds the tab-open action for a saved item or history entry on a connection.
 * Returns an error message instead when the item cannot be opened.
 */
export function openAction(item: SavedItem | HistoryEntry, session: SessionInfo, saved?: SavedItem): TabsAction | string {
  const base = { connId: session.id, connName: session.name, db: item.db || 'test' }
  const init = { savedId: saved?.id, savedName: saved?.name }
  switch (item.kind) {
    case 'query':
      if (!item.coll) return 'This query has no collection.'
      return { type: 'open', tab: { ...base, kind: 'collection', coll: item.coll, init: { ...init, query: item.query } } }
    case 'aggregation':
      if (!item.coll) return 'This pipeline has no collection.'
      return { type: 'open', tab: { ...base, kind: 'aggregation', coll: item.coll, init: { ...init, stages: item.stages } } }
    case 'script':
      return { type: 'open', tab: { ...base, kind: 'shell', init: { ...init, text: item.text } } }
    case 'sql':
      return { type: 'open', tab: { ...base, kind: 'sql', init: { ...init, text: item.text } } }
  }
}

/** Relative time like "3 min ago". */
export function timeAgo(iso: string, now = Date.now()): string {
  const t = new Date(iso).getTime()
  if (!Number.isFinite(t)) return ''
  const s = Math.max(0, Math.round((now - t) / 1000))
  if (s < 60) return 'just now'
  if (s < 3600) return `${Math.floor(s / 60)} min ago`
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`
  return new Date(iso).toLocaleDateString()
}
