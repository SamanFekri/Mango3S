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
  CodeLanguage,
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
}

/** Subscribes to backend job updates; returns an unsubscribe function. */
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
