# Graph Report - Mango3S  (2026-10-16)

## Corpus Check
- 199 files · ~213,501 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 3255 nodes · 7098 edges · 270 communities (97 shown, 173 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 650 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `31362eb7`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- transfer.go
- Collection
- Open
- parser
- Parser
- WriteJSON
- .evalColl
- codegen.go
- App
- Build
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
- Is
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
- Compact
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
- config.go
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
- ParseValue
- providers_test.go
- New
- Source
- csv.go
- denied
- App
- migrate_test.go
- Mango3S P2 reference
- Store
- TasksView.tsx
- setup
- shell/parse.go
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
- newApp
- call
- Outcome
- tools.go
- BuildClientOptions
- .encode
- mcpserver_test.go
- ResultsView.tsx
- ProfileInput
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
- Invalid
- DatabaseInfo
- ParseCron
- profiles.go
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
- Store
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
- `newApp()` --calls--> `NewManager()`  [INFERRED]
  main.go → internal/mongox/manager.go
- `appMenu()` --calls--> `MenuEvent()`  [INFERRED]
  main.go → internal/api/profiles.go
- `runDesktop()` --calls--> `BeforeClose()`  [INFERRED]
  main.go → internal/api/profiles.go
- `runDesktop()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Editors sharing the mql value language** — readme_query_editor, readme_aggregation_editor, readme_intellishell, readme_visual_query_builder, readme_mql_shared_value_language [EXTRACTED 1.00]
- **P1 features reusing P0 query/aggregate services** — readme_sql_subset, readme_code_generation, readme_explain_plans, readme_import_export_wizard, readme_p1_reuses_p0 [EXTRACTED 1.00]
- **Docker compose dev/test stack** — docker_compose_mongo, docker_compose_mongo_auth, docker_compose_integration_tests, docker_compose_app, docker_compose_mongo_init [EXTRACTED 1.00]

## Communities (270 total, 173 thin omitted)

### Community 0 - "transfer.go"
Cohesion: 0.14
Nodes (30): ColumnSpec, ParseDelimiter(), DetectFormat(), exportQuery(), fieldsProjection(), firstLine(), Context, D (+22 more)

### Community 1 - "Collection"
Cohesion: 0.10
Nodes (25): Code, Error, CollectionInfo, CollectionStats, DatabaseInfo, IndexInfo, Service, Check() (+17 more)

### Community 2 - "Open"
Cohesion: 0.11
Nodes (16): AEAD, Mutex, loadOrCreateKey(), NewFileStore(), NewMemoryStore(), Open(), probeKeyring(), T (+8 more)

### Community 3 - "parser"
Cohesion: 0.08
Nodes (42): mqlCompact(), errAt(), lex(), isLiteralFunc(), parse(), compact(), T, TestErrorPosition() (+34 more)

### Community 5 - "Parser"
Cohesion: 0.19
Nodes (5): construct(), isIdentPart(), isIdentStart(), regexOptions(), Parser

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

### Community 11 - "Build"
Cohesion: 0.19
Nodes (18): Request, Service, Stage, Build(), FromText(), Context, Page, indentLines() (+10 more)

### Community 12 - "DocViews.tsx"
Cohesion: 0.06
Nodes (66): Harness(), CellEditing, CellEditor(), ResultsNotice, sameRef(), scrollIntoView(), step(), TableView() (+58 more)

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
Nodes (97): Checks(), ConfirmInput(), EndpointPicker(), JobStatus(), Stat(), useJob(), useJobDone(), AI_FIELDS (+89 more)

### Community 17 - "AppContext.tsx"
Cohesion: 0.06
Nodes (52): onJobUpdate(), KIND_LABELS, needsCollection(), openAction(), timeAgo(), ctx, labels(), COLLECTION_METHODS (+44 more)

### Community 18 - "schema.go"
Cohesion: 0.13
Nodes (22): Field, D, DateTime, lessPath(), NewAnalyzer(), ptr(), field(), Report (+14 more)

### Community 19 - "SchemaView.tsx"
Cohesion: 0.18
Nodes (18): Metric(), elapsed(), jobProgress(), JobRow(), JobsIndicator(), formatBytes(), formatCount(), formatDuration() (+10 more)

### Community 20 - "compilerOptions"
Cohesion: 0.07
Nodes (28): compilerOptions, allowJs, allowSyntheticDefaultImports, esModuleInterop, forceConsistentCasingInFileNames, isolatedModules, jsx, lib (+20 more)

### Community 21 - "CollectionView.tsx"
Cohesion: 0.13
Nodes (39): AIDialog(), BulkWriteDialog(), CodeDialog(), CodeEditor(), CodeEditorProps, HELPERS, OPERATORS, selectionOrAll() (+31 more)

### Community 22 - "ConnectionManager.tsx"
Cohesion: 0.08
Nodes (33): blankProfile(), CONNECTION_TABS, ConnectionManager(), emptyAdvanced(), ERROR_TABS, FolderNode, folderTree(), fromProfile() (+25 more)

### Community 23 - "logging.go"
Cohesion: 0.14
Nodes (17): Attr, Handler, Context, LevelVar, Logger, isSecretKey(), NewRedactingHandler(), ParseLevel() (+9 more)

### Community 24 - "App.tsx"
Cohesion: 0.08
Nodes (28): TabErrorBoundary, ConfirmDialog(), PromptDialog(), Toasts(), base(), CollectionIcon(), CollectionKind, DatabaseIcon() (+20 more)

### Community 25 - ".convertValues"
Cohesion: 0.04
Nodes (15): AIProviders, Entry, ImportOptions, MCPInfo, Options, Output, Page, Proposal (+7 more)

### Community 26 - "Is"
Cohesion: 0.18
Nodes (17): Is(), SecretKey(), Profile, T, TestBuildURI(), TestNormalizeDefaults(), TestParseURIRoundTrip(), TestProfileValidationNamesTheTab() (+9 more)

### Community 28 - ".cancel"
Cohesion: 0.10
Nodes (16): AppInfo, Deps, Lifecycle, opRegistry, runningOp, CancelFunc, T, TestLiveProviders() (+8 more)

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
Cohesion: 0.10
Nodes (13): Bool, CollectionInfo, CollectionStats, DatabaseInfo, History, call(), App, Page (+5 more)

### Community 33 - "newFixture"
Cohesion: 0.30
Nodes (16): fixture, App, newOpRegistry(), decodeErr(), Buffer, T, newFixture(), TestCancelOperation() (+8 more)

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
Cohesion: 0.05
Nodes (61): backend, jobListeners, profile, session, app, backend, rerender, Keymap (+53 more)

### Community 47 - "dial_test.go"
Cohesion: 0.07
Nodes (58): Addr, ClientConfig, HostKeyCallback, Int32, buildDialer(), classifySSH(), containsAny(), Client (+50 more)

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
Cohesion: 0.08
Nodes (38): Check, describe(), Client, Logger, Profile, RWMutex, Time, open() (+30 more)

### Community 55 - "Compact"
Cohesion: 0.29
Nodes (14): Compact(), Equal(), T, mustDec(), mustHex(), mustOID(), TestErrorPosition(), TestFormatShape() (+6 more)

### Community 56 - "transfer_test.go"
Cohesion: 0.21
Nodes (22): NewCSVReader(), NewJSONReader(), dataset(), D, T, readAll(), TestCSVExportImportRoundTrip(), TestCSVReaderErrors() (+14 more)

### Community 57 - "dialect.go"
Cohesion: 0.22
Nodes (21): DB, Redact(), beginRead(), dialectFor(), Context, Rows, Tx, Introspect() (+13 more)

### Community 69 - "config.go"
Cohesion: 0.20
Nodes (13): profilesFile, Settings, DefaultSettings(), NewStore(), normalizeKeymap(), SplitCredentials(), T, TestDefaultViewIsTreeUntilChosen() (+5 more)

### Community 79 - "EventsOn"
Cohesion: 0.67
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 83 - "local.go"
Cohesion: 0.20
Nodes (12): httpStatus, Local, LocalServer, Error, asStatus(), checkURL(), DetectLocalServers(), doJSON() (+4 more)

### Community 84 - ".register"
Cohesion: 0.21
Nodes (12): CheckValue(), Context, D, Logger, Policy, Profile, RawMessage, New() (+4 more)

### Community 85 - "mql/format.go"
Cohesion: 0.20
Nodes (13): Buffer, CanonicalJSON(), escapeRegexSlashes(), Format(), formatTime(), D, Time, Lookup() (+5 more)

### Community 86 - "ParseValue"
Cohesion: 0.18
Nodes (16): TypeName(), TestParseDocumentVariants(), fromExtJSONWrapper(), D, Time, narrowInt(), NewParser(), ParseDate() (+8 more)

### Community 87 - "providers_test.go"
Cohesion: 0.34
Nodes (12): argAfter(), fakeRunner(), T, hasArg(), TestClaudeCodeErrors(), TestClaudeCodeRunsWithoutToolsOrSettings(), TestCodexUsesReadOnlySandboxAndSchemaFile(), TestDetectClaudeCode() (+4 more)

### Community 88 - "New"
Cohesion: 0.27
Nodes (11): ClaudeCode, CLIInfo, Codex, cliError(), DetectClaudeCode(), DetectCodex(), emptyDir(), exeName() (+3 more)

### Community 89 - "Source"
Cohesion: 0.20
Nodes (16): CollectionRef(), D, New(), Parse(), parseField(), T, TestParse(), TestParseErrorsNameTheField() (+8 more)

### Community 90 - "csv.go"
Cohesion: 0.17
Nodes (13): FormatDouble(), FormatCell(), D, LeafPaths(), LookupPath(), NewCSVWriter(), setPath(), validType() (+5 more)

### Community 91 - "denied"
Cohesion: 0.24
Nodes (8): contains(), denied(), Mutex, NewStore(), toolInfo(), Policy, Store, ToolInfo

### Community 92 - "App"
Cohesion: 0.10
Nodes (10): SecretInput, Credentials, FieldValue, App, ToolsConfig, ParsedURI, Profile, Session (+2 more)

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

### Community 97 - "setup"
Cohesion: 0.08
Nodes (63): env, D, T, Time, mustTime(), rawDoc(), TestFieldLevelEditing(), TestSystemCollectionsAreListedSeparately() (+55 more)

### Community 98 - "shell/parse.go"
Cohesion: 0.29
Nodes (12): describe(), Parse(), parseArgs(), parseChain(), parseStatement(), peekIdent(), T, TestHumanBytes() (+4 more)

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
Cohesion: 0.24
Nodes (6): Decoder, D, NewJSONWriter(), truncate(), JSONReader, JSONWriter

### Community 106 - "server.go"
Cohesion: 0.24
Nodes (13): CallToolResult, In, add(), errorResult(), textResult(), aggIn, collIn, connIn (+5 more)

### Community 110 - ".backupDir"
Cohesion: 0.18
Nodes (5): Func, expandPath(), Time, NewMCPServer(), Server

### Community 111 - "codegen_test.go"
Cohesion: 0.34
Nodes (14): fakeProvider, aggSpec(), gen(), Spec, T, mustContain(), richSpec(), TestAggregateAllLanguages() (+6 more)

### Community 112 - "newApp"
Cohesion: 0.27
Nodes (9): NewLifecycle(), BeforeClose(), MenuEvent(), appMenu(), App, newApp(), runDesktop(), runMCP() (+1 more)

### Community 113 - "call"
Cohesion: 0.22
Nodes (4): call, FieldResult, FieldUpdate, Source

### Community 115 - "tools.go"
Cohesion: 0.27
Nodes (13): commonDirs(), Detect(), detectOne(), exe(), firstLine(), ToolsConfig, isFile(), locate() (+5 more)

### Community 116 - "BuildClientOptions"
Cohesion: 0.17
Nodes (21): ClientOptions, applyAdvanced(), applyAuth(), applyTLS(), BuildClientOptions(), CheckTLSFiles(), AdvancedConfig, Profile (+13 more)

### Community 118 - "mcpserver_test.go"
Cohesion: 0.33
Nodes (12): ClientSession, call(), connectClient(), Policy, T, TestAuditLogOmitsArguments(), TestBoundaryEnforcementBeforeConnecting(), TestCheckValueRefusesJavaScriptAndWrites() (+4 more)

### Community 119 - "ResultsView.tsx"
Cohesion: 0.07
Nodes (38): api(), cell(), doc, Extra, focusCell(), fv(), Harness(), page (+30 more)

### Community 120 - "ProfileInput"
Cohesion: 0.25
Nodes (7): ProfileInput, AdvancedConfig, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig

### Community 150 - "Invalid"
Cohesion: 0.13
Nodes (16): AdvancedConfig, HostPort, ParsedURI, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig (+8 more)

### Community 152 - "ParseCron"
Cohesion: 0.39
Nodes (4): Time, ParseCron(), parseField(), Cron

### Community 153 - "profiles.go"
Cohesion: 0.60
Nodes (4): Capability, agentNote(), agentSupported(), goos()

### Community 161 - "profiles_test.go"
Cohesion: 0.46
Nodes (7): fullInput(), T, TestDuplicateAndDeleteProfile(), TestParseConnectionStringNeverReturnsThePassword(), TestProfileValidationErrorsReachTheUI(), TestSaveProfileAllTabsAndSecrets(), ProfileInput

### Community 163 - "Mango3S UI reference: Welcome tab, connections, editing and shortcuts"
Cohesion: 0.20
Nodes (9): Collection query editor and paging, Connection Manager, Document and field actions (right-click menu), Keyboard shortcuts, Mango3S UI reference: Welcome tab, connections, editing and shortcuts, Preferences → Keymap, Result views and field editing, Unsupported and limitations (+1 more)

### Community 244 - "Store"
Cohesion: 0.20
Nodes (11): Profile, Store, AdvancedConfig, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, Time (+3 more)

## Knowledge Gaps
- **252 isolated node(s):** `Welcome tab and workspace tabs`, `Connection Manager`, `Collection query editor and paging`, `Result views and field editing`, `Document and field actions (right-click menu)` (+247 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **173 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `NewID()` connect `Scheduler` to `setup`, `backup.go`, `config.go`, `WriteJSON`, `.evalColl`, `Manager`, `Store`, `dialect.go`, `Store`?**
  _High betweenness centrality (0.067) - this node is a cross-community bridge._
- **Why does `App` connect `App` to `.previewTransform`, `Info`, `.backupDir`, `call`, `Outcome`, `p2.go`, `providers_test.go`, `Preview`, `MCPInfo`?**
  _High betweenness centrality (0.065) - this node is a cross-community bridge._
- **Why does `Run` connect `.convertValues` to `models.ts`, `.backupDir`?**
  _High betweenness centrality (0.063) - this node is a cross-community bridge._
- **Are the 36 inferred relationships involving `Invalid()` (e.g. with `.validate()` and `.Validate()`) actually correct?**
  _`Invalid()` has 36 INFERRED edges - model-reasoned connections that need verification._
- **Are the 36 inferred relationships involving `Compact()` (e.g. with `.Insert()` and `.CollectionStats()`) actually correct?**
  _`Compact()` has 36 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Welcome tab and workspace tabs`, `Connection Manager`, `Collection query editor and paging` to the rest of the system?**
  _252 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `transfer.go` be split into smaller, more focused modules?**
  _Cohesion score 0.1361344537815126 - nodes in this community are weakly interconnected._