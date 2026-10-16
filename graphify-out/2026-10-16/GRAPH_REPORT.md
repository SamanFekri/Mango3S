# Graph Report - Mango3S  (2026-10-16)

## Corpus Check
- 199 files · ~214,180 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 3260 nodes · 7103 edges · 263 communities (90 shown, 173 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 651 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `c2d3bae0`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- transfer.go
- Invalid
- Collection
- parser
- Parser
- WriteJSON
- .evalColl
- codegen.go
- App
- Source
- DocViews.tsx
- Manager
- Analyze
- Mango3S (MongoDB desktop IDE)
- types.ts
- AppContext.tsx
- schema.go
- SchemaView.tsx
- compilerOptions
- CollectionView.tsx
- ConnectionManager.tsx
- logging.go
- App.tsx
- .convertValues
- shellpaging.test.tsx
- Manager
- .cancel
- dependencies
- devDependencies
- runtime/package.json
- App
- newFixture
- backup.go
- models.ts
- wails.json
- .Apply
- compilerOptions
- runtime.d.ts
- Scheduler
- scripts
- 01-sample-data.js
- frontend/package.json
- .Replace
- shortcuts.ts
- CollectionStats
- dial_test.go
- App
- Item
- compare.go
- ai.go
- p2.go
- migrate.go
- Session
- App.test.tsx
- Compact
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
- Store
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
- local.go
- .register
- mql/format.go
- TabErrorBoundary
- providers_test.go
- New
- .Find
- csv.go
- denied
- App
- migrate_test.go
- Mango3S P2 reference
- Store
- TasksView.tsx
- Is
- Service
- wait
- NewDocument
- Writer
- Info
- newFixture
- NewJSONWriter
- server.go
- CodeRequest
- DBResult
- DocDiff
- .backupDir
- codegen_test.go
- call
- Outcome
- tools.go
- mcpserver_test.go
- docactions.test.tsx
- Task
- TestShellResultsArePaged
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
- profile.go
- DatabaseInfo
- ParseCron
- Manager
- Preview
- DBOptions
- WaitGroup
- Document
- .previewTransform
- Logger
- profiles_test.go
- FieldResult
- Mango3S UI reference: Welcome tab, connections, editing and shortcuts
- Field
- ParsedURI
- Plan
- Context
- Job
- Profile
- ProfileInput
- RWMutex
- TransformPreview
- ServerConfig
- CLIInfo
- QuerySpec
- A
- AdvancedConfig
- FieldUpdate
- FieldValue
- HostPort
- ProxyConfig
- SecretInput
- ShellConfig
- SSHConfig
- TLSConfig
- ToolsConfig
- Context
- env
- CancelFunc
- Context
- LevelVar
- Profile
- Report
- Request
- Result
- Service
- D
- Store
- A
- Context
- Context
- Context
- Manager
- Collection
- Context
- Context
- Buffer
- Document
- Request
- LocalServer
- DBResult
- Stage
- SessionInfo
- Entry
- SQLResult
- Info
- Context
- Proposal
- Rand
- Request
- Source
- Store
- Plan
- Report
- Request
- Result
- Context
- Policy
- Proposal
- RawMessage
- Report
- Request
- Result
- Rule
- Service
- Source
- Spec
- Store
- Table
- Profile
- Item
- Language
- MethodInfo
- Op
- Options
- Outcome
- Output
- p2
- Page
- Preview
- Table
- Result
- RuleSet
- MCPInfo
- Store
- BulkPreview
- BulkResult
- BulkWriteRequest
- Duration
- ExportOptions
- Job
- Provider
- Session
- SessionInfo
- Mutex
- Document
- WritePreview
- WriteResult

## God Nodes (most connected - your core abstractions)
1. `App` - 58 edges
2. `App` - 50 edges
3. `useApp()` - 43 edges
4. `Invalid()` - 39 edges
5. `Compact()` - 38 edges
6. `Scheduler` - 37 edges
7. `toAppError()` - 34 edges
8. `Is()` - 32 edges
9. `Collection()` - 32 edges
10. `Manager` - 32 edges

## Surprising Connections (you probably didn't know these)
- `newApp()` --calls--> `NewStore()`  [INFERRED]
  main.go → internal/config/config.go
- `runDesktop()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go
- `runMCP()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go
- `newApp()` --calls--> `NewManager()`  [INFERRED]
  main.go → internal/mongox/manager.go
- `TreeView()` --indirect_call--> `key()`  [INFERRED]
  frontend/src/components/DocViews.tsx → frontend/src/App.test.tsx

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Editors sharing the mql value language** — readme_query_editor, readme_aggregation_editor, readme_intellishell, readme_visual_query_builder, readme_mql_shared_value_language [EXTRACTED 1.00]
- **P1 features reusing P0 query/aggregate services** — readme_sql_subset, readme_code_generation, readme_explain_plans, readme_import_export_wizard, readme_p1_reuses_p0 [EXTRACTED 1.00]
- **Docker compose dev/test stack** — docker_compose_mongo, docker_compose_mongo_auth, docker_compose_integration_tests, docker_compose_app, docker_compose_mongo_init [EXTRACTED 1.00]

## Communities (263 total, 173 thin omitted)

### Community 0 - "transfer.go"
Cohesion: 0.14
Nodes (30): ColumnSpec, ParseDelimiter(), DetectFormat(), exportQuery(), fieldsProjection(), firstLine(), Context, D (+22 more)

### Community 1 - "Invalid"
Cohesion: 0.12
Nodes (22): Code, Error, CollectionInfo, CollectionStats, DatabaseInfo, IndexInfo, Service, Check() (+14 more)

### Community 2 - "Collection"
Cohesion: 0.27
Nodes (19): allDocs(), byID(), Context, D, Job, Manager, T, representative() (+11 more)

### Community 3 - "parser"
Cohesion: 0.09
Nodes (41): errAt(), lex(), isLiteralFunc(), parse(), compact(), T, TestErrorPosition(), TestLikeToRegex() (+33 more)

### Community 5 - "Parser"
Cohesion: 0.07
Nodes (45): Equal(), TypeName(), T, mustDec(), mustHex(), mustOID(), TestErrorPosition(), TestFormatShape() (+37 more)

### Community 6 - "WriteJSON"
Cohesion: 0.06
Nodes (34): WriteJSON(), Mutex, Stage, Time, NewHistory(), NewStore(), sameRun(), T (+26 more)

### Community 7 - ".evalColl"
Cohesion: 0.06
Nodes (65): A, BulkPreview, BulkResult, BulkWriteRequest, Collection, Cursor, D, Database (+57 more)

### Community 8 - "codegen.go"
Cohesion: 0.18
Nodes (35): Language, renderer, Spec, b64(), block(), csRenderer(), escapeRegex(), floatLit() (+27 more)

### Community 9 - "App"
Cohesion: 0.08
Nodes (18): CodeRequest, SQLRequest, SQLResult, SQLTranslation, ImportOptions, codeSpec(), errorMessage(), findRequest() (+10 more)

### Community 11 - "Source"
Cohesion: 0.16
Nodes (21): Request, Service, Stage, Build(), FromText(), Context, Page, indentLines() (+13 more)

### Community 12 - "DocViews.tsx"
Cohesion: 0.05
Nodes (71): Harness(), CellEditing, CellEditor(), CellRef, EditState, FieldApi, ResultsNotice, sameRef() (+63 more)

### Community 13 - "Manager"
Cohesion: 0.12
Nodes (14): CancelFunc, Context, Duration, Logger, Mutex, Time, WaitGroup, NewManager() (+6 more)

### Community 14 - "Analyze"
Cohesion: 0.13
Nodes (33): Detail, Diagnostic, m, Plan, Request, Service, Stage, Summary (+25 more)

### Community 15 - "Mango3S (MongoDB desktop IDE)"
Cohesion: 0.06
Nodes (38): app service (GUI, Linux X11), integration-tests service (test profile), mongo service (MongoDB 8.0, seeded), mongo-auth service (auth profile), docker/mongo-init seed scripts, /src/main.tsx entry script, index.html #root mount, Aggregation Editor (+30 more)

### Community 16 - "types.ts"
Cohesion: 0.04
Nodes (93): Checks(), ConfirmInput(), EndpointPicker(), JobStatus(), Stat(), useJob(), useJobDone(), AI_FIELDS (+85 more)

### Community 17 - "AppContext.tsx"
Cohesion: 0.06
Nodes (49): FolderNode, onJobUpdate(), KIND_LABELS, needsCollection(), openAction(), timeAgo(), ctx, labels() (+41 more)

### Community 18 - "schema.go"
Cohesion: 0.13
Nodes (22): Field, D, DateTime, lessPath(), NewAnalyzer(), ptr(), field(), Report (+14 more)

### Community 19 - "SchemaView.tsx"
Cohesion: 0.16
Nodes (19): Metric(), stageClass(), StageNode(), elapsed(), jobProgress(), JobRow(), JobsIndicator(), formatBytes() (+11 more)

### Community 20 - "compilerOptions"
Cohesion: 0.07
Nodes (28): compilerOptions, allowJs, allowSyntheticDefaultImports, esModuleInterop, forceConsistentCasingInFileNames, isolatedModules, jsx, lib (+20 more)

### Community 21 - "CollectionView.tsx"
Cohesion: 0.10
Nodes (54): AIDialog(), BulkWriteDialog(), CodeDialog(), CodeEditor(), CodeEditorProps, HELPERS, OPERATORS, selectionOrAll() (+46 more)

### Community 22 - "ConnectionManager.tsx"
Cohesion: 0.09
Nodes (31): blankProfile(), CONNECTION_TABS, ConnectionManager(), emptyAdvanced(), ERROR_TABS, folderTree(), fromProfile(), TabId (+23 more)

### Community 23 - "logging.go"
Cohesion: 0.14
Nodes (17): Attr, Handler, Context, LevelVar, Logger, isSecretKey(), NewRedactingHandler(), ParseLevel() (+9 more)

### Community 24 - "App.tsx"
Cohesion: 0.09
Nodes (29): ConfirmDialog(), PromptDialog(), Toasts(), base(), CollectionIcon(), CollectionKind, DatabaseIcon(), FolderIcon() (+21 more)

### Community 25 - ".convertValues"
Cohesion: 0.04
Nodes (15): AIProviders, Entry, ImportOptions, MCPInfo, Options, Output, Page, Proposal (+7 more)

### Community 26 - "shellpaging.test.tsx"
Cohesion: 0.25
Nodes (8): ShellOutput, app, backend, doc(), page(), Entry, ShellView(), statementAt()

### Community 28 - ".cancel"
Cohesion: 0.16
Nodes (11): AppInfo, Lifecycle, opRegistry, runningOp, CancelFunc, callWithTimeout(), Duration, Mutex (+3 more)

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
Cohesion: 0.08
Nodes (15): Bool, CollectionInfo, CollectionStats, DatabaseInfo, History, call(), App, Page (+7 more)

### Community 33 - "newFixture"
Cohesion: 0.33
Nodes (15): fixture, App, decodeErr(), Buffer, T, newFixture(), TestCancelOperation(), TestConnectFailureIsReadable() (+7 more)

### Community 34 - "backup.go"
Cohesion: 0.18
Nodes (15): Header, Info, ManyResult, RestoreStats, Create(), Collection, Context, Time (+7 more)

### Community 35 - "models.ts"
Cohesion: 0.13
Nodes (27): aggregate, ai, api, backup, codegen, compare, config, documents (+19 more)

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
Cohesion: 0.13
Nodes (19): NewID(), CancelFunc, Context, Duration, Logger, Mutex, RawMessage, Time (+11 more)

### Community 41 - "scripts"
Cohesion: 0.29
Nodes (7): scripts, build, dev, preview, test, test:watch, typecheck

### Community 42 - "01-sample-data.js"
Cohesion: 0.33
Nodes (5): customers, orders, products, statuses, store

### Community 43 - "frontend/package.json"
Cohesion: 0.40
Nodes (4): name, private, type, version

### Community 44 - ".Replace"
Cohesion: 0.24
Nodes (13): D, Document, Service, Source, IDFilter(), IDsFilter(), New(), ParseID() (+5 more)

### Community 45 - "shortcuts.ts"
Cohesion: 0.06
Nodes (61): Harness(), app, backend, rerender, Keymap, KeymapSettings(), parseKeymapFile(), TEXT_EDITING (+53 more)

### Community 47 - "dial_test.go"
Cohesion: 0.06
Nodes (60): Addr, ClientConfig, HostKeyCallback, Int32, buildDialer(), classifySSH(), containsAny(), Client (+52 more)

### Community 48 - "App"
Cohesion: 0.08
Nodes (6): Duration, App, SetAIProvider(), Provider, Task, TaskType

### Community 50 - "compare.go"
Cohesion: 0.05
Nodes (73): CollectionRow, DBOptions, DBResult, DocDiff, Endpoint, Entry, FieldChange, fingerprint (+65 more)

### Community 51 - "ai.go"
Cohesion: 0.06
Nodes (42): answer, Check, Claude, Context, FieldSummary, Proposal, Provider, Request (+34 more)

### Community 52 - "p2.go"
Cohesion: 0.13
Nodes (19): AIProviders, AIStatus, exportTask, maskCopyTask, p2, syncTask, xformPlan, CLIInfo (+11 more)

### Community 53 - "migrate.go"
Cohesion: 0.23
Nodes (24): contains(), copyTable(), embed(), Execute(), fieldFor(), Collection, Context, D (+16 more)

### Community 54 - "Session"
Cohesion: 0.05
Nodes (59): Check, ClientOptions, describe(), Client, Logger, Profile, RWMutex, Time (+51 more)

### Community 55 - "App.test.tsx"
Cohesion: 0.29
Nodes (5): backend, jobListeners, key(), profile, session

### Community 56 - "Compact"
Cohesion: 0.19
Nodes (24): Compact(), mqlCompact(), NewCSVReader(), NewJSONReader(), dataset(), D, T, readAll() (+16 more)

### Community 57 - "dialect.go"
Cohesion: 0.22
Nodes (21): DB, Redact(), beginRead(), dialectFor(), Context, Rows, Tx, Introspect() (+13 more)

### Community 69 - "Store"
Cohesion: 0.11
Nodes (24): Profile, profilesFile, Settings, Store, DefaultSettings(), AdvancedConfig, ProxyConfig, ServerConfig (+16 more)

### Community 79 - "EventsOn"
Cohesion: 0.67
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 83 - "local.go"
Cohesion: 0.15
Nodes (15): httpStatus, Local, LocalServer, Error, T, TestLiveProviders(), asStatus(), checkURL() (+7 more)

### Community 84 - ".register"
Cohesion: 0.21
Nodes (12): CheckValue(), Context, D, Logger, Policy, Profile, RawMessage, New() (+4 more)

### Community 85 - "mql/format.go"
Cohesion: 0.20
Nodes (13): Buffer, CanonicalJSON(), escapeRegexSlashes(), Format(), FormatDouble(), formatTime(), D, Time (+5 more)

### Community 87 - "providers_test.go"
Cohesion: 0.34
Nodes (12): argAfter(), fakeRunner(), T, hasArg(), TestClaudeCodeErrors(), TestClaudeCodeRunsWithoutToolsOrSettings(), TestCodexUsesReadOnlySandboxAndSchemaFile(), TestDetectClaudeCode() (+4 more)

### Community 88 - "New"
Cohesion: 0.19
Nodes (15): ClaudeCode, CLIInfo, Codex, Deps, cliError(), DetectClaudeCode(), DetectCodex(), emptyDir() (+7 more)

### Community 89 - ".Find"
Cohesion: 0.19
Nodes (16): CollectionRef(), Context, D, Page, New(), Parse(), parseField(), T (+8 more)

### Community 90 - "csv.go"
Cohesion: 0.18
Nodes (12): FormatCell(), D, InferType(), LeafPaths(), LookupPath(), NewCSVWriter(), setPath(), validType() (+4 more)

### Community 91 - "denied"
Cohesion: 0.24
Nodes (8): contains(), denied(), Mutex, NewStore(), toolInfo(), Policy, Store, ToolInfo

### Community 92 - "App"
Cohesion: 0.06
Nodes (29): Capability, ProfileInput, SecretInput, Credentials, FieldValue, agentNote(), agentSupported(), BeforeClose() (+21 more)

### Community 93 - "migrate_test.go"
Cohesion: 0.22
Nodes (14): clip(), ConvertValue(), SuggestType(), SuggestTypeFor(), toInt64(), T, SeedSQLite(), TestConvertValue() (+6 more)

### Community 94 - "Mango3S P2 reference"
Cohesion: 0.13
Nodes (14): Access control (deny by default), AI-assisted queries and pipelines, Compare and sync, Configuration files (in the config directory), Credentials and sensitive data, Data-integrity model, Mango3S P2 reference, MCP server (+6 more)

### Community 95 - "Store"
Cohesion: 0.24
Nodes (9): Duration, entry, Mutex, T, Time, New(), entry, Store (+1 more)

### Community 96 - "TasksView.tsx"
Cohesion: 0.17
Nodes (13): app, backend, base, AIProposal, Task, TaskRun, TaskSchedule, TaskType (+5 more)

### Community 97 - "Is"
Cohesion: 0.06
Nodes (67): AEAD, env, Is(), D, T, Time, mustTime(), rawDoc() (+59 more)

### Community 99 - "Service"
Cohesion: 0.29
Nodes (8): ReadJSON(), Mutex, Source, Time, NewService(), Run, runsFile, Service

### Community 100 - "wait"
Cohesion: 0.45
Nodes (11): Job, Manager, T, newManager(), TestCancelRunningAndQueued(), TestIgnoredCancellationStillCancels(), TestJobFailureAndPanic(), TestJobSucceedsWithProgress() (+3 more)

### Community 101 - "NewDocument"
Cohesion: 0.24
Nodes (9): ComputePaging(), D, NewDocument(), T, TestComputePaging(), TestNewDocument(), Document, Page (+1 more)

### Community 102 - "Writer"
Cohesion: 0.27
Nodes (8): line, Writer, File, canonical(), D, Mutex, RawMessage, WriteManyWithBackup()

### Community 104 - "newFixture"
Cohesion: 0.31
Nodes (13): Duration, Mutex, T, Time, newFixture(), TestApprovalGatesExecution(), TestCronParseAndNext(), TestNoOverlap() (+5 more)

### Community 105 - "NewJSONWriter"
Cohesion: 0.23
Nodes (7): Decoder, D, NewJSONWriter(), truncate(), JSONReader, JSONWriter, Writer

### Community 106 - "server.go"
Cohesion: 0.24
Nodes (13): CallToolResult, In, add(), errorResult(), textResult(), aggIn, collIn, connIn (+5 more)

### Community 110 - ".backupDir"
Cohesion: 0.18
Nodes (5): Func, expandPath(), Time, NewMCPServer(), Server

### Community 111 - "codegen_test.go"
Cohesion: 0.34
Nodes (14): fakeProvider, aggSpec(), gen(), Spec, T, mustContain(), richSpec(), TestAggregateAllLanguages() (+6 more)

### Community 113 - "call"
Cohesion: 0.22
Nodes (4): call, FieldResult, FieldUpdate, Source

### Community 115 - "tools.go"
Cohesion: 0.27
Nodes (13): commonDirs(), Detect(), detectOne(), exe(), firstLine(), ToolsConfig, isFile(), locate() (+5 more)

### Community 118 - "mcpserver_test.go"
Cohesion: 0.33
Nodes (12): ClientSession, call(), connectClient(), Policy, T, TestAuditLogOmitsArguments(), TestBoundaryEnforcementBeforeConnecting(), TestCheckValueRefusesJavaScriptAndWrites() (+4 more)

### Community 119 - "docactions.test.tsx"
Cohesion: 0.11
Nodes (15): api(), cell(), doc, Extra, focusCell(), fv(), page, result() (+7 more)

### Community 150 - "profile.go"
Cohesion: 0.09
Nodes (25): AdvancedConfig, HostPort, ParsedURI, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig (+17 more)

### Community 152 - "ParseCron"
Cohesion: 0.39
Nodes (4): Time, ParseCron(), parseField(), Cron

### Community 161 - "profiles_test.go"
Cohesion: 0.46
Nodes (7): fullInput(), T, TestDuplicateAndDeleteProfile(), TestParseConnectionStringNeverReturnsThePassword(), TestProfileValidationErrorsReachTheUI(), TestSaveProfileAllTabsAndSecrets(), ProfileInput

### Community 163 - "Mango3S UI reference: Welcome tab, connections, editing and shortcuts"
Cohesion: 0.20
Nodes (9): Collection query editor and paging, Connection Manager, Document and field actions (right-click menu), Keyboard shortcuts, Mango3S UI reference: Welcome tab, connections, editing and shortcuts, Preferences → Keymap, Result views and field editing, Unsupported and limitations (+1 more)

## Knowledge Gaps
- **254 isolated node(s):** `profile`, `session`, `backend`, `jobListeners`, `EditState` (+249 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **173 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `App` connect `App` to `.previewTransform`, `Info`, `.backupDir`, `call`, `Outcome`, `p2.go`, `providers_test.go`, `Preview`, `MCPInfo`?**
  _High betweenness centrality (0.065) - this node is a cross-community bridge._
- **Why does `NewID()` connect `Scheduler` to `Is`, `backup.go`, `Collection`, `Store`, `WriteJSON`, `.evalColl`, `Manager`, `dialect.go`, `Store`?**
  _High betweenness centrality (0.064) - this node is a cross-community bridge._
- **Why does `newP2()` connect `p2.go` to `New`, `App`, `Store`?**
  _High betweenness centrality (0.058) - this node is a cross-community bridge._
- **Are the 36 inferred relationships involving `Invalid()` (e.g. with `.validate()` and `.Validate()`) actually correct?**
  _`Invalid()` has 36 INFERRED edges - model-reasoned connections that need verification._
- **Are the 36 inferred relationships involving `Compact()` (e.g. with `.Insert()` and `.CollectionStats()`) actually correct?**
  _`Compact()` has 36 INFERRED edges - model-reasoned connections that need verification._
- **What connects `profile`, `session`, `backend` to the rest of the system?**
  _254 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `transfer.go` be split into smaller, more focused modules?**
  _Cohesion score 0.1361344537815126 - nodes in this community are weakly interconnected._