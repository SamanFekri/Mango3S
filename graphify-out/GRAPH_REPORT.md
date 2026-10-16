# Graph Report - Mango3S  (2026-10-16)

## Corpus Check
- 155 files · ~150,384 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2541 nodes · 5756 edges · 158 communities (78 shown, 80 thin omitted)
- Extraction: 90% EXTRACTED · 10% INFERRED · 0% AMBIGUOUS · INFERRED: 562 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `83565e36`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- transfer.go
- Collection
- setup
- parser
- Parser
- Store
- .evalColl
- codegen.go
- App
- Build
- lib.test.ts
- Manager
- Analyze
- Mango3S (MongoDB desktop IDE)
- types.ts
- ShellView.tsx
- schema.go
- ResultsView.tsx
- compilerOptions
- CollectionView.tsx
- AppContext.tsx
- logging.go
- App.tsx
- .convertValues
- Session
- Manager
- api.go
- dependencies
- devDependencies
- runtime/package.json
- App
- newFixture
- p2_test.go
- models.ts
- wails.json
- .Apply
- compilerOptions
- runtime.d.ts
- Scheduler
- scripts
- 01-sample-data.js
- frontend/package.json
- App.test.tsx
- TabErrorBoundary
- CollectionStats
- Entry
- App
- Item
- compare.go
- ai.go
- p2.go
- migrate.go
- Request
- SessionInfo
- transfer_test.go
- dialect.go
- AppInfo
- CollectionInfo
- ColumnPreview
- ColumnSpec
- DatabaseInfo
- Detail
- Diagnostic
- Document
- ExportOptions
- IndexInfo
- Language
- ProfileInput
- QueryPayload
- Settings
- SQLRequest
- SQLTranslation
- Summary
- TestResult
- TypeStat
- react-dom
- @testing-library/jest-dom
- EventsOn
- mango3s
- Invalid
- .register
- mql/format.go
- ParseValue
- Apply
- .Replace
- .Find
- csv.go
- Compact
- denied
- migrate_test.go
- Mango3S P2 reference
- Store
- privileges.go
- mcpserver_test.go
- server.go
- Service
- wait
- NewDocument
- NewJSONWriter
- .Generate
- shell/parse.go
- MCPInfo
- TestBuildClientOptions
- CodeRequest
- DBResult
- DocDiff
- Field
- Info
- MCPInfo
- Options
- Outcome
- Plan
- Preview
- Proposal
- Result
- RuleSet
- Table
- Task
- TransformPreview
- TransformRequest
- AIStatus
- Check
- CollectionRow
- Column
- ColumnMapping
- Counts
- DBOptions
- Embed
- EmbedReport
- Endpoint
- FieldChange
- FieldSummary
- ForeignKey
- MethodInfo
- Op
- Policy
- Retry
- Rule
- Schedule
- Source
- Spec
- TableReport
- TaskType
- ToolInfo
- WritePreview
- WriteResult
- DatabaseInfo
- Document
- Page
- RWMutex
- Manager
- Service
- Store
- WaitGroup

## God Nodes (most connected - your core abstractions)
1. `App` - 55 edges
2. `App` - 52 edges
3. `Invalid()` - 39 edges
4. `Scheduler` - 38 edges
5. `call()` - 35 edges
6. `Manager` - 34 edges
7. `useApp()` - 34 edges
8. `Collection()` - 33 edges
9. `Compact()` - 32 edges
10. `toAppError()` - 31 edges

## Surprising Connections (you probably didn't know these)
- `runMCP()` --calls--> `NewMCPServer()`  [INFERRED]
  main.go → internal/api/p2.go
- `newApp()` --calls--> `NewManager()`  [INFERRED]
  main.go → internal/jobs/jobs.go
- `newApp()` --calls--> `Setup()`  [INFERRED]
  main.go → internal/logging/logging.go
- `runDesktop()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go
- `runMCP()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Editors sharing the mql value language** — readme_query_editor, readme_aggregation_editor, readme_intellishell, readme_visual_query_builder, readme_mql_shared_value_language [EXTRACTED 1.00]
- **P1 features reusing P0 query/aggregate services** — readme_sql_subset, readme_code_generation, readme_explain_plans, readme_import_export_wizard, readme_p1_reuses_p0 [EXTRACTED 1.00]
- **Docker compose dev/test stack** — docker_compose_mongo, docker_compose_mongo_auth, docker_compose_integration_tests, docker_compose_app, docker_compose_mongo_init [EXTRACTED 1.00]

## Communities (158 total, 80 thin omitted)

### Community 0 - "transfer.go"
Cohesion: 0.13
Nodes (31): ColumnSpec, ParseDelimiter(), DetectFormat(), exportQuery(), fieldsProjection(), firstLine(), Context, D (+23 more)

### Community 1 - "Collection"
Cohesion: 0.16
Nodes (15): CollectionInfo, CollectionStats, DatabaseInfo, IndexInfo, Service, Check(), CollectPaths(), Context (+7 more)

### Community 2 - "setup"
Cohesion: 0.14
Nodes (35): env, contains(), containsDB(), Context, DatabaseInfo, Manager, T, nField() (+27 more)

### Community 3 - "parser"
Cohesion: 0.08
Nodes (42): mqlCompact(), errAt(), lex(), isLiteralFunc(), parse(), compact(), T, TestErrorPosition() (+34 more)

### Community 5 - "Parser"
Cohesion: 0.22
Nodes (3): isIdentPart(), isIdentStart(), Parser

### Community 6 - "Store"
Cohesion: 0.06
Nodes (40): Profile, profilesFile, Settings, Store, DefaultSettings(), Mutex, Time, NewStore() (+32 more)

### Community 7 - ".evalColl"
Cohesion: 0.15
Nodes (33): Cursor, Database, describe(), T, TestHumanBytes(), TestParseErrors(), TestParseStatements(), argArray() (+25 more)

### Community 8 - "codegen.go"
Cohesion: 0.18
Nodes (35): Language, renderer, Spec, b64(), block(), csRenderer(), escapeRegex(), floatLit() (+27 more)

### Community 9 - "App"
Cohesion: 0.07
Nodes (25): CodeRequest, exportTask, SQLRequest, SQLResult, SQLTranslation, Entry, ExportOptions, ImportOptions (+17 more)

### Community 11 - "Build"
Cohesion: 0.19
Nodes (18): Request, Service, Stage, Build(), FromText(), Context, Page, indentLines() (+10 more)

### Community 12 - "lib.test.ts"
Cohesion: 0.13
Nodes (29): Harness(), page, BuiltQuery, QueryBuilder(), newStage(), PipelineAction, pipelineReducer(), STAGE_TEMPLATES (+21 more)

### Community 13 - "Manager"
Cohesion: 0.05
Nodes (43): Counts, DocDiff, CancelFunc, Context, Duration, Mutex, Time, WaitGroup (+35 more)

### Community 14 - "Analyze"
Cohesion: 0.14
Nodes (32): Detail, Diagnostic, m, Plan, Request, Service, Stage, Summary (+24 more)

### Community 15 - "Mango3S (MongoDB desktop IDE)"
Cohesion: 0.06
Nodes (38): app service (GUI, Linux X11), integration-tests service (test profile), mongo service (MongoDB 8.0, seeded), mongo-auth service (auth profile), docker/mongo-init seed scripts, /src/main.tsx entry script, index.html #root mount, Aggregation Editor (+30 more)

### Community 16 - "types.ts"
Cohesion: 0.05
Nodes (73): Checks(), ConfirmInput(), EndpointPicker(), JobStatus(), Stat(), useJob(), useJobDone(), a (+65 more)

### Community 17 - "ShellView.tsx"
Cohesion: 0.10
Nodes (29): selectionOrAll(), KIND_LABELS, needsCollection(), openAction(), timeAgo(), ctx, labels(), COLLECTION_METHODS (+21 more)

### Community 18 - "schema.go"
Cohesion: 0.13
Nodes (22): Field, D, DateTime, lessPath(), NewAnalyzer(), ptr(), field(), Report (+14 more)

### Community 19 - "ResultsView.tsx"
Cohesion: 0.09
Nodes (37): Metric(), elapsed(), jobProgress(), JobRow(), JobsIndicator(), DocCard, PAGE_SIZES, ResultsView() (+29 more)

### Community 20 - "compilerOptions"
Cohesion: 0.07
Nodes (27): compilerOptions, allowJs, allowSyntheticDefaultImports, esModuleInterop, forceConsistentCasingInFileNames, isolatedModules, jsx, lib (+19 more)

### Community 21 - "CollectionView.tsx"
Cohesion: 0.10
Nodes (47): AIDialog(), CodeDialog(), CodeEditor(), CodeEditorProps, HELPERS, OPERATORS, theme, ErrorBanner() (+39 more)

### Community 22 - "AppContext.tsx"
Cohesion: 0.13
Nodes (24): onJobUpdate(), EditorStage, sameTarget(), tabId(), TabInit, TabKind, TabsAction, tabsReducer() (+16 more)

### Community 23 - "logging.go"
Cohesion: 0.14
Nodes (17): Attr, Handler, Context, LevelVar, Logger, isSecretKey(), NewRedactingHandler(), ParseLevel() (+9 more)

### Community 24 - "App.tsx"
Cohesion: 0.09
Nodes (22): ConfirmDialog(), PromptDialog(), Toasts(), blank(), ConnectionManager(), fromProfile(), MECHANISMS, AISettings() (+14 more)

### Community 25 - ".convertValues"
Cohesion: 0.05
Nodes (12): Context, ImportOptions, Job, Output, Page, Profile, Report, Run (+4 more)

### Community 26 - "Session"
Cohesion: 0.20
Nodes (15): Client, ClientOptions, BuildClientOptions(), describe(), Context, Logger, Profile, RWMutex (+7 more)

### Community 28 - "api.go"
Cohesion: 0.12
Nodes (16): AppInfo, Deps, Lifecycle, opRegistry, runningOp, CancelFunc, LevelVar, Logger (+8 more)

### Community 29 - "dependencies"
Cohesion: 0.11
Nodes (19): codemirror, @codemirror/autocomplete, @codemirror/commands, @codemirror/lang-javascript, @codemirror/lang-sql, @codemirror/language, @codemirror/state, @codemirror/view (+11 more)

### Community 30 - "devDependencies"
Cohesion: 0.11
Nodes (19): devDependencies, jsdom, @testing-library/dom, @testing-library/react, @types/react, @types/react-dom, typescript, vite (+11 more)

### Community 31 - "runtime/package.json"
Cohesion: 0.11
Nodes (18): author, bugs, url, description, homepage, keywords, license, main (+10 more)

### Community 32 - "App"
Cohesion: 0.07
Nodes (21): ProfileInput, CollectionInfo, CollectionStats, DatabaseInfo, History, call(), App, Context (+13 more)

### Community 33 - "newFixture"
Cohesion: 0.08
Nodes (32): AEAD, fixture, App, Error, decodeErr(), Buffer, T, newFixture() (+24 more)

### Community 34 - "p2_test.go"
Cohesion: 0.07
Nodes (62): Header, Info, line, ManyResult, RestoreStats, Writer, env, File (+54 more)

### Community 35 - "models.ts"
Cohesion: 0.15
Nodes (24): aggregate, ai, api, backup, codegen, compare, config, explain (+16 more)

### Community 36 - "wails.json"
Cohesion: 0.12
Nodes (15): author, email, name, frontend:build, frontend:dev:serverUrl, frontend:dev:watcher, frontend:install, info (+7 more)

### Community 37 - ".Apply"
Cohesion: 0.06
Nodes (63): Clone(), CloneDoc(), Delete(), Exists(), Get(), D, index(), Parent() (+55 more)

### Community 38 - "compilerOptions"
Cohesion: 0.22
Nodes (8): compilerOptions, allowSyntheticDefaultImports, composite, module, moduleResolution, skipLibCheck, include, vite.config.ts

### Community 39 - "runtime.d.ts"
Cohesion: 0.25
Nodes (7): EnvironmentInfo, NotificationAction, NotificationCategory, NotificationOptions, Position, Screen, Size

### Community 40 - "Scheduler"
Cohesion: 0.08
Nodes (37): Int32, NewID(), Time, ParseCron(), parseField(), CancelFunc, Context, Duration (+29 more)

### Community 41 - "scripts"
Cohesion: 0.29
Nodes (7): scripts, build, dev, preview, test, test:watch, typecheck

### Community 42 - "01-sample-data.js"
Cohesion: 0.33
Nodes (5): customers, orders, products, statuses, store

### Community 43 - "frontend/package.json"
Cohesion: 0.40
Nodes (4): name, private, type, version

### Community 44 - "App.test.tsx"
Cohesion: 0.40
Nodes (4): backend, jobListeners, profile, session

### Community 48 - "App"
Cohesion: 0.05
Nodes (25): Provider, DBOptions, DBResult, Info, New(), expandPath(), App, Context (+17 more)

### Community 50 - "compare.go"
Cohesion: 0.10
Nodes (43): CollectionRow, DBOptions, DBResult, DocDiff, Endpoint, Entry, FieldChange, fingerprint (+35 more)

### Community 51 - "ai.go"
Cohesion: 0.09
Nodes (38): answer, Check, Context, FieldSummary, Proposal, Request, Service, WritePreview (+30 more)

### Community 52 - "p2.go"
Cohesion: 0.11
Nodes (19): AIStatus, maskCopyTask, p2, syncTask, TransformPreview, TransformRequest, xformPlan, Func (+11 more)

### Community 53 - "migrate.go"
Cohesion: 0.23
Nodes (24): contains(), copyTable(), embed(), Execute(), fieldFor(), Collection, Context, D (+16 more)

### Community 56 - "transfer_test.go"
Cohesion: 0.21
Nodes (22): NewCSVReader(), NewJSONReader(), dataset(), D, T, readAll(), TestCSVExportImportRoundTrip(), TestCSVReaderErrors() (+14 more)

### Community 57 - "dialect.go"
Cohesion: 0.22
Nodes (21): DB, Redact(), beginRead(), dialectFor(), Context, Rows, Tx, Introspect() (+13 more)

### Community 79 - "EventsOn"
Cohesion: 0.67
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 83 - "Invalid"
Cohesion: 0.22
Nodes (13): Code, Error, firstLine(), From(), Invalid(), Is(), New(), T (+5 more)

### Community 84 - ".register"
Cohesion: 0.19
Nodes (14): In, CheckValue(), add(), Context, D, Logger, Policy, Profile (+6 more)

### Community 85 - "mql/format.go"
Cohesion: 0.18
Nodes (14): escapeRegexSlashes(), Format(), FormatDouble(), formatTime(), Buffer, D, Time, Lookup() (+6 more)

### Community 86 - "ParseValue"
Cohesion: 0.17
Nodes (18): TypeName(), TestParseDocumentVariants(), construct(), fromExtJSONWrapper(), D, Time, narrowInt(), NewParser() (+10 more)

### Community 87 - "Apply"
Cohesion: 0.26
Nodes (16): Check, Apply(), Collection, Context, D, Source, idOf(), RunPreview() (+8 more)

### Community 88 - ".Replace"
Cohesion: 0.23
Nodes (14): Service, A, Context, D, Document, IDFilter(), IDsFilter(), New() (+6 more)

### Community 89 - ".Find"
Cohesion: 0.20
Nodes (15): CollectionRef(), Context, D, Page, New(), Parse(), parseField(), T (+7 more)

### Community 90 - "csv.go"
Cohesion: 0.19
Nodes (11): D, LeafPaths(), LookupPath(), NewCSVWriter(), setPath(), validType(), ColumnSpec, CSVOptions (+3 more)

### Community 91 - "Compact"
Cohesion: 0.26
Nodes (15): Decimal128, Compact(), Equal(), T, mustDec(), mustHex(), mustOID(), TestErrorPosition() (+7 more)

### Community 92 - "denied"
Cohesion: 0.24
Nodes (8): contains(), denied(), Mutex, NewStore(), toolInfo(), Policy, Store, ToolInfo

### Community 93 - "migrate_test.go"
Cohesion: 0.22
Nodes (14): clip(), ConvertValue(), SuggestType(), SuggestTypeFor(), toInt64(), T, SeedSQLite(), TestConvertValue() (+6 more)

### Community 94 - "Mango3S P2 reference"
Cohesion: 0.13
Nodes (14): Access control (deny by default), AI-assisted queries and pipelines, Compare and sync, Configuration files (in the config directory), Credentials and sensitive data, Data-integrity model, Mango3S P2 reference, MCP server (+6 more)

### Community 95 - "Store"
Cohesion: 0.24
Nodes (9): Duration, entry, Mutex, T, Time, New(), entry, Store (+1 more)

### Community 96 - "privileges.go"
Cohesion: 0.23
Nodes (10): contains(), Context, D, granted(), MissingActions(), T, priv(), TestMissingActions() (+2 more)

### Community 97 - "mcpserver_test.go"
Cohesion: 0.33
Nodes (12): ClientSession, call(), connectClient(), Policy, T, TestAuditLogOmitsArguments(), TestBoundaryEnforcementBeforeConnecting(), TestCheckValueRefusesJavaScriptAndWrites() (+4 more)

### Community 98 - "server.go"
Cohesion: 0.27
Nodes (11): CallToolResult, errorResult(), textResult(), aggIn, collIn, connIn, dbIn, deleteIn (+3 more)

### Community 99 - "Service"
Cohesion: 0.29
Nodes (8): ReadJSON(), Mutex, Source, Time, NewService(), Run, runsFile, Service

### Community 100 - "wait"
Cohesion: 0.45
Nodes (11): Job, Manager, T, newManager(), TestCancelRunningAndQueued(), TestIgnoredCancellationStillCancels(), TestJobFailureAndPanic(), TestJobSucceedsWithProgress() (+3 more)

### Community 101 - "NewDocument"
Cohesion: 0.24
Nodes (9): ComputePaging(), D, NewDocument(), T, TestComputePaging(), TestNewDocument(), Document, Page (+1 more)

### Community 102 - "NewJSONWriter"
Cohesion: 0.24
Nodes (6): Decoder, D, NewJSONWriter(), truncate(), JSONReader, JSONWriter

### Community 103 - ".Generate"
Cohesion: 0.36
Nodes (6): Claude, Usage, classify(), Context, NewClaude(), supportsDefaultFallback()

### Community 104 - "shell/parse.go"
Cohesion: 0.57
Nodes (6): Parse(), parseArgs(), parseChain(), parseStatement(), peekIdent(), statement

### Community 105 - "MCPInfo"
Cohesion: 0.40
Nodes (3): MCPInfo, Policy, ToolInfo

### Community 106 - "TestBuildClientOptions"
Cohesion: 0.60
Nodes (4): T, TestBuildClientOptions(), TestRequireWritable(), TestUnreachableServerIsClassified()

## Knowledge Gaps
- **189 isolated node(s):** `Data-integrity model`, `Compare and sync`, `Reschema and masking`, `Migration from relational databases`, `Policies` (+184 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **80 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Progress` connect `Manager` to `transfer.go`, `Scheduler`, `compare.go`, `migrate.go`, `Apply`?**
  _High betweenness centrality (0.065) - this node is a cross-community bridge._
- **Why does `Invalid()` connect `Invalid` to `transfer.go`, `Collection`, `parser`, `wait`, `Store`, `.evalColl`, `codegen.go`, `App`, `Build`, `.Replace`, `.Find`?**
  _High betweenness centrality (0.061) - this node is a cross-community bridge._
- **Why does `NewID()` connect `Scheduler` to `p2_test.go`, `setup`, `Store`, `Manager`, `dialect.go`, `Store`?**
  _High betweenness centrality (0.045) - this node is a cross-community bridge._
- **Are the 36 inferred relationships involving `Invalid()` (e.g. with `.Delete()` and `.Insert()`) actually correct?**
  _`Invalid()` has 36 INFERRED edges - model-reasoned connections that need verification._
- **Are the 11 inferred relationships involving `call()` (e.g. with `.AnalyzeSchema()` and `.ExecuteAIWrite()`) actually correct?**
  _`call()` has 11 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Data-integrity model`, `Compare and sync`, `Reschema and masking` to the rest of the system?**
  _189 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `transfer.go` be split into smaller, more focused modules?**
  _Cohesion score 0.13333333333333333 - nodes in this community are weakly interconnected._