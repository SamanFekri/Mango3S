// The visual query builder produces shell-syntax text that is placed in the
// regular Filter / Sort / Projection editors, so both paths share one parser.

export type Operator = 'eq' | 'ne' | 'gt' | 'gte' | 'lt' | 'lte' | 'in' | 'nin' | 'regex' | 'exists' | 'type'
export type ValueType = 'auto' | 'string' | 'number' | 'boolean' | 'null' | 'objectId' | 'date'

export interface Condition {
  id: string
  field: string
  op: Operator
  value: string
  type: ValueType
}

export interface SortRule {
  id: string
  field: string
  dir: 1 | -1
}

export interface BuilderState {
  combinator: 'and' | 'or'
  conditions: Condition[]
  sort: SortRule[]
  projection: string[]
  projectionMode: 'include' | 'exclude'
}

export const OPERATORS: { value: Operator; label: string; needsValue: boolean }[] = [
  { value: 'eq', label: 'equals', needsValue: true },
  { value: 'ne', label: 'not equals', needsValue: true },
  { value: 'gt', label: '>', needsValue: true },
  { value: 'gte', label: '≥', needsValue: true },
  { value: 'lt', label: '<', needsValue: true },
  { value: 'lte', label: '≤', needsValue: true },
  { value: 'in', label: 'in list', needsValue: true },
  { value: 'nin', label: 'not in list', needsValue: true },
  { value: 'regex', label: 'matches regex', needsValue: true },
  { value: 'exists', label: 'exists', needsValue: false },
  { value: 'type', label: 'has type', needsValue: true },
]

export const VALUE_TYPES: { value: ValueType; label: string }[] = [
  { value: 'auto', label: 'Auto' },
  { value: 'string', label: 'String' },
  { value: 'number', label: 'Number' },
  { value: 'boolean', label: 'Boolean' },
  { value: 'null', label: 'Null' },
  { value: 'objectId', label: 'ObjectId' },
  { value: 'date', label: 'Date' },
]

export const BSON_TYPES = [
  'double', 'string', 'object', 'array', 'binData', 'objectId', 'bool', 'date',
  'null', 'regex', 'int', 'timestamp', 'long', 'decimal', 'number',
]

let nextId = 0
export function uid(): string {
  nextId += 1
  return `c${nextId}`
}

export function emptyBuilder(): BuilderState {
  return {
    combinator: 'and',
    conditions: [{ id: uid(), field: '', op: 'eq', value: '', type: 'auto' }],
    sort: [],
    projection: [],
    projectionMode: 'include',
  }
}

export class BuilderError extends Error {}

const BARE_KEY = /^[A-Za-z_$][\w$]*(\.[\w$]+)*$/

export function key(field: string): string {
  return BARE_KEY.test(field) ? field : JSON.stringify(field)
}

const OBJECT_ID = /^[0-9a-fA-F]{24}$/
const ISO_DATE = /^\d{4}-\d{2}-\d{2}([T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?)?$/
const NUMBER = /^-?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/

/** Converts a raw input string into a shell literal of the requested type. */
export function literal(raw: string, type: ValueType): string {
  const v = raw.trim()
  switch (type) {
    case 'string':
      return JSON.stringify(raw)
    case 'number':
      if (!NUMBER.test(v)) throw new BuilderError(`"${raw}" is not a number`)
      return v
    case 'boolean':
      if (v !== 'true' && v !== 'false') throw new BuilderError(`"${raw}" is not true or false`)
      return v
    case 'null':
      return 'null'
    case 'objectId':
      if (!OBJECT_ID.test(v)) throw new BuilderError(`"${raw}" is not a 24-character hex ObjectId`)
      return `ObjectId("${v.toLowerCase()}")`
    case 'date':
      if (!ISO_DATE.test(v)) throw new BuilderError(`"${raw}" is not an ISO date (e.g. 2024-01-31)`)
      return `ISODate("${v}")`
    case 'auto':
      if (v === 'true' || v === 'false' || v === 'null') return v
      if (NUMBER.test(v)) return v
      if (OBJECT_ID.test(v)) return `ObjectId("${v.toLowerCase()}")`
      if (ISO_DATE.test(v) && v.length >= 10) return `ISODate("${v}")`
      return JSON.stringify(raw)
  }
}

function listLiteral(raw: string, type: ValueType): string {
  const items = raw
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s !== '')
  if (items.length === 0) throw new BuilderError('Enter one or more comma-separated values')
  return `[${items.map((i) => literal(i, type)).join(', ')}]`
}

/** Returns the expression for one condition (the part after "field: "). */
export function conditionExpr(c: Condition): string {
  switch (c.op) {
    case 'eq':
      return literal(c.value, c.type)
    case 'ne':
    case 'gt':
    case 'gte':
    case 'lt':
    case 'lte':
      return `{ $${c.op}: ${literal(c.value, c.type)} }`
    case 'in':
    case 'nin':
      return `{ $${c.op}: ${listLiteral(c.value, c.type)} }`
    case 'regex':
      if (c.value === '') throw new BuilderError('Enter a regular expression')
      try {
        new RegExp(c.value)
      } catch {
        throw new BuilderError(`"${c.value}" is not a valid regular expression`)
      }
      return `{ $regex: ${JSON.stringify(c.value)}, $options: "i" }`
    case 'exists':
      return `{ $exists: ${c.value.trim() === 'false' ? 'false' : 'true'} }`
    case 'type':
      if (!BSON_TYPES.includes(c.value.trim())) throw new BuilderError(`Unknown BSON type "${c.value}"`)
      return `{ $type: ${JSON.stringify(c.value.trim())} }`
  }
}

export function buildFilter(state: BuilderState): string {
  const active = state.conditions.filter((c) => c.field.trim() !== '')
  if (active.length === 0) return '{}'
  const parts = active.map((c) => {
    try {
      return { field: c.field.trim(), expr: conditionExpr(c) }
    } catch (e) {
      throw new BuilderError(`${c.field}: ${(e as Error).message}`)
    }
  })
  const single = (p: { field: string; expr: string }) => `{ ${key(p.field)}: ${p.expr} }`
  if (state.combinator === 'or' && parts.length > 1) {
    return `{ $or: [${parts.map(single).join(', ')}] }`
  }
  const unique = new Set(parts.map((p) => p.field)).size === parts.length
  if (unique) return `{ ${parts.map((p) => `${key(p.field)}: ${p.expr}`).join(', ')} }`
  return `{ $and: [${parts.map(single).join(', ')}] }`
}

export function buildSort(rules: SortRule[]): string {
  const active = rules.filter((r) => r.field.trim() !== '')
  if (active.length === 0) return ''
  return `{ ${active.map((r) => `${key(r.field.trim())}: ${r.dir}`).join(', ')} }`
}

export function buildProjection(fields: string[], mode: 'include' | 'exclude'): string {
  const active = fields.map((f) => f.trim()).filter((f) => f !== '')
  if (active.length === 0) return ''
  const v = mode === 'include' ? 1 : 0
  return `{ ${active.map((f) => `${key(f)}: ${v}`).join(', ')} }`
}
