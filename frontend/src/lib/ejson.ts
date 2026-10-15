// Helpers for rendering relaxed Extended JSON (as produced by the Go driver)
// in the grid view. Exact values are always available in Doc.text; these
// helpers only produce display strings.

export type JsonValue = null | boolean | number | string | JsonValue[] | { [k: string]: JsonValue }
export type JsonObject = { [k: string]: JsonValue }

export type CellKind =
  | 'null'
  | 'bool'
  | 'number'
  | 'string'
  | 'objectId'
  | 'date'
  | 'binary'
  | 'regex'
  | 'special'
  | 'array'
  | 'object'
  | 'missing'

export interface Cell {
  text: string
  kind: CellKind
}

export function parseDoc(json: string): JsonObject {
  try {
    const v = JSON.parse(json)
    return v && typeof v === 'object' && !Array.isArray(v) ? v : {}
  } catch {
    return {}
  }
}

function isObject(v: JsonValue | undefined): v is JsonObject {
  return v !== null && typeof v === 'object' && !Array.isArray(v)
}

function keys(o: JsonObject): string[] {
  return Object.keys(o)
}

/** Renders a wrapper like {"$oid": "..."} or returns null for plain objects. */
export function renderWrapper(o: JsonObject): Cell | null {
  const k = keys(o)
  if (k.length === 0 || !k[0].startsWith('$')) return null
  const v = o[k[0]]
  switch (k[0]) {
    case '$oid':
      return { text: `ObjectId("${v}")`, kind: 'objectId' }
    case '$date': {
      if (typeof v === 'string') return { text: v, kind: 'date' }
      if (isObject(v) && typeof v.$numberLong === 'string') {
        const ms = Number(v.$numberLong)
        const d = new Date(ms)
        return { text: isNaN(d.getTime()) ? `Date(${v.$numberLong})` : d.toISOString(), kind: 'date' }
      }
      return { text: String(v), kind: 'date' }
    }
    case '$numberLong':
    case '$numberInt':
      return { text: String(v), kind: 'number' }
    case '$numberDouble':
      return { text: String(v), kind: 'number' }
    case '$numberDecimal':
      return { text: `${v}`, kind: 'number' }
    case '$binary': {
      if (isObject(v)) {
        const sub = String(v.subType ?? '00')
        if (sub === '04' || sub === '4') return { text: `UUID(${uuidFromBase64(String(v.base64))})`, kind: 'binary' }
        return { text: `BinData(${parseInt(sub, 16)}, …${String(v.base64).length} chars)`, kind: 'binary' }
      }
      return { text: 'BinData', kind: 'binary' }
    }
    case '$uuid':
      return { text: `UUID(${v})`, kind: 'binary' }
    case '$regularExpression':
      if (isObject(v)) return { text: `/${v.pattern}/${v.options ?? ''}`, kind: 'regex' }
      return { text: 'RegExp', kind: 'regex' }
    case '$timestamp':
      if (isObject(v)) return { text: `Timestamp(${v.t}, ${v.i})`, kind: 'special' }
      return { text: 'Timestamp', kind: 'special' }
    case '$minKey':
      return { text: 'MinKey', kind: 'special' }
    case '$maxKey':
      return { text: 'MaxKey', kind: 'special' }
    case '$undefined':
      return { text: 'undefined', kind: 'null' }
    case '$code':
      return { text: `Code(${truncate(String(v), 60)})`, kind: 'special' }
    case '$symbol':
      return { text: String(v), kind: 'string' }
  }
  return null
}

function uuidFromBase64(b64: string): string {
  try {
    const bin = atob(b64)
    let hex = ''
    for (let i = 0; i < bin.length; i++) hex += bin.charCodeAt(i).toString(16).padStart(2, '0')
    if (hex.length !== 32) return hex
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
  } catch {
    return b64
  }
}

export function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max - 1) + '…' : s
}

export function renderCell(v: JsonValue | undefined, maxLen = 120): Cell {
  if (v === undefined) return { text: '', kind: 'missing' }
  if (v === null) return { text: 'null', kind: 'null' }
  if (typeof v === 'boolean') return { text: String(v), kind: 'bool' }
  if (typeof v === 'number') return { text: String(v), kind: 'number' }
  if (typeof v === 'string') return { text: truncate(v, maxLen), kind: 'string' }
  if (Array.isArray(v)) return { text: `[ ${v.length} ${v.length === 1 ? 'item' : 'items'} ]`, kind: 'array' }
  const w = renderWrapper(v)
  if (w) return { ...w, text: truncate(w.text, maxLen) }
  const n = keys(v).length
  return { text: `{ ${n} ${n === 1 ? 'field' : 'fields'} }`, kind: 'object' }
}

/** Union of top-level field names in first-seen order, with _id first. */
export function columnsOf(docs: JsonObject[], max = 60): string[] {
  const seen = new Set<string>()
  const cols: string[] = []
  for (const d of docs) {
    for (const k of keys(d)) {
      if (!seen.has(k)) {
        seen.add(k)
        cols.push(k)
      }
    }
  }
  cols.sort((a, b) => (a === '_id' ? -1 : b === '_id' ? 1 : 0))
  return cols.slice(0, max)
}
