// Typed wrapper around the Wails-generated bindings. All UI code calls the
// backend through this module so errors are normalized in one place.
import * as App from '../../wailsjs/go/api/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { toAppError } from './errors'
import type {
  AggregateRequest,
  AppInfo,
  CollectionInfo,
  CollectionStats,
  DatabaseInfo,
  AIContext,
  AIProposal,
  AIRequest,
  AIStatus,
  AIWritePreview,
  AIWriteResult,
  BackupInfo,
  BackupInspection,
  BackupRequest,
  BackupTools,
  CodeLanguage,
  CollectionDetails,
  CompareOptions,
  CopyOptions,
  CreateCollectionOptions,
  DatabaseDetails,
  ConnectionExportResult,
  ConnectionImportOutcome,
  ConnectionImportPreview,
  CompareResult,
  DBCompareOptions,
  DBCompareResult,
  ImportChoice,
  IndexAdvice,
  ProfileEntry,
  ProfileQuery,
  Profiling,
  SQLExportOptions,
  IndexDetail,
  IndexOptions,
  MaskMethod,
  MaskRule,
  MaskRuleSet,
  MCPInfo,
  MCPPolicy,
  MigrationRequest,
  MigrationRun,
  ReschemaOp,
  RestoreRequest,
  SQLSource,
  SQLTable,
  SyncOptions,
  SyncOutcome,
  SyncPreview,
  Task,
  TaskRun,
  TaskType,
  TransformPreview,
  Validation,
  ValidationReport,
  TransformReport,
  TransformSpec,
  CodeRequest,
  Doc,
  ExplainPlan,
  ExplainRequest,
  ExportOptions,
  HistoryEntry,
  ImportOptions,
  ImportPreview,
  Job,
  Page,
  SavedItem,
  SchemaReport,
  SQLResult,
  SQLTranslation,
  Profile,
  ProfileInput,
  ConnectionStep,
  BulkPreview,
  BulkResult,
  BulkWriteRequest,
  AIProviders,
  QuerySpec,
  ShellOutput,
  Capability,
  CollectionHistoryEntry,
  FieldResult,
  FieldUpdate,
  FieldValue,
  ParsedURI,
  ServerConfig,
  ToolInfo,
  ToolsConfig,
  QueryRequest,
  SessionInfo,
  Settings,
  ShellResult,
  Stage,
  TestResult,
} from './types'

async function wrap<T>(p: Promise<unknown>): Promise<T> {
  try {
    return (await p) as T
  } catch (err) {
    throw toAppError(err)
  }
}

// The generated signatures use Wails model classes; our plain objects have the
// same shape, so we pass them through `any` at this single boundary.
const a = App as any

export const backend = {
  getAppInfo: () => wrap<AppInfo>(a.GetAppInfo()),
  getSettings: () => wrap<Settings>(a.GetSettings()),
  saveSettings: (s: Settings) => wrap<Settings>(a.SaveSettings(s)),

  listProfiles: () => wrap<Profile[]>(a.ListProfiles()),
  saveProfile: (p: ProfileInput) => wrap<Profile>(a.SaveProfile(p)),
  deleteProfile: (id: string) => wrap<void>(a.DeleteProfile(id)),
  testConnection: (p: ProfileInput) => wrap<TestResult>(a.TestConnection(p)),
  testConnectionWithLog: (opId: string, p: ProfileInput) => wrap<TestResult>(a.TestConnectionWithLog(opId, p)),
  connectWithLog: (opId: string, profileId: string) => wrap<SessionInfo>(a.ConnectWithLog(opId, profileId)),
  duplicateProfile: (id: string) => wrap<Profile>(a.DuplicateProfile(id)),
  reorderProfiles: (ids: string[]) => wrap<Profile[]>(a.ReorderProfiles(ids)),
  setProfileFolder: (id: string, folder: string) => wrap<Profile>(a.SetProfileFolder(id, folder)),
  parseConnectionString: (uri: string) => wrap<ParsedURI>(a.ParseConnectionString(uri)),
  buildConnectionString: (s: ServerConfig) => wrap<string>(a.BuildConnectionString(s)),
  detectTools: (cfg: ToolsConfig, connId: string) => wrap<ToolInfo[]>(a.DetectTools(cfg, connId)),
  connectionCapabilities: () => wrap<Record<string, Capability[]>>(a.ConnectionCapabilities()),
  recordCollectionOpened: (connId: string, db: string, coll: string) => wrap<void>(a.RecordCollectionOpened(connId, db, coll)),
  listCollectionHistory: (profileId: string) => wrap<CollectionHistoryEntry[]>(a.ListCollectionHistory(profileId)),
  clearCollectionHistory: (profileId: string) => wrap<void>(a.ClearCollectionHistory(profileId)),
  getField: (connId: string, db: string, coll: string, id: string, path: string[]) => wrap<FieldValue>(a.GetField(connId, db, coll, id, path)),
  exportTextFile: (title: string, defaultName: string, content: string) => wrap<string>(a.ExportTextFile(title, defaultName, content)),
  importTextFile: (title: string) => wrap<string>(a.ImportTextFile(title)),
  previewBulkWrite: (r: BulkWriteRequest) => wrap<BulkPreview>(a.PreviewBulkWrite(r)),
  executeBulkWrite: (r: BulkWriteRequest, confirm: string, expected: number) => wrap<BulkResult>(a.ExecuteBulkWrite(r, confirm, expected)),
  updateField: (u: FieldUpdate) => wrap<FieldResult>(a.UpdateField(u)),
  setUnsavedWork: (dirty: boolean) => wrap<void>(a.SetUnsavedWork(dirty)),
  chooseFile: (kind: 'cert' | 'key' | 'tool' | 'folder', title: string) => wrap<string>(a.ChooseFile(kind, title)),
  connect: (id: string) => wrap<SessionInfo>(a.Connect(id)),
  disconnect: (id: string) => wrap<void>(a.Disconnect(id)),
  listConnections: () => wrap<SessionInfo[]>(a.ListConnections()),

  listDatabases: (conn: string) => wrap<DatabaseInfo[]>(a.ListDatabases(conn)),
  listCollections: (conn: string, db: string) => wrap<CollectionInfo[]>(a.ListCollections(conn, db)),
  collectionStats: (conn: string, db: string, coll: string) =>
    wrap<CollectionStats>(a.CollectionStats(conn, db, coll)),
  sampleFields: (conn: string, db: string, coll: string) => wrap<string[]>(a.SampleFields(conn, db, coll)),
  createCollection: (conn: string, db: string, coll: string) => wrap<void>(a.CreateCollection(conn, db, coll)),
  dropCollection: (conn: string, db: string, coll: string) => wrap<void>(a.DropCollection(conn, db, coll)),
  dropDatabase: (conn: string, db: string) => wrap<void>(a.DropDatabase(conn, db)),

  runQuery: (opId: string, req: QueryRequest) => wrap<Page>(a.RunQuery(opId, req)),
  countQuery: (opId: string, req: QueryRequest) => wrap<number>(a.CountQuery(opId, req)),
  queryToShell: (req: QueryRequest) => wrap<string>(a.QueryToShell(req)),

  getDocument: (conn: string, db: string, coll: string, id: string) => wrap<Doc>(a.GetDocument(conn, db, coll, id)),
  insertDocuments: (conn: string, db: string, coll: string, text: string) =>
    wrap<string[]>(a.InsertDocuments(conn, db, coll, text)),
  replaceDocument: (conn: string, db: string, coll: string, id: string, text: string) =>
    wrap<Doc>(a.ReplaceDocument(conn, db, coll, id, text)),
  deleteDocuments: (conn: string, db: string, coll: string, ids: string[]) =>
    wrap<number>(a.DeleteDocuments(conn, db, coll, ids)),
  formatDocument: (text: string) => wrap<string>(a.FormatDocument(text)),

  runAggregation: (opId: string, req: AggregateRequest) => wrap<Page>(a.RunAggregation(opId, req)),
  pipelineToText: (stages: Stage[]) => wrap<string>(a.PipelineToText(stages)),
  pipelineFromText: (text: string) => wrap<Stage[]>(a.PipelineFromText(text)),

  runShellPage: (opId: string, conn: string, db: string, statement: string, page: number) =>
    wrap<ShellOutput>(a.RunShellPage(opId, conn, db, statement, page)),
  parseCollectionQuery: (text: string, db: string) => wrap<QuerySpec>(a.ParseCollectionQuery(text, db)),
  runShell: (opId: string, conn: string, db: string, script: string) =>
    wrap<ShellResult>(a.RunShell(opId, conn, db, script)),
  cancel: (opId: string) => wrap<boolean>(a.CancelOperation(opId)),

  // P1
  listJobs: () => wrap<Job[]>(a.ListJobs()),
  cancelJob: (id: string) => wrap<boolean>(a.CancelJob(id)),
  clearFinishedJobs: () => wrap<void>(a.ClearFinishedJobs()),
  chooseImportFile: () => wrap<string>(a.ChooseImportFile()),
  chooseExportFile: (defaultName: string) => wrap<string>(a.ChooseExportFile(defaultName)),
  previewImport: (o: ImportOptions) => wrap<ImportPreview>(a.PreviewImport(o)),
  startImport: (o: ImportOptions) => wrap<Job>(a.StartImport(o)),
  startExport: (o: ExportOptions) => wrap<Job>(a.StartExport(o)),
  backupToolsStatus: (connId: string) => wrap<BackupTools>(a.BackupToolsStatus(connId)),
  startBackup: (r: BackupRequest) => wrap<Job>(a.StartBackup(r)),
  inspectBackup: (dir: string) => wrap<BackupInspection>(a.InspectBackup(dir)),
  startRestoreBackup: (r: RestoreRequest, confirm: string) => wrap<Job>(a.StartRestoreBackup(r, confirm)),
  chooseSaveFile: (title: string, defaultName: string) => wrap<string>(a.ChooseSaveFile(title, defaultName)),
  exportConnections: (ids: string[], includeSecrets: boolean, passphrase: string) =>
    wrap<ConnectionExportResult>(a.ExportConnections(ids, includeSecrets, passphrase)),
  chooseConnectionsFile: () => wrap<string>(a.ChooseConnectionsFile()),
  previewConnectionImport: (path: string, passphrase: string) => wrap<ConnectionImportPreview>(a.PreviewConnectionImport(path, passphrase)),
  applyConnectionImport: (path: string, passphrase: string, choices: ImportChoice[]) =>
    wrap<ConnectionImportOutcome>(a.ApplyConnectionImport(path, passphrase, choices)),
  connectionURI: (id: string, withPassword: boolean) => wrap<string>(a.ConnectionURI(id, withPassword)),
  createCollectionWithOptions: (conn: string, db: string, o: CreateCollectionOptions) => wrap<void>(a.CreateCollectionWithOptions(conn, db, o)),
  createView: (conn: string, db: string, name: string, viewOn: string, pipeline: string, collation: string) =>
    wrap<void>(a.CreateView(conn, db, name, viewOn, pipeline, collation)),
  renameCollection: (conn: string, db: string, from: string, toDb: string, to: string, dropTarget: boolean, confirm: string) =>
    wrap<void>(a.RenameCollection(conn, db, from, toDb, to, dropTarget, confirm)),
  startRenameDatabase: (conn: string, db: string, to: string, confirm: string) => wrap<Job>(a.StartRenameDatabase(conn, db, to, confirm)),
  clearCollection: (conn: string, db: string, coll: string, confirm: string) => wrap<number>(a.ClearCollection(conn, db, coll, confirm)),
  startCopy: (r: { srcConn: string; dstConn: string; options: CopyOptions }, confirm: string) => wrap<Job>(a.StartCopy(r, confirm)),
  getValidation: (conn: string, db: string, coll: string) => wrap<Validation>(a.GetValidation(conn, db, coll)),
  checkValidator: (conn: string, db: string, coll: string, validator: string) => wrap<ValidationReport>(a.CheckValidator(conn, db, coll, validator)),
  setValidation: (conn: string, db: string, coll: string, v: Validation) => wrap<void>(a.SetValidation(conn, db, coll, v)),
  listIndexDetails: (conn: string, db: string, coll: string) => wrap<IndexDetail[]>(a.ListIndexDetails(conn, db, coll)),
  createIndex: (opId: string, conn: string, db: string, coll: string, o: IndexOptions) => wrap<string>(a.CreateIndex(opId, conn, db, coll, o)),
  dropIndex: (conn: string, db: string, coll: string, name: string) => wrap<void>(a.DropIndex(conn, db, coll, name)),
  modifyIndex: (conn: string, db: string, coll: string, name: string, ch: { hidden?: boolean | null; ttl?: number | null }) =>
    wrap<void>(a.ModifyIndex(conn, db, coll, name, ch)),
  collectionDetails: (conn: string, db: string, coll: string) => wrap<CollectionDetails>(a.CollectionDetails(conn, db, coll)),
  databaseDetails: (conn: string, db: string) => wrap<DatabaseDetails>(a.DatabaseDetails(conn, db)),
  exportTasks: (ids: string[]) => wrap<string>(a.ExportTasks(ids)),
  importTasks: (content: string) => wrap<{ imported: string[]; errors: string[] }>(a.ImportTasks(content)),
  getProfiling: (conn: string, db: string) => wrap<Profiling>(a.GetProfiling(conn, db)),
  setProfiling: (conn: string, db: string, p: Profiling) => wrap<void>(a.SetProfiling(conn, db, p)),
  listProfile: (conn: string, db: string, q: ProfileQuery) => wrap<ProfileEntry[]>(a.ListProfile(conn, db, q)),
  clearProfile: (conn: string, db: string) => wrap<void>(a.ClearProfile(conn, db)),
  analyzeIndexes: (opId: string, conn: string, db: string, o: { coll: string; minMillis: number; limit: number }) =>
    wrap<IndexAdvice>(a.AnalyzeIndexes(opId, conn, db, o)),
  startSQLExport: (r: { connId: string; db: string; coll: string; path: string; options: SQLExportOptions }) => wrap<Job>(a.StartSQLExport(r)),
  analyzeSchema: (opId: string, req: { connId: string; db: string; coll: string; sampleSize: number; filter: string }) =>
    wrap<SchemaReport>(a.AnalyzeSchema(opId, req)),
  translateSQL: (sql: string) => wrap<SQLTranslation>(a.TranslateSQL(sql)),
  runSQL: (opId: string, req: { connId: string; db: string; sql: string; page: number; pageSize: number }) =>
    wrap<SQLResult>(a.RunSQL(opId, req)),
  codeLanguages: () => wrap<CodeLanguage[]>(a.CodeLanguages()),
  generateCode: (req: CodeRequest) => wrap<string>(a.GenerateCode(req)),
  explain: (opId: string, req: ExplainRequest) => wrap<ExplainPlan>(a.Explain(opId, req)),
  listSaved: (kind = '') => wrap<SavedItem[]>(a.ListSaved(kind)),
  saveItem: (item: Partial<SavedItem> & Pick<SavedItem, 'kind' | 'name'>) =>
    wrap<SavedItem>(a.SaveItem({ id: '', description: '', db: '', coll: '', ...item })),
  deleteItem: (id: string) => wrap<void>(a.DeleteItem(id)),
  listHistory: (kind = '', search = '', limit = 200) => wrap<HistoryEntry[]>(a.ListHistory(kind, search, limit)),
  clearHistory: () => wrap<void>(a.ClearHistory()),

  // P2
  startCompare: (o: CompareOptions) => wrap<Job>(a.StartCompare(o)),
  startCompareDatabases: (o: DBCompareOptions) => wrap<Job>(a.StartCompareDatabases(o)),
  getCompareResult: (jobId: string) => wrap<CompareResult>(a.GetCompareResult(jobId)),
  getDatabaseCompareResult: (jobId: string) => wrap<DBCompareResult>(a.GetDatabaseCompareResult(jobId)),
  previewSync: (compareJobId: string, o: SyncOptions) => wrap<SyncPreview>(a.PreviewSync(compareJobId, o)),
  startSync: (planId: string, confirm: string) => wrap<Job>(a.StartSync(planId, confirm)),
  getSyncOutcome: (jobId: string) => wrap<SyncOutcome>(a.GetSyncOutcome(jobId)),
  listBackups: () => wrap<BackupInfo[]>(a.ListBackups()),
  startRestore: (path: string, connId: string, confirm: string) => wrap<Job>(a.StartRestore(path, connId, confirm)),
  previewReschema: (spec: TransformSpec, ops: ReschemaOp[]) => wrap<TransformPreview>(a.PreviewReschema({ spec, ops, rules: [], ruleSetId: '' })),
  previewMask: (spec: TransformSpec, rules: MaskRule[], ruleSetId = '') =>
    wrap<TransformPreview>(a.PreviewMask({ spec, ops: [], rules, ruleSetId })),
  startTransform: (planId: string, confirm: string) => wrap<Job>(a.StartTransform(planId, confirm)),
  getTransformReport: (jobId: string) => wrap<TransformReport>(a.GetTransformReport(jobId)),
  reschemaTypes: () => wrap<string[]>(a.ReschemaTypes()),
  maskMethods: () => wrap<MaskMethod[]>(a.MaskMethods()),
  listMaskRuleSets: () => wrap<MaskRuleSet[]>(a.ListMaskRuleSets()),
  saveMaskRuleSet: (rs: Partial<MaskRuleSet> & { name: string; rules: MaskRule[] }) =>
    wrap<MaskRuleSet>(a.SaveMaskRuleSet({ id: '', updatedAt: new Date().toISOString(), ...rs })),
  deleteMaskRuleSet: (id: string) => wrap<void>(a.DeleteMaskRuleSet(id)),
  introspectSQL: (src: SQLSource) => wrap<SQLTable[]>(a.IntrospectSQL(src)),
  migrationTypes: () => wrap<string[]>(a.MigrationTypes()),
  startMigration: (req: MigrationRequest) => wrap<Job>(a.StartMigration(req)),
  listMigrations: () => wrap<MigrationRun[]>(a.ListMigrations()),
  rollbackMigration: (runId: string, confirm: string) => wrap<MigrationRun>(a.RollbackMigration(runId, confirm)),
  listTaskTypes: () => wrap<TaskType[]>(a.ListTaskTypes()),
  listTasks: () => wrap<Task[]>(a.ListTasks()),
  saveTask: (t: Partial<Task> & Pick<Task, 'name' | 'type' | 'schedule'>) =>
    wrap<Task>(a.SaveTask({ id: '', enabled: false, retry: { maxAttempts: 1, backoffSeconds: 60 }, misfire: 'skip', params: {}, ...t })),
  deleteTask: (id: string) => wrap<void>(a.DeleteTask(id)),
  approveTask: (id: string) => wrap<Task>(a.ApproveTask(id)),
  revokeTask: (id: string) => wrap<Task>(a.RevokeTask(id)),
  setTaskEnabled: (id: string, enabled: boolean) => wrap<Task>(a.SetTaskEnabled(id, enabled)),
  runTaskNow: (id: string) => wrap<TaskRun>(a.RunTaskNow(id)),
  listTaskRuns: (taskId = '', limit = 100) => wrap<TaskRun[]>(a.ListTaskRuns(taskId, limit)),
  getAIStatus: () => wrap<AIStatus>(a.GetAIStatus()),
  detectAIProviders: () => wrap<AIProviders>(a.DetectAIProviders()),
  listLocalModels: (url: string, api: string) => wrap<string[]>(a.ListLocalModels(url, api)),
  setLocalAIKey: (key: string) => wrap<void>(a.SetLocalAIKey(key)),
  clearLocalAIKey: () => wrap<void>(a.ClearLocalAIKey()),
  testAI: (opId: string) => wrap<string>(a.TestAI(opId)),
  setAIKey: (key: string) => wrap<void>(a.SetAIKey(key)),
  clearAIKey: () => wrap<void>(a.ClearAIKey()),
  previewAIContext: (req: AIRequest) => wrap<AIContext>(a.PreviewAIContext(req)),
  generateAI: (opId: string, req: AIRequest) => wrap<AIProposal>(a.GenerateAI(opId, req)),
  previewAIWrite: (id: string) => wrap<AIWritePreview>(a.PreviewAIWrite(id)),
  executeAIWrite: (id: string, confirm: string) => wrap<AIWriteResult>(a.ExecuteAIWrite(id, confirm)),
  getMCPInfo: () => wrap<MCPInfo>(a.GetMCPInfo()),
  saveMCPPolicy: (p: MCPPolicy) => wrap<MCPPolicy>(a.SaveMCPPolicy(p)),
}

/** Subscribes to backend job updates; returns an unsubscribe function. */
/** Subscribes to connection log steps ("conn:step"). */
export function onConnectionStep(cb: (e: { opId: string; step: ConnectionStep }) => void): () => void {
  try {
    return EventsOn('conn:step', cb)
  } catch {
    return () => undefined
  }
}

/** Subscribes to native menu clicks ("menu:<name>"). */
export function onMenu(name: string, cb: () => void): () => void {
  try {
    return EventsOn(`menu:${name}`, cb)
  } catch {
    return () => undefined // not running inside Wails (tests, browser preview)
  }
}

export function onJobUpdate(cb: (j: Job) => void): () => void {
  try {
    return EventsOn('jobs:update', cb)
  } catch {
    return () => undefined // not running inside Wails (tests, plain browser)
  }
}

export type Backend = typeof backend

let seq = 0
/** Unique operation ID used to cancel long-running calls. */
export function newOpId(prefix: string): string {
  seq += 1
  return `${prefix}-${Date.now().toString(36)}-${seq}`
}
