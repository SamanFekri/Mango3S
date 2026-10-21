// Plain TypeScript mirrors of the Go DTOs. The Wails-generated classes in
// wailsjs/go/models.ts are only used at the call boundary (see backend.ts).

export interface Settings {
  theme: 'system' | 'light' | 'dark'
  pageSize: number
  queryTimeoutSeconds: number
  confirmDeletes: boolean
  defaultView: 'tree' | 'json' | 'table'
  /** Set once the user picks a default view in Settings; until then Tree is used. */
  defaultViewChosen: boolean
  editorFontSize: number
  logLevel: 'debug' | 'info' | 'warn' | 'error'
  aiEnabled: boolean
  aiModel: string
  aiAllowSamples: boolean
  aiProvider: AIProviderId
  aiTimeoutSeconds: number
  claudeCodePath: string
  claudeCodeModel: string
  codexPath: string
  codexModel: string
  localUrl: string
  localApi: 'ollama' | 'openai'
  localModel: string
  schedulerEnabled: boolean
  /** Customized shortcuts: platform → action id → bindings (Preferences → Keymap). */
  keymap?: Partial<Record<'mac' | 'other', Record<string, string[]>>>

  /** Platform check-in (the web admin platform this build was distributed from). */
  autoUpdateEnabled: boolean
  deviceFingerprint?: string
  updateInstallationId?: string
  /** Back connections up to the signed-in Mango3S account in the background. */
  syncAuto: boolean
  /** Show Mango, the animated mascot, in the explorer. */
  showMascot: boolean
}

export type UpdateState =
  | 'idle'
  | 'checking'
  | 'up_to_date'
  | 'update_available'
  | 'downloading'
  | 'ready_to_install'
  | 'installing'
  | 'error'

export interface UpdateStatus {
  connected: boolean
  revoked: boolean
  state: UpdateState
  currentVersion: string
  latestVersion?: string
  releaseNotes?: string
  downloadDone?: number
  downloadTotal?: number
  error?: string
  lastCheckedAt?: string
}

/** One platform request recorded while MANGO3S_PLATFORM_TRACE is on (`make dev`); secrets are masked. */
export interface PlatformTraceEntry {
  id: number
  time: string
  method: string
  url: string
  /** 0 when no response arrived. */
  status: number
  durationMs: number
  error?: string
  requestHeaders?: Record<string, string>
  requestBody?: string
  responseHeaders?: Record<string, string>
  responseBody?: string
}

export interface PlatformDebug {
  baseUrl: string
  baseUrlSource: string
  traceEnabled: boolean
  registered: boolean
  installationId: string
  status: UpdateStatus
  requests: PlatformTraceEntry[]
}

/** One route of the web platform API (API console, test mode). */
export interface APIEndpoint {
  id: string
  group: string
  name: string
  method: string
  /** May contain {variables}. */
  path: string
  query?: string
  body?: string
  auth: 'none' | 'installation' | 'account' | 'device' | 'admin'
  writes: boolean
  multipart?: boolean
  description: string
  captures?: Record<string, string>
}

export interface APIConsoleState {
  enabled: boolean
  baseUrl: string
  endpoints: APIEndpoint[]
  vars: Record<string, string>
  /** Variables shown masked. */
  secret: string[]
}

export interface APIRequest {
  endpointId: string
  method: string
  path: string
  query: string
  body: string
  auth: string
}

export interface APIResponse {
  endpointId: string
  method: string
  url: string
  status: number
  statusText: string
  durationMs: number
  requestHeaders: Record<string, string>
  requestBody?: string
  responseHeaders?: Record<string, string>
  responseBody?: string
  error?: string
  captured?: Record<string, string>
}

/** One platform API the connection test can call on its own. */
export interface PlatformCheck {
  id: string
  name: string
  method: string
  path: string
  needsToken: boolean
  writes: boolean
  description: string
}

/** A category of files Mango3S keeps on disk (Settings › Diagnostics › Storage). */
export interface StorageItem {
  id: string
  label: string
  description: string
  bytes: number
  files: number
  clearable: boolean
  recommended: boolean
}

export interface StorageUsage {
  configDir: string
  items: StorageItem[]
}

export interface StorageCleanResult {
  freedBytes: number
  cleared: string[]
  skipped?: string[]
  usage: StorageUsage
}

export interface PlatformProbeStep {
  id: string
  name: string
  method: string
  path: string
  ok: boolean
  skipped: boolean
  status: number
  durationMs: number
  detail: string
  error?: string
}

export interface PlatformProbeReport {
  baseUrl: string
  ok: boolean
  steps: PlatformProbeStep[]
}

export type NotificationAction = 'open_link' | 'ok' | 'accept' | 'cancel'

export interface NotificationButton {
  id: string
  label: string
  action: NotificationAction
  url: string | null
  style: 'primary' | 'secondary'
}

export interface PlatformNotification {
  id: string
  title: string
  body: string
  /** 0–3 buttons, in order; a close (X) is always shown as well. */
  buttons?: NotificationButton[] | null
}

/** GET /v1/policies/privacy for this installation. */
export interface PrivacyPolicy {
  version: number
  title: string
  /** Markdown. */
  body: string
  summary: string | null
  publishedAt: string | null
  url: string
  acceptanceRequired: boolean
  acceptedAt: string | null
}

export interface Donation {
  url: string
  heading: string
  message: string
  buttonLabel: string
}

/** About / Support links from the Mango3S server; donation is null when turned off. */
export interface PlatformInfo {
  name: string
  websiteUrl: string
  downloadUrl: string
  privacyPolicyUrl: string
  contactUrl: string
  donation: Donation | null
}

export interface AccountMachine {
  machineId: string
  label: string | null
  lastSeenAt: string | null
}

export interface Account {
  clientId: string
  email: string
  createdAt: string
  recoveryKeyCreatedAt: string | null
  machines: AccountMachine[] | null
  sync: { enabled: boolean; itemCount: number; lastSyncedAt: string | null; maxItems: number; maxItemKb: number }
}

export type AccountFeature = 'registration' | 'login' | 'passwordReset' | 'sync'

export interface AccountState {
  signedIn: boolean
  email: string
  account: Account | null
  /** Features the server reported as turned off by the administrator. */
  disabled: Partial<Record<AccountFeature, boolean>> | null
  syncing: boolean
  lastSyncAt: string
  lastSyncError: string
  pendingChanges: number
  restoreSuggested: boolean
  privacyPending: boolean
}

export interface AccountResult {
  state: AccountState
  /** Shown once after sign-up (and after a reset that erased the data). */
  recoveryKey?: string
  dataKept?: boolean | null
}

export interface CodeSent {
  codeSentTo: string
  expiresInMinutes: number
}

export interface SyncReport {
  uploaded: number
  deleted: number
  added: number
  updated: number
  removed: number
  skipped: number
  errors: string[] | null
}

export interface ResetPasswordInput {
  email: string
  code: string
  newPassword: string
  recoveryKey: string
  discardSyncedData: boolean
}

export type ServerMode = 'uri' | 'manual'
export type ServerType = 'standalone' | 'replicaSet' | 'sharded' | 'srv'

export interface HostPort {
  host: string
  port: number
}

export interface ServerConfig {
  mode: ServerMode
  type: ServerType
  hosts: HostPort[]
  replicaSet: string
  directConnection: boolean
}

export interface TLSConfig {
  enabled: boolean
  caFile: string
  certKeyFile: string
  allowInvalidCertificates: boolean
  allowInvalidHostnames: boolean
}

export interface SSHConfig {
  enabled: boolean
  host: string
  port: number
  username: string
  authMethod: 'password' | 'privateKey' | 'agent'
  privateKeyFile: string
  hostKey: string
}

export interface ProxyConfig {
  type: 'none' | 'socks5' | 'http'
  host: string
  port: number
  username: string
}

export interface ShellConfig {
  defaultDb: string
  initScript: string
  runInitScript: boolean
}

export interface ToolsConfig {
  dir: string
  paths: Record<string, string>
}

export interface AdvancedConfig {
  serverSelectionTimeoutSeconds: number
  operationTimeoutSeconds: number
  heartbeatSeconds: number
  localThresholdMs: number
  appName: string
  readPreference: '' | 'primary' | 'primaryPreferred' | 'secondary' | 'secondaryPreferred' | 'nearest'
  readPreferenceTags: string
  maxStalenessSeconds: number
  readConcern: string
  writeConcernW: string
  writeConcernJournal: '' | 'true' | 'false'
  maxPoolSize: number
  minPoolSize: number
  maxConnecting: number
  maxIdleTimeSeconds: number
  retryWrites: '' | 'true' | 'false'
  retryReads: '' | 'true' | 'false'
  compressors: string[]
}

export type SecretKind = 'sshPassword' | 'sshPassphrase' | 'proxyPassword' | 'tlsKeyPassword' | 'awsSessionToken'

export interface Profile {
  id: string
  name: string
  uri: string
  username?: string
  authSource?: string
  authMechanism?: string
  authProperties?: string
  connectTimeoutSeconds: number
  readOnly: boolean
  hasPassword: boolean
  createdAt: string
  updatedAt: string
  folder?: string
  /** The user's order (drag to reorder); 0/absent = not placed yet. */
  position?: number
  server: ServerConfig
  tls: TLSConfig
  ssh: SSHConfig
  proxy: ProxyConfig
  shell: ShellConfig
  tools: ToolsConfig
  advanced: AdvancedConfig
  storedSecrets?: SecretKind[]
  lastConnectedAt?: string
}

export interface SecretInput {
  value: string
  clear: boolean
}

export interface ProfileInput {
  id: string
  name: string
  folder: string
  uri: string
  server: ServerConfig
  username: string
  password: string
  clearPassword: boolean
  authSource: string
  authMechanism: string
  authProperties: string
  connectTimeoutSeconds: number
  readOnly: boolean
  tls: TLSConfig
  ssh: SSHConfig
  proxy: ProxyConfig
  shell: ShellConfig
  tools: ToolsConfig
  advanced: AdvancedConfig
  secrets: Partial<Record<SecretKind, SecretInput>>
}

export interface ParsedURI {
  server: ServerConfig
  username: string
  hasPassword: boolean
  authSource: string
  authMechanism: string
  tls: boolean
  appName: string
  readPreference: string
  database: string
  unmapped: string[] | null
}

export interface ToolInfo {
  name: string
  purpose: string
  path: string
  source: string
  version: string
  status: 'ok' | 'missing' | 'incompatible' | 'error'
  message: string
}

export interface Capability {
  id: string
  label: string
  supported: boolean
  note?: string
}

export interface CollectionHistoryEntry {
  db: string
  coll: string
  lastOpened: string
  count: number
}

export interface FieldValue {
  exists: boolean
  type: string
  text: string
  expected: string
  editable: boolean
  reason?: string
}

export interface FieldUpdate {
  connId: string
  db: string
  coll: string
  id: string
  path: string[]
  type: string
  text: string
  value?: string
  expected: string
  expectedMissing: boolean
  unset: boolean
  renameTo?: string
  append?: boolean
}

export interface FieldResult {
  document: Doc
  previous: string
  current: string
  existed: boolean
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
  via?: string
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
  /** Name starts with "system."; shown in the explorer's System folder. */
  system: boolean
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
  /** find/aggregate results: re-run page by page with runShellPage. */
  pageable?: boolean
  page?: number
  pageSize?: number
}

/** A query from the collection tab's editor, run with paging. */
export interface QuerySpec {
  kind: 'find' | 'aggregate'
  db: string
  coll: string
  filter: string
  projection: string
  sort: string
  skip: number
  limit: number
  stages: Stage[] | null
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
  commit?: string
  buildTime?: string
  modified?: boolean
  goVersion?: string
  platform?: string
  executable?: string
  /** Test mode (`make dev`): the API console is available. */
  testMode?: boolean
  backupDir?: string
  components: Record<string, string>
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
  /** Timestamped log lines (long jobs such as backups); the oldest are dropped past 400. */
  log?: string[]
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

// ---- P2 ----------------------------------------------------------------------

export interface Endpoint {
  connId: string
  db: string
  coll: string
}

export interface CompareOptions {
  source: Endpoint
  target: Endpoint
  filter: string
  ignoreFields: string[]
  ignoreFieldOrder: boolean
  sampleSize: number
}

export interface FieldChange {
  path: string
  change: 'added' | 'removed' | 'changed'
  source?: string
  target?: string
}

export interface DocDiff {
  id: string
  kind: 'changed' | 'onlySource' | 'onlyTarget'
  fields: FieldChange[]
}

export interface CompareResult {
  options: CompareOptions
  sourceCount: number
  targetCount: number
  identical: number
  changed: number
  onlySource: number
  onlyTarget: number
  samples: DocDiff[]
  fingerprint: string
  durationMs: number
}

export interface DBCompareOptions {
  sourceConnId: string
  sourceDb: string
  targetConnId: string
  targetDb: string
  deep: boolean
}

export interface CollectionRow {
  name: string
  inSource: boolean
  inTarget: boolean
  sourceCount: number
  targetCount: number
  indexDiffs: string[]
  compared: boolean
  identical: number
  changed: number
  onlySource: number
  onlyTarget: number
  fingerprint?: string
}

export interface DBCompareResult {
  options: DBCompareOptions
  collections: CollectionRow[]
  durationMs: number
}

export interface SyncOptions {
  insert: boolean
  replace: boolean
  delete: boolean
}

export interface SyncPreview {
  planId: string
  target: string
  source: string
  inserts: number
  replaces: number
  deletes: number
  samples: DocDiff[]
  confirmText: string
  expiresInSec: number
}

export interface SyncCounts {
  inserts: number
  replaces: number
  deletes: number
}

export interface SyncOutcome {
  planned: SyncCounts
  applied: SyncCounts
  skipped: SyncCounts
  skippedIds: string[]
  matchesPreview: boolean
  backupPath: string
  verifiedEqual: boolean
  remaining: SyncCounts
  postFingerprint: string
  durationMs: number
}

export interface BackupInfo {
  mango3sBackup: number
  id: string
  created: string
  operation: string
  description: string
  db: string
  coll: string
  path: string
  replaced: number
  deleted: number
  inserted: number
  bytes: number
}

export interface TransformSpec {
  connId: string
  db: string
  coll: string
  filter: string
  mode: 'copy' | 'inPlace'
  targetColl: string
  onError: 'skip' | 'fail'
}

export interface ReschemaOp {
  type: 'rename' | 'remove' | 'set' | 'copy' | 'convert' | 'flatten' | 'nest'
  field: string
  to: string
  value: string
  onlyIfMissing: boolean
  targetType: string
  separator: string
  fields: string[]
}

export interface MaskRule {
  field: string
  method: string
  keepLast: number
  maskChar: string
  value: string
  maxDays: number
  percent: number
  onUnsupported: '' | 'error' | 'null' | 'skip'
}

export interface MaskMethod {
  id: string
  label: string
  types: string[]
  deterministic: boolean
  description: string
}

export interface MaskRuleSet {
  id: string
  name: string
  rules: MaskRule[]
  updatedAt: string
}

export interface TransformSample {
  id: string
  before: Doc
  after: Doc
  changed: boolean
  error?: string
}

export interface TransformPreviewData {
  total: number
  scanned: number
  changed: number
  errors: number
  errorSamples: string[]
  samples: TransformSample[]
  estimated: boolean
}

export interface TransformPreview {
  planId: string
  preview: TransformPreviewData
  target: string
  confirmText: string
}

export interface ValidationCheck {
  name: string
  ok: boolean
  detail: string
}

export interface TransformReport {
  mode: string
  scanned: number
  changed: number
  written: number
  skipped: number
  drift: number
  errorSamples: string[]
  backupPath?: string
  validation: ValidationCheck[]
  valid: boolean
  durationMs: number
  target: string
  createdTarget: boolean
}

export interface SQLSource {
  driver: 'postgres' | 'mysql' | 'sqlite'
  dsn: string
}

export interface SQLColumn {
  name: string
  dbType: string
  nullable: boolean
  suggest: string
}

export interface SQLTable {
  name: string
  columns: SQLColumn[]
  primaryKey: string[]
  foreignKeys: { columns: string[]; refTable: string; refColumns: string[] }[]
  rows: number
}

export interface ColumnMapping {
  column: string
  field: string
  type: string
}

export interface TableMapping {
  table: string
  collection: string
  idStrategy: 'pk' | 'objectId'
  columns: ColumnMapping[]
  embedOnly: boolean
}

export interface EmbedSpec {
  child: string
  parent: string
  childColumns: string[]
  as: string
}

export interface MigrationRequest {
  source: SQLSource
  connId: string
  db: string
  tables: TableMapping[]
  embeds: EmbedSpec[]
  onRowError: 'fail' | 'skip'
  batchSize: number
}

export interface MigrationTableReport {
  table: string
  collection: string
  rows: number
  inserted: number
  skipped: number
  errors: string[]
  countOk: boolean
  sampleChecked: number
  sampleMismatches: string[]
}

export interface MigrationRun {
  id: string
  created: string
  source: string
  connId: string
  db: string
  createdCollections: string[]
  state: 'running' | 'succeeded' | 'failed' | 'rolledBack'
  summary: string
  report?: {
    runId: string
    tables: MigrationTableReport[]
    embeds: { child: string; parent: string; as: string; childRows: number; embedded: number; orphans: number; countOk: boolean }[]
    valid: boolean
    durationMs: number
  }
}

export interface TaskType {
  id: string
  label: string
  description: string
}

export interface TaskSchedule {
  kind: 'interval' | 'cron'
  everyMinutes: number
  cron: string
}

export interface Task {
  id: string
  name: string
  type: string
  params: unknown
  schedule: TaskSchedule
  enabled: boolean
  retry: { maxAttempts: number; backoffSeconds: number }
  misfire: 'skip' | 'runOnce'
  approvedHash: string
  approvedAt?: string
  nextRun: string
  createdAt: string
  updatedAt: string
}

export interface TaskRun {
  id: string
  taskId: string
  taskName: string
  trigger: 'schedule' | 'manual' | 'misfire'
  state: 'running' | 'succeeded' | 'failed' | 'skipped' | 'missed' | 'interrupted' | 'canceled'
  attempts: number
  errors: string[]
  result?: string
  jobId?: string
  startedAt: string
  finishedAt: string
}

export type AIProviderId = 'anthropic' | 'claudeCode' | 'codex' | 'local'

export interface AIStatus {
  enabled: boolean
  provider: AIProviderId
  providerName: string
  ready: boolean
  detail: string
  model: string
  /** The context goes to a remote service (false for a model on this computer). */
  leavesDevice: boolean
  hasKey: boolean
  keySource: 'stored' | 'environment' | 'none'
  allowSamples: boolean
  localHasKey: boolean
}

export interface CLIInfo {
  name: string
  found: boolean
  path: string
  version: string
  signedIn: boolean | null
  account: string
  message: string
}

export interface LocalServer {
  name: string
  url: string
  api: 'ollama' | 'openai'
  models: string[]
}

export interface AIProviders {
  claudeCode: CLIInfo
  codex: CLIInfo
  localServers: LocalServer[]
}

export interface AIRequest {
  connId: string
  db: string
  coll: string
  prompt: string
  mode: 'auto' | 'find' | 'aggregate'
  includeSamples: boolean
  allowWrites: boolean
}

export interface AIContext {
  collection: string
  fields: { path: string; types: string[]; presence: number }[]
  samples: string[]
}

export interface AIProposal {
  id: string
  kind: 'find' | 'aggregate' | 'updateMany' | 'deleteMany'
  db: string
  coll: string
  filter: string
  projection: string
  sort: string
  limit: number
  pipeline: Stage[]
  update: string
  explanation: string
  warnings: string[]
  isWrite: boolean
  requiresConfirmation: boolean
  valid: boolean
  validation: ValidationCheck[]
  shell: string
  sent: AIContext
  model: string
  inputTokens: number
  outputTokens: number
  durationMs: number
}

export interface AIWritePreview {
  proposalId: string
  shell: string
  affected: number
  sampleIds: string[]
  confirmText: string
}

export interface AIWriteResult {
  matched: number
  modified: number
  deleted: number
  backupPath: string
}

export interface MCPPolicy {
  enabled: boolean
  profiles: string[]
  databases: string[]
  tools: string[]
  allowWrites: boolean
  maxDocs: number
  maxWriteDocs: number
  maskRuleSetId: string
  auditLog: boolean
}

export interface MCPInfo {
  policy: MCPPolicy
  catalog: { name: string; write: boolean; description: string }[]
  command: string
  args: string[]
  snippet: string
}

/** Update dialog / Remove dialog. */
export interface BulkWriteRequest {
  connId: string
  db: string
  coll: string
  kind: 'update' | 'delete'
  scope: 'selected' | 'query'
  ids: string[]
  filter: string
  update: string
}

export interface BulkPreview {
  affected: number
  sampleIds: string[]
  confirmText: string
  shell: string
}

export interface BulkResult {
  matched: number
  modified: number
  deleted: number
  backupPath: string
}

/** One stage of testing or opening a connection (the connection log). */
export interface ConnectionStep {
  id: string
  label: string
  state: 'pending' | 'running' | 'ok' | 'warning' | 'failed' | 'skipped'
  detail?: string
  error?: string
  code?: string
  durationMs: number
}

// ---- backup & restore -----------------------------------------------------------

export type BackupEngine = 'builtin' | 'tools'

export interface NamespaceSelection {
  databases: string[]
  include: string[]
  exclude: string[]
  includeSystem: boolean
  views: boolean
}

export interface BuiltinBackupOptions {
  dir: string
  gzip: boolean
  selection: NamespaceSelection
  query: string
  source?: string
}

export interface ToolDumpOptions {
  archive: string
  out: string
  gzip: boolean
  db: string
  collection: string
  excludeCollections: string[]
  query: string
  usersAndRoles: boolean
  oplog: boolean
  viewsAsCollections: boolean
}

export interface BackupRequest {
  connId: string
  engine: BackupEngine
  builtin: BuiltinBackupOptions
  tool: ToolDumpOptions
}

export type RestoreMode = 'insert' | 'upsert' | 'drop'

export interface BuiltinRestoreOptions {
  dir: string
  mode: RestoreMode
  selection: NamespaceSelection
  renames: { from: string; to: string }[]
  skipIndexes: boolean
  skipOptions: boolean
  stopOnError: boolean
  bypassValidate: boolean
}

export interface ToolRestoreOptions {
  archive: string
  dir: string
  gzip: boolean
  nsInclude: string[]
  nsExclude: string[]
  nsFrom: string[]
  nsTo: string[]
  drop: boolean
  noIndexRestore: boolean
  noOptionsRestore: boolean
  stopOnError: boolean
  bypassValidation: boolean
  usersAndRoles: boolean
  oplogReplay: boolean
  dryRun: boolean
}

export interface RestoreRequest {
  connId: string
  engine: BackupEngine
  builtin: BuiltinRestoreOptions
  tool: ToolRestoreOptions
}

export interface BackupNamespace {
  db: string
  coll: string
  type: string
  count: number
  bytes: number
  indexes: number
  gzip: boolean
  error?: string
  expected?: number
}

export interface BackupManifest {
  format: string
  version: number
  created: string
  tool: string
  serverVersion?: string
  source?: string
  gzip: boolean
  namespaces: BackupNamespace[]
  notes?: string[]
}

export interface BackupInspection {
  dir: string
  manifest?: BackupManifest
  entries: BackupNamespace[]
  warnings: string[]
}

export interface BackupTools {
  dump: ToolInfo
  restore: ToolInfo
  blocked: string
}

// ---- connection import / export ------------------------------------------------

export interface ConnectionExportResult {
  path: string
  count: number
  secretsIncluded: boolean
}

export interface ConnectionImportItem {
  index: number
  name: string
  folder: string
  hosts: string
  via: string
  secretKinds: string[]
  conflict: '' | 'id' | 'name'
  existingId: string
  notes: string[]
  error: string
}

export interface ConnectionImportPreview {
  path: string
  format: 'mango3s' | 'compass' | 'uris'
  hasSecrets: boolean
  secretsUnlocked: boolean
  items: ConnectionImportItem[]
  warnings: string[]
}

export interface ImportChoice {
  index: number
  action: 'add' | 'replace' | 'skip'
  name: string
  folder: string
}

export interface ConnectionImportOutcome {
  added: number
  replaced: number
  skipped: number
  errors: string[]
}

// ---- database & collection management --------------------------------------

export interface CreateCollectionOptions {
  name: string
  capped: boolean
  sizeBytes: number
  maxDocs: number
  collation: string
  validator: string
  validationLevel: string
  validationAction: string
  timeField: string
  metaField: string
  granularity: string
  expireAfterSeconds: number
  clustered: boolean
  storageEngine: string
}

export interface Validation {
  validator: string
  level: 'off' | 'strict' | 'moderate'
  action: 'error' | 'warn' | 'errorAndLog'
}

export interface ValidationReport {
  checked: number
  failing: number
  sample: string[]
}

export interface IndexDetail {
  name: string
  keys: string
  spec: string
  unique: boolean
  sparse: boolean
  hidden: boolean
  ttl?: number
  partial?: string
  collation?: string
  size: number
  accesses: number
  accessSince?: string
}

export interface IndexOptions {
  keys: string
  name: string
  unique: boolean
  sparse: boolean
  hidden: boolean
  expireAfterSeconds: number | null
  partialFilterExpression: string
  collation: string
  wildcardProjection: string
  extra: string
}

export interface CollectionDetails {
  type: string
  options: string
  uuid?: string
  readOnly: boolean
  stats: Record<string, string>
  count: number
  size: number
  storageSize: number
  totalIndexSize: number
  avgObjSize: number
  capped: boolean
  sharded: boolean
  shell: string
}

export interface DatabaseDetails {
  collections: number
  views: number
  objects: number
  dataSize: number
  storageSize: number
  indexes: number
  indexSize: number
  avgObjSize: number
  fsUsedSize: number
  fsTotalSize: number
}

export interface CopyOptions {
  srcDb: string
  collections: string[]
  dstDb: string
  dstColl: string
  mode: RestoreMode
  filter: string
  skipIndexes: boolean
  skipOptions: boolean
  views: boolean
}

// ---- profiler & SQL export ------------------------------------------------------

export interface Profiling {
  level: 0 | 1 | 2
  slowms: number
  sampleRate: number
}

export interface ProfileQuery {
  coll: string
  op: string
  minMillis: number
  limit: number
}

export interface ProfileEntry {
  ts: string
  op: string
  ns: string
  millis: number
  planSummary: string
  keysExamined: number
  docsExamined: number
  nreturned: number
  command: string
  client: string
  user: string
  appName: string
}

export type SQLDialect = 'postgres' | 'mysql' | 'sqlite'

export interface SQLExportOptions {
  dialect: SQLDialect
  table: string
  filter: string
  sampleSize: number
  batchRows: number
  flatten: boolean
  dropTable: boolean
}

// ---- index advisor ----------------------------------------------------------------

export interface QueryShape {
  coll: string
  op: string
  shape: string
  count: number
  totalMillis: number
  avgMillis: number
  maxMillis: number
  docsExamined: number
  keysExamined: number
  returned: number
  plans: string[]
  example: string
  lastSeen: string
  status: 'missing' | 'partial' | 'covered' | 'hidden' | 'none'
  suggested?: string
  name?: string
  servedBy?: string
  reason: string
}

export interface IndexFinding {
  coll: string
  name: string
  keys: string
  reason: string
  size: number
  hidden: boolean
}

export interface IndexAdvice {
  profilingLevel: number
  analyzed: number
  since?: string
  shapes: QueryShape[]
  unused: IndexFinding[]
  redundant: IndexFinding[]
  notes: string[]
}
