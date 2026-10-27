import type { DbRole, Privilege, PrivilegeResource } from '../../lib/types'

/** Built-in roles that apply to one database. */
export const DB_ROLES = ['read', 'readWrite', 'dbAdmin', 'dbOwner', 'userAdmin']

/** Built-in roles that only exist in admin and apply to every database or the
 * whole deployment. */
export const ADMIN_ROLES = [
  'readAnyDatabase',
  'readWriteAnyDatabase',
  'dbAdminAnyDatabase',
  'userAdminAnyDatabase',
  'clusterMonitor',
  'clusterManager',
  'hostManager',
  'clusterAdmin',
  'backup',
  'restore',
  'root',
]

/** Actions offered as checkboxes in the privilege matrix; others can be typed. */
export const COLLECTION_ACTIONS = [
  'find',
  'insert',
  'update',
  'remove',
  'createIndex',
  'dropIndex',
  'listIndexes',
  'createCollection',
  'dropCollection',
  'listCollections',
  'collStats',
  'dbStats',
  'changeStream',
]
export const CLUSTER_ACTIONS = ['serverStatus', 'listDatabases', 'replSetGetStatus', 'inprog', 'killop', 'top', 'listShards']

export const roleKey = (r: DbRole) => `${r.role}@${r.db}`
export const sameRole = (a: DbRole, b: DbRole) => a.role === b.role && a.db === b.db
export const hasRole = (roles: DbRole[], r: DbRole) => roles.some((x) => sameRole(x, r))

/** "role@db" → a role; a bare name belongs to defaultDb. */
export function parseRole(text: string, defaultDb: string): DbRole | null {
  const t = text.trim()
  if (!t) return null
  const at = t.lastIndexOf('@')
  if (at <= 0) return { role: t, db: defaultDb }
  return { role: t.slice(0, at), db: t.slice(at + 1) || defaultDb }
}

/** A short label for what a privilege applies to. */
export function resourceLabel(r: PrivilegeResource): string {
  if (r.anyResource) return 'any resource'
  if (r.cluster) return 'cluster'
  const db = r.db || 'any database'
  if (!r.collection) return `${db} (every collection)`
  return r.db ? `${r.db}.${r.collection}` : `${r.collection} (in any database)`
}

/** Privileges merged by resource, actions sorted, for display. */
export function groupPrivileges(privs: Privilege[]): { label: string; actions: string[] }[] {
  const by = new Map<string, Set<string>>()
  for (const p of privs) {
    const l = resourceLabel(p.resource)
    const s = by.get(l) ?? new Set<string>()
    p.actions.forEach((a) => s.add(a))
    by.set(l, s)
  }
  return [...by.entries()].map(([label, s]) => ({ label, actions: [...s].sort() })).sort((a, b) => a.label.localeCompare(b.label))
}

/** A random password: 20 characters from letters, digits and a few symbols
 * that are safe in connection strings and shells. */
export function generatePassword(length = 20): string {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789-_.~'
  const bytes = new Uint8Array(length)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => chars[b % chars.length]).join('')
}
