// Navigation helpers for documents in canonical Extended JSON (exact types),
// shared by the tree and table views and their cell editing.
import { renderCell, renderWrapper, type JsonObject, type JsonValue } from './ejson'

/** A field location: field names, with array indexes as decimal strings. */
export type FieldPath = string[]

const WRAPPER_TYPES: Record<string, string> = {
  $oid: 'objectId',
  $date: 'date',
  $numberInt: 'int32',
  $numberLong: 'int64',
  $numberDouble: 'double',
  $numberDecimal: 'decimal',
  $binary: 'binary',
  $uuid: 'binary',
  $regularExpression: 'regex',
  $timestamp: 'timestamp',
  $minKey: 'minKey',
  $maxKey: 'maxKey',
  $undefined: 'undefined',
  $code: 'javascript',
  $symbol: 'string',
}

export function isPlainObject(v: JsonValue | undefined): v is JsonObject {
  return v !== null && typeof v === 'object' && !Array.isArray(v)
}

/** BSON type name of a canonical Extended JSON value (matches the backend's mql.TypeName). */
export function bsonType(v: JsonValue | undefined): string {
  if (v === undefined) return 'missing'
  if (v === null) return 'null'
  if (typeof v === 'boolean') return 'boolean'
  if (typeof v === 'string') return 'string'
  if (typeof v === 'number') return Number.isInteger(v) ? 'int32' : 'double' // relaxed input
  if (Array.isArray(v)) return 'array'
  const ks = Object.keys(v)
  if (ks.length > 0 && WRAPPER_TYPES[ks[0]] && renderWrapper(v)) return WRAPPER_TYPES[ks[0]]
  return 'document'
}

/** True for embedded documents and arrays (expandable in the tree). */
export function isContainer(v: JsonValue | undefined): boolean {
  return Array.isArray(v) || (isPlainObject(v) && bsonType(v) === 'document')
}

/** Child entries of a container: [key, value] with array indexes as keys. */
export function childrenOf(v: JsonValue | undefined): [string, JsonValue][] {
  if (Array.isArray(v)) return v.map((x, i) => [String(i), x])
  if (isPlainObject(v) && bsonType(v) === 'document') return Object.entries(v)
  return []
}

export function getAt(doc: JsonValue, path: FieldPath): JsonValue | undefined {
  let cur: JsonValue | undefined = doc
  for (const seg of path) {
    if (Array.isArray(cur)) {
      const i = Number(seg)
      if (!/^\d+$/.test(seg) || i >= cur.length) return undefined
      cur = cur[i]
    } else if (isPlainObject(cur) && bsonType(cur) === 'document') {
      if (!Object.prototype.hasOwnProperty.call(cur, seg)) return undefined
      cur = cur[seg]
    } else {
      return undefined
    }
  }
  return cur
}

/** One-line display text for a value. */
export function displayValue(v: JsonValue | undefined, maxLen = 120): string {
  return renderCell(v, maxLen).text
}

export const pathKey = (path: FieldPath) => JSON.stringify(path)

/**
 * Table columns: top-level fields, or, with expand, the leaf paths of
 * embedded documents ("address.city"). Arrays stay single columns.
 */
export function tableColumns(docs: JsonObject[], expand: boolean, max = 80): FieldPath[] {
  const seen = new Set<string>()
  const cols: FieldPath[] = []
  const add = (p: FieldPath) => {
    const k = pathKey(p)
    if (!seen.has(k)) {
      seen.add(k)
      cols.push(p)
    }
  }
  const walk = (v: JsonValue, prefix: FieldPath, depth: number) => {
    for (const [k, child] of Object.entries(v as JsonObject)) {
      const p = [...prefix, k]
      if (expand && depth < 4 && isPlainObject(child) && bsonType(child) === 'document' && Object.keys(child).length > 0) walk(child, p, depth + 1)
      else add(p)
    }
  }
  for (const d of docs) walk(d, [], 0)
  cols.sort((a, b) => (a[0] === '_id' ? -1 : b[0] === '_id' ? 1 : 0))
  return cols.slice(0, max)
}

/** Text to copy for a value: strings raw, everything else as displayed. */
export function copyText(v: JsonValue | undefined): string {
  if (typeof v === 'string') return v
  if (v === undefined) return ''
  return renderCell(v, 100000).kind === 'object' || renderCell(v).kind === 'array' ? JSON.stringify(v, null, 2) : renderCell(v, 100000).text
}
