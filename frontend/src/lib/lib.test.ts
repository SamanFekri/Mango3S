import { describe, expect, it } from 'vitest'
import { columnsOf, parseDoc, renderCell } from './ejson'
import { toAppError } from './errors'
import { formatBytes, pageRange } from './format'
import { pipelineReducer, newStage, templateFor, toStages, type EditorStage } from './pipeline'
import {
  buildFilter,
  buildProjection,
  buildSort,
  literal,
  type BuilderState,
  type Condition,
} from './queryBuilder'
import { tabsReducer, type TabsState } from './tabs'

describe('errors', () => {
  it('parses JSON-encoded backend errors', () => {
    const e = toAppError(new Error('{"code":"AUTH_FAILED","message":"bad creds","detail":"x"}'))
    expect(e).toEqual({ code: 'AUTH_FAILED', message: 'bad creds', detail: 'x' })
  })
  it('wraps plain strings', () => {
    expect(toAppError('boom')).toEqual({ code: 'UNKNOWN', message: 'boom' })
  })
  it('passes typed errors through', () => {
    const e = { code: 'TIMEOUT', message: 't' }
    expect(toAppError(e)).toBe(e)
  })
})

describe('ejson rendering', () => {
  it('renders wrappers and containers', () => {
    expect(renderCell({ $oid: 'abc' })).toEqual({ text: 'ObjectId("abc")', kind: 'objectId' })
    expect(renderCell({ $date: '2024-01-01T00:00:00Z' }).kind).toBe('date')
    expect(renderCell({ $date: { $numberLong: '0' } }).text).toBe('1970-01-01T00:00:00.000Z')
    expect(renderCell({ $numberLong: '9007199254740993' }).text).toBe('9007199254740993')
    expect(renderCell({ $numberDecimal: '1.10' }).text).toBe('1.10')
    expect(renderCell({ $binary: { base64: 'ASNFZ4mrze8BI0VniavN7w==', subType: '04' } }).text).toBe(
      'UUID(01234567-89ab-cdef-0123-456789abcdef)',
    )
    expect(renderCell({ $regularExpression: { pattern: 'a', options: 'i' } }).text).toBe('/a/i')
    expect(renderCell([1, 2]).text).toBe('[ 2 items ]')
    expect(renderCell({ a: 1 }).text).toBe('{ 1 field }')
    expect(renderCell(undefined).kind).toBe('missing')
    expect(renderCell('x'.repeat(500), 10).text).toHaveLength(10)
  })
  it('collects columns with _id first', () => {
    const docs = [parseDoc('{"b":1,"_id":2}'), parseDoc('{"c":3,"b":4}')]
    expect(columnsOf(docs)).toEqual(['_id', 'b', 'c'])
    expect(parseDoc('not json')).toEqual({})
  })
})

describe('query builder', () => {
  const cond = (p: Partial<Condition>): Condition => ({ id: 'x', field: 'f', op: 'eq', value: '', type: 'auto', ...p })
  const state = (conditions: Condition[], combinator: 'and' | 'or' = 'and'): BuilderState => ({
    combinator,
    conditions,
    sort: [],
    projection: [],
    projectionMode: 'include',
  })

  it('infers literal types in auto mode', () => {
    expect(literal('42', 'auto')).toBe('42')
    expect(literal('true', 'auto')).toBe('true')
    expect(literal('5f1d7f0b2c3a4b5c6d7e8f90', 'auto')).toBe('ObjectId("5f1d7f0b2c3a4b5c6d7e8f90")')
    expect(literal('2024-01-31', 'auto')).toBe('ISODate("2024-01-31")')
    expect(literal('he said "hi"', 'auto')).toBe('"he said \\"hi\\""')
    expect(literal('42', 'string')).toBe('"42"')
    expect(() => literal('abc', 'number')).toThrow()
    expect(() => literal('zz', 'objectId')).toThrow()
  })

  it('builds filters', () => {
    expect(buildFilter(state([cond({ field: '' })]))).toBe('{}')
    expect(buildFilter(state([cond({ field: 'age', op: 'gte', value: '21' }), cond({ field: 'name', value: 'Ada' })]))).toBe(
      '{ age: { $gte: 21 }, name: "Ada" }',
    )
    expect(buildFilter(state([cond({ field: 'a', value: '1' }), cond({ field: 'a', op: 'ne', value: '2' })]))).toBe(
      '{ $and: [{ a: 1 }, { a: { $ne: 2 } }] }',
    )
    expect(buildFilter(state([cond({ field: 'a', value: '1' }), cond({ field: 'b', value: '2' })], 'or'))).toBe(
      '{ $or: [{ a: 1 }, { b: 2 }] }',
    )
    expect(buildFilter(state([cond({ field: 'tags', op: 'in', value: 'x, y' })]))).toBe('{ tags: { $in: ["x", "y"] } }')
    expect(buildFilter(state([cond({ field: 'name', op: 'regex', value: '^A' })]))).toBe(
      '{ name: { $regex: "^A", $options: "i" } }',
    )
    expect(buildFilter(state([cond({ field: 'x', op: 'exists', value: '' })]))).toBe('{ x: { $exists: true } }')
    expect(buildFilter(state([cond({ field: 'weird key', value: '1' })]))).toBe('{ "weird key": 1 }')
    expect(buildFilter(state([cond({ field: 'a.b', value: '1' })]))).toBe('{ a.b: 1 }')
    expect(() => buildFilter(state([cond({ field: 'n', op: 'regex', value: '(' })]))).toThrow(/n:/)
  })

  it('builds sort and projection', () => {
    expect(buildSort([{ id: '1', field: 'a', dir: -1 }, { id: '2', field: '', dir: 1 }])).toBe('{ a: -1 }')
    expect(buildSort([])).toBe('')
    expect(buildProjection(['a', ' b '], 'include')).toBe('{ a: 1, b: 1 }')
    expect(buildProjection(['a'], 'exclude')).toBe('{ a: 0 }')
  })
})

describe('pipeline reducer', () => {
  const three = (): EditorStage[] => [newStage('$match'), newStage('$sort'), newStage('$limit')]

  it('adds, moves, toggles and removes stages', () => {
    let s = three()
    s = pipelineReducer(s, { type: 'move', from: 2, to: 0 })
    expect(s.map((x) => x.operator)).toEqual(['$limit', '$match', '$sort'])
    s = pipelineReducer(s, { type: 'move', from: 0, to: 9 })
    expect(s[0].operator).toBe('$limit')
    s = pipelineReducer(s, { type: 'toggle', index: 1 })
    expect(s[1].enabled).toBe(false)
    s = pipelineReducer(s, { type: 'add', operator: '$group', after: 0 })
    expect(s[1].operator).toBe('$group')
    s = pipelineReducer(s, { type: 'remove', index: 1 })
    expect(s).toHaveLength(3)
    s = pipelineReducer(s, { type: 'duplicate', index: 0 })
    expect(s[1].operator).toBe('$limit')
    expect(s[1].key).not.toBe(s[0].key)
  })

  it('swaps an untouched template when the operator changes', () => {
    let s = [newStage('$match')]
    s = pipelineReducer(s, { type: 'update', index: 0, patch: { operator: '$limit' } })
    expect(s[0].body).toBe(templateFor('$limit'))
    s = pipelineReducer(s, { type: 'update', index: 0, patch: { body: '5' } })
    s = pipelineReducer(s, { type: 'update', index: 0, patch: { operator: '$skip' } })
    expect(s[0].body).toBe('5')
    expect(toStages(s)[0]).toEqual({ operator: '$skip', body: '5', enabled: true })
  })
})

describe('tabs reducer', () => {
  const empty: TabsState = { tabs: [], activeId: null }

  it('opens, reuses singletons and closes', () => {
    let s = tabsReducer(empty, { type: 'open', tab: { kind: 'settings' } })
    const settingsId = s.activeId
    s = tabsReducer(s, { type: 'open', tab: { kind: 'collection', connId: 'c', db: 'd', coll: 'x' } })
    expect(s.tabs).toHaveLength(2)
    expect(s.tabs[1].title).toBe('x')
    s = tabsReducer(s, { type: 'open', tab: { kind: 'settings' } })
    expect(s.tabs).toHaveLength(2)
    expect(s.activeId).toBe(settingsId)
    s = tabsReducer(s, { type: 'open', tab: { kind: 'collection', connId: 'c', db: 'd', coll: 'x' }, reuse: true })
    expect(s.tabs).toHaveLength(2)
    s = tabsReducer(s, { type: 'close', id: s.activeId! })
    expect(s.tabs).toHaveLength(1)
    expect(s.activeId).toBe(settingsId)
  })

  it('closes all tabs of a connection', () => {
    let s = tabsReducer(empty, { type: 'open', tab: { kind: 'shell', connId: 'a' } })
    s = tabsReducer(s, { type: 'open', tab: { kind: 'shell', connId: 'b' } })
    s = tabsReducer(s, { type: 'closeConnection', connId: 'b' })
    expect(s.tabs.map((t) => t.connId)).toEqual(['a'])
    expect(s.activeId).toBe(s.tabs[0].id)
  })
})

describe('format', () => {
  it('formats bytes and ranges', () => {
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1536)).toBe('1.50 KB')
    expect(pageRange(1, 50, 50)).toBe('51–100')
    expect(pageRange(0, 50, 0)).toBe('No documents')
  })
})

import { buildQueryText } from './queryText'

describe('buildQueryText', () => {
  it('renders the query as one statement', () => {
    expect(buildQueryText('agent_definitions', { filter: '{}', projection: '', sort: '', skip: 0, limit: 0 })).toBe('db.getCollection("agent_definitions").find({})')
    expect(buildQueryText('o', { filter: '{ a: 1 }', projection: '{ b: 1 }', sort: '{ c: -1 }', skip: 5, limit: 10 })).toBe(
      'db.getCollection("o").find({ a: 1 }, { b: 1 })\n  .sort({ c: -1 })\n  .skip(5)\n  .limit(10)',
    )
    expect(buildQueryText('we"ird', { filter: '', projection: '{}', sort: '{ }', skip: 0, limit: 0 })).toBe('db.getCollection("we\\"ird").find({})')
  })
})

import { moveId } from './tabs'

describe('moveId', () => {
  it('moves an id before or after another', () => {
    expect(moveId(['a', 'b', 'c', 'd'], 'd', 'b', true)).toEqual(['a', 'd', 'b', 'c'])
    expect(moveId(['a', 'b', 'c', 'd'], 'a', 'c', false)).toEqual(['b', 'c', 'a', 'd'])
    expect(moveId(['a', 'b'], 'a', 'a', true)).toEqual(['a', 'b'])
    expect(moveId(['a', 'b'], 'x', 'a', true)).toEqual(['a', 'b'])
  })
})
