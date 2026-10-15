import { describe, expect, it } from 'vitest'
import { needsCollection, openAction, timeAgo } from './library'
import { currentCollection, historyStep, shellCompletions } from './shellComplete'
import { tabsReducer } from './tabs'
import type { SavedItem, SessionInfo } from './types'

const ctx = { collections: ['users', 'orders', 'orders.archive', 'my-coll'], fields: ['name', 'age'] }
const labels = (before: string) => shellCompletions(before, ctx)?.options.map((o) => o.label) ?? []

describe('shell completion', () => {
  it('offers collections and db methods after db.', () => {
    const r = shellCompletions('db.us', ctx)!
    expect(r.from).toBe(3)
    expect(labels('db.')).toEqual(expect.arrayContaining(['users', 'orders', 'getCollection("my-coll")', 'runCommand']))
  })

  it('offers collection methods after db.coll.', () => {
    const r = shellCompletions('db.users.fi', ctx)!
    expect(r.from).toBe('db.users.'.length)
    expect(r.options.find((o) => o.label === 'find')?.apply).toBe('find(')
    expect(labels('db.getCollection("my-coll").')).toContain('insertOne')
    // dotted collection names are offered too
    expect(labels('db.orders.')).toEqual(expect.arrayContaining(['find', 'archive']))
  })

  it('offers cursor methods after a call', () => {
    expect(labels('db.users.find({ a: 1 }).')).toEqual(expect.arrayContaining(['sort', 'limit', 'explain']))
    expect(labels('db.users.find().sort({a: 1}).li')).toContain('limit')
  })

  it('offers show targets, operators, fields and statements', () => {
    expect(labels('show ')).toEqual(['dbs', 'collections'])
    expect(labels('db.users.find({ age: { $g')).toEqual(expect.arrayContaining(['$gt', '$gte']))
    expect(labels('db.users.find({ a')).toEqual(expect.arrayContaining(['name', 'age', 'ObjectId']))
    expect(labels('')).toEqual(expect.arrayContaining(['db', 'show dbs', 'use']))
    expect(shellCompletions('db.users.find({ a: 1, ', ctx)).toBeNull()
  })

  it('finds the collection referenced by the current statement', () => {
    expect(currentCollection('db.users.find({ ')).toBe('users')
    expect(currentCollection('db.getCollection("my-coll").find({ ')).toBe('my-coll')
    expect(currentCollection('db.orders.archive.find({')).toBe('orders.archive')
    expect(currentCollection('db.users.find();\ndb.orders.count(')).toBe('orders')
    expect(currentCollection('show dbs')).toBeNull()
  })

  it('walks history like a terminal', () => {
    const h = ['newest', 'middle', 'oldest']
    expect(historyStep(h, -1, true)).toBe(0)
    expect(historyStep(h, 2, true)).toBe(2)
    expect(historyStep(h, 0, false)).toBe(-1)
    expect(historyStep([], -1, true)).toBe(-1)
  })
})

describe('library helpers', () => {
  const session: SessionInfo = { id: 'c1', name: 'Local', readOnly: false, serverVersion: '8', topology: 'standalone', connectedAt: '' }
  const base: SavedItem = { id: 'i1', kind: 'query', name: 'Q', description: '', db: 'shop', coll: 'orders', createdAt: '', updatedAt: '' }

  it('opens the right tab kind with its payload', () => {
    const q = openAction({ ...base, query: { filter: '{a: 1}', projection: '', sort: '', skip: 0, limit: 0 } }, session, base)
    expect(q).toMatchObject({ type: 'open', tab: { kind: 'collection', coll: 'orders', connId: 'c1', init: { savedId: 'i1', query: { filter: '{a: 1}' } } } })
    expect(openAction({ ...base, kind: 'sql', coll: '', text: 'SELECT 1' }, session)).toMatchObject({ tab: { kind: 'sql', db: 'shop', init: { text: 'SELECT 1' } } })
    expect(openAction({ ...base, kind: 'script', db: '', text: 'show dbs' }, session)).toMatchObject({ tab: { kind: 'shell', db: 'test' } })
    expect(typeof openAction({ ...base, coll: '' }, session)).toBe('string')
    expect(needsCollection('aggregation')).toBe(true)
    expect(needsCollection('sql')).toBe(false)
  })

  it('formats relative times', () => {
    const now = Date.parse('2026-01-01T12:00:00Z')
    expect(timeAgo('2026-01-01T11:59:30Z', now)).toBe('just now')
    expect(timeAgo('2026-01-01T11:30:00Z', now)).toBe('30 min ago')
    expect(timeAgo('2026-01-01T09:00:00Z', now)).toBe('3 h ago')
    expect(timeAgo('garbage', now)).toBe('')
  })

  it('library and new tab kinds have titles; library is a singleton', () => {
    let s = tabsReducer({ tabs: [], activeId: null }, { type: 'open', tab: { kind: 'library' } })
    s = tabsReducer(s, { type: 'open', tab: { kind: 'library' } })
    expect(s.tabs).toHaveLength(1)
    s = tabsReducer(s, { type: 'open', tab: { kind: 'schema', coll: 'orders' } })
    s = tabsReducer(s, { type: 'open', tab: { kind: 'sql', db: 'shop' } })
    expect(s.tabs.map((t) => t.title)).toEqual(['Library', 'orders · schema', 'SQL · shop'])
  })
})
