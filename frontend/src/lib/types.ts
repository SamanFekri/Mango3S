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
