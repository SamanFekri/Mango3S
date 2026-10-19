// Context-aware completion for the shell editor. Pure functions so the
// rules can be unit-tested without an editor.

export interface ShellOption {
  label: string
  type: 'method' | 'property' | 'keyword' | 'function' | 'variable'
  detail?: string
  /** Text inserted instead of the label (e.g. "find(" to open the call). */
  apply?: string
}

export interface ShellCompletion {
  /** Offset into the text where the completed word starts. */
  from: number
  options: ShellOption[]
}

export interface ShellContext {
  collections: string[]
  /** Field names of the collection referenced before the cursor, if known. */
  fields: string[]
}

const m = (label: string, detail: string): ShellOption => ({ label, type: 'method', detail, apply: `${label}(` })

export const DB_METHODS: ShellOption[] = [
  m('getCollection', '(name)'),
  m('getCollectionNames', '()'),
  m('getSiblingDB', '(name)'),
  m('getName', '()'),
  m('stats', '()'),
  m('version', '()'),
  m('serverStatus', '()'),
  m('runCommand', '(command)'),
  m('adminCommand', '(command)'),
  m('createCollection', '(name, options?)'),
  m('dropDatabase', '()'),
]

export const COLLECTION_METHODS: ShellOption[] = [
  m('find', '(filter?, projection?)'),
  m('findOne', '(filter?, projection?)'),
  m('countDocuments', '(filter?)'),
  m('estimatedDocumentCount', '()'),
  m('distinct', '(field, filter?)'),
  m('aggregate', '(pipeline, options?)'),
  m('insertOne', '(document)'),
  m('insertMany', '([documents])'),
  m('updateOne', '(filter, update, { upsert? })'),
  m('updateMany', '(filter, update, { upsert? })'),
  m('replaceOne', '(filter, replacement, { upsert? })'),
  m('deleteOne', '(filter)'),
  m('deleteMany', '(filter)'),
  m('createIndex', '(keys, { name?, unique?, sparse?, expireAfterSeconds? })'),
  m('getIndexes', '()'),
  m('dropIndex', '(name | keys)'),
  m('drop', '()'),
]

export const CURSOR_METHODS: ShellOption[] = [
  m('sort', '({ field: 1 | -1 })'),
  m('limit', '(n)'),
  m('skip', '(n)'),
  m('projection', '({ field: 1 })'),
  m('count', '()'),
  m('explain', '()'),
  m('toArray', '()'),
  m('pretty', '()'),
]

const OPERATORS = [
  '$eq', '$ne', '$gt', '$gte', '$lt', '$lte', '$in', '$nin', '$and', '$or', '$nor', '$not', '$exists', '$type',
  '$regex', '$options', '$elemMatch', '$size', '$all', '$expr', '$set', '$unset', '$inc', '$push', '$pull',
  '$addToSet', '$rename', '$match', '$project', '$group', '$sort', '$limit', '$skip', '$unwind', '$lookup',
  '$addFields', '$count', '$facet', '$sum', '$avg', '$min', '$max', '$first', '$last',
].map((o): ShellOption => ({ label: o, type: 'keyword' }))

const HELPERS: ShellOption[] = ['ObjectId', 'ISODate', 'NumberLong', 'NumberInt', 'NumberDecimal', 'UUID', 'Timestamp'].map((h) => ({
  label: h,
  type: 'function',
  apply: `${h}(`,
}))

const STATEMENTS: ShellOption[] = [
  { label: 'db', type: 'variable' },
  { label: 'show dbs', type: 'keyword' },
  { label: 'show collections', type: 'keyword' },
  { label: 'use', type: 'keyword', apply: 'use ' },
  { label: 'help', type: 'keyword' },
]

const IDENT = /^[A-Za-z_$][\w$]*$/

/** The collection referenced by the statement that contains the cursor. */
export function currentCollection(text: string): string | null {
  const stmt = text.slice(Math.max(text.lastIndexOf(';'), text.lastIndexOf('\n\n')) + 1)
  const re = /db\.(?:getCollection\(\s*["']([^"']+)["']\s*\)|([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*?))\.[A-Za-z_$][\w$]*\s*\(/g
  let found: string | null = null
  let match: RegExpExecArray | null
  while ((match = re.exec(stmt))) found = match[1] ?? match[2]
  return found
}

/** Computes completions for the text before the cursor. */
export function shellCompletions(before: string, ctx: ShellContext): ShellCompletion | null {
  let match: RegExpMatchArray | null

  // show <what>
  if ((match = before.match(/(?:^|\n)\s*show\s+(\w*)$/))) {
    return {
      from: before.length - match[1].length,
      options: [
        { label: 'dbs', type: 'keyword' },
        { label: 'collections', type: 'keyword' },
      ],
    }
  }

  // db.getCollection("x").<here> is a collection, not a cursor.
  if ((match = before.match(/\bdb\.getCollection\(\s*["'][^"']*["']\s*\)\s*\.\s*([\w$]*)$/))) {
    return { from: before.length - match[1].length, options: COLLECTION_METHODS }
  }

  // cursor modifiers after a call: db.coll.find({...}).<here>
  if ((match = before.match(/\)\s*\.\s*([\w$]*)$/))) {
    return { from: before.length - match[1].length, options: CURSOR_METHODS }
  }

  // collection methods: db.<coll>.<here> or db.getCollection("x").<here>
  if ((match = before.match(/\bdb\.(?:getCollection\([^)]*\)|[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*)\.([\w$]*)$/))) {
    const word = match[1]
    const collPart = before.slice(0, before.length - word.length - 1).replace(/^[\s\S]*\bdb\./, '')
    // "db.users." could also be the start of a dotted collection name; offer both.
    const nested = ctx.collections
      .filter((c) => c.startsWith(collPart + '.'))
      .map((c): ShellOption => ({ label: c.slice(collPart.length + 1), type: 'property', detail: 'collection' }))
    return { from: before.length - word.length, options: [...COLLECTION_METHODS, ...nested] }
  }

  // db.<here>: collections and database methods
  if ((match = before.match(/\bdb\.([\w$]*)$/))) {
    const colls = ctx.collections.map((c): ShellOption => {
      const label = IDENT.test(c) ? c : `getCollection("${c}")`
      return { label, type: 'property', detail: 'collection' }
    })
    return { from: before.length - match[1].length, options: [...colls, ...DB_METHODS] }
  }

  // Anything else: operators, BSON helpers, field names, statements.
  match = before.match(/[$\w.]*$/)
  const word = match ? match[0] : ''
  const atStatementStart = /(?:^|[\n;])\s*[\w$]*$/.test(before)
  if (word === '' && !atStatementStart) return null
  const fields = ctx.fields.map((f): ShellOption => ({ label: f, type: 'property', detail: 'field' }))
  const options = word.startsWith('$') ? OPERATORS : [...fields, ...HELPERS, ...OPERATORS, ...(atStatementStart ? STATEMENTS : [])]
  return { from: before.length - word.length, options }
}

/**
 * Walks through history like a terminal: older = true moves back in time.
 * index -1 means "not navigating" (the current draft).
 */
export function historyStep(history: string[], index: number, older: boolean): number {
  if (history.length === 0) return -1
  if (older) return Math.min(index + 1, history.length - 1)
  return Math.max(index - 1, -1)
}
