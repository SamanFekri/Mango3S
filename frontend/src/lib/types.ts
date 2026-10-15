// Plain TypeScript mirrors of the Go DTOs. The Wails-generated classes in
// wailsjs/go/models.ts are only used at the call boundary (see backend.ts).

export interface Settings {
  theme: 'system' | 'light' | 'dark'
  pageSize: number
  queryTimeoutSeconds: number
  confirmDeletes: boolean
  defaultView: 'json' | 'table'
  editorFontSize: number
  logLevel: 'debug' | 'info' | 'warn' | 'error'
}

export interface Profile {
  id: string
  name: string
  uri: string
  username?: string
  authSource?: string
  authMechanism?: string
  connectTimeoutSeconds: number
  readOnly: boolean
  hasPassword: boolean
  createdAt: string
  updatedAt: string
}

export interface ProfileInput {
  id: string
  name: string
  uri: string
  username: string
  password: string
  clearPassword: boolean
  authSource: string
  authMechanism: string
  connectTimeoutSeconds: number
  readOnly: boolean
}

export interface SessionInfo {
  id: string
  name: string
  readOnly: boolean
  serverVersion: string
  topology: string
  connectedAt: string
}

export interface TestResult {
  serverVersion: string
  topology: string
  latencyMs: number
}

export interface DatabaseInfo {
  name: string
  sizeOnDisk: number
  empty: boolean
}

export interface CollectionInfo {
  name: string
  type: string
  readOnly: boolean
}

export interface IndexInfo {
  name: string
  keys: string
  unique: boolean
}

export interface CollectionStats {
  count: number
  size: number
  avgObjSize: number
  storageSize: number
  totalIndexSize: number
  indexes: IndexInfo[]
}

export interface Doc {
  /** _id in shell syntax; empty when the document has no _id. */
  id: string
  /** Pretty, round-trippable shell syntax. */
  text: string
  /** Relaxed Extended JSON. */
  json: string
}

export interface Page {
  documents: Doc[]
  page: number
  pageSize: number
  hasMore: boolean
  durationMs: number
  notice?: string
}

export interface QueryRequest {
  connId: string
  db: string
  coll: string
  filter: string
  projection: string
  sort: string
  skip: number
  limit: number
  page: number
  pageSize: number
}

export interface Stage {
  operator: string
  body: string
  enabled: boolean
}

export interface AggregateRequest {
  connId: string
  db: string
  coll: string
  stages: Stage[]
  upTo: number
  page: number
  pageSize: number
  allowDiskUse: boolean
}

export interface ShellOutput {
  statement: string
  kind: 'documents' | 'text' | 'error'
  text?: string
  documents?: Doc[]
  hasMore: boolean
  durationMs: number
}

export interface ShellResult {
  outputs: ShellOutput[]
  currentDb: string
}

export interface AppInfo {
  version: string
  configDir: string
  logFile: string
  secretBackend: string
}

// ---- P1 ----------------------------------------------------------------------

export type JobState = 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled'

export interface Job {
  id: string
  kind: string
  title: string
  state: JobState
  done: number
  total: number
  unit: string
  message: string
  error?: string
  result?: string
  createdAt: string
  startedAt: string
  finishedAt: string
}

export type TransferFormat = 'json' | 'csv'

export interface ColumnSpec {
  name: string
  type: string
  skip: boolean
}

export interface ImportOptions {
  connId: string
  db: string
  coll: string
  path: string
  format: TransferFormat | ''
  delimiter: string
  columns: ColumnSpec[]
  keepBlanks: boolean
  mode: 'insert' | 'upsert'
  dropFirst: boolean
  stopOnError: boolean
  batchSize: number
}

export interface ExportOptions {
  connId: string
  db: string
  coll: string
  filter: string
  projection: string
  sort: string
  skip: number
  limit: number
  path: string
  format: TransferFormat
  jsonMode: 'array' | 'lines'
  canonical: boolean
  pretty: boolean
  delimiter: string
  noHeader: boolean
  fields: string[]
}

export interface ColumnPreview {
  name: string
  inferred: string
  samples: string[]
}

export interface ImportPreview {
  format: TransferFormat
  sizeBytes: number
  documents: Doc[]
  columns: ColumnPreview[]
  error?: string
}

export interface TypeStat {
  type: string
  count: number
  percent: number
}

export interface SchemaField {
  path: string
  name: string
  depth: number
  count: number
  presence: number
  types: TypeStat[]
  elementTypes?: TypeStat[]
  minLength?: number
  maxLength?: number
  avgLength?: number
  min?: string
  max?: string
  distinct: number
  distinctCapped: boolean
  unique: boolean
  samples: string[]
}

export interface SchemaReport {
  sampled: number
  estimatedDocs: number
  fields: SchemaField[]
  durationMs: number
}

export interface SQLTranslation {
  kind: 'find' | 'aggregate'
  collection: string
  shell: string
}

export interface SQLResult {
  translation: SQLTranslation
  page: Page
}

export interface CodeLanguage {
  id: string
  label: string
  ext: string
}

export interface CodeRequest {
  language: string
  source: 'query' | 'aggregate' | 'sql'
  db: string
  coll: string
  query?: QueryRequest
  stages?: Stage[]
  sql?: string
}

export interface ExplainDetail {
  key: string
  value: string
}

export interface ExplainStage {
  name: string
  details: ExplainDetail[]
  nReturned?: number
  docsExamined?: number
  keysExamined?: number
  timeMs?: number
  children: ExplainStage[]
}

export interface ExplainPlan {
  namespace: string
  root: ExplainStage | null
  pipeline: ExplainStage[]
  summary: {
    executionTimeMs?: number
    nReturned?: number
    docsExamined?: number
    keysExamined?: number
    indexesUsed: string[]
    collScan: boolean
    inMemorySort: boolean
    covered: boolean
    rejectedPlans: number
    engine: string
  }
  diagnostics: { level: 'good' | 'info' | 'warning'; title: string; detail: string }[]
  raw: string
}

export interface ExplainRequest {
  kind: 'find' | 'aggregate'
  query?: QueryRequest
  connId?: string
  db?: string
  coll?: string
  stages?: Stage[]
}

export type ItemKind = 'query' | 'aggregation' | 'script' | 'sql'

export interface QueryPayload {
  filter: string
  projection: string
  sort: string
  skip: number
  limit: number
}

export interface SavedItem {
  id: string
  kind: ItemKind
  name: string
  description: string
  db: string
  coll: string
  query?: QueryPayload
  stages?: Stage[]
  text?: string
  createdAt: string
  updatedAt: string
}

export interface HistoryEntry {
  id: string
  kind: ItemKind
  connName: string
  db: string
  coll: string
  summary: string
  query?: QueryPayload
  stages?: Stage[]
  text?: string
  at: string
  durationMs: number
  ok: boolean
  error?: string
}
