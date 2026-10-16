# Graph Report - Mango3S  (2026-10-16)

## Corpus Check
- 192 files · ~203,001 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 3167 nodes · 6950 edges · 257 communities (93 shown, 164 thin omitted)
- Extraction: 90% EXTRACTED · 10% INFERRED · 0% AMBIGUOUS · INFERRED: 696 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `f4b3bc86`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- transfer.go
- Invalid
- queryBuilder.ts
- parser
- Parser
- WriteJSON
- .evalColl
- codegen.go
- App
- Compile
- DocViews.tsx
- Manager
- Analyze
- Mango3S (MongoDB desktop IDE)
- types.ts
- ShellView.tsx
- schema.go
- SchemaView.tsx
- compilerOptions
- CollectionView.tsx
- ConnectionManager.tsx
- logging.go
- TabErrorBoundary
- .convertValues
- AppContext.tsx
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
- tabs.ts
- App.tsx
- CollectionStats
- Wrap
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
- .UpdateField
- providers_test.go
- New
- .Find
- csv.go
- denied
- App
- migrate_test.go
- Mango3S P2 reference
- Store
- privileges.go
- Is
- Service
- Service
- wait
- NewDocument
- Writer
- NewID
- newFixture
- Proposal
- server.go
- CodeRequest
- DBResult
- DocDiff
- .previewTransform
- codegen_test.go
- Apply
- Options
- Outcome
- tools.go
- BuildClientOptions
- Result
- mcpserver_test.go
- docviews.test.tsx
- options_test.go
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
- tunneledConn
- Manager
- Preview
- DBOptions
- WaitGroup
- Document
- Entry
- Logger
- profiles_test.go
- FieldResult
- Mango3S UI reference: Welcome tab, connections, editing and shortcuts
- MCPInfo
- ParsedURI
- Plan
- Context
- ImportOptions
- Profile
- Page
- RWMutex
- TransformPreview
- ServerConfig
- CLIInfo
- TableMapping
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
- Service
- LocalServer
- DBResult
- Stage
- Duration
- Entry
- ExportOptions
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
- Mutex
- Op
- Options
- Outcome
- Output
- p2
- Page
- Preview
- Provider
- Result
- RuleSet
- SessionInfo
- Store

## God Nodes (most connected - your core abstractions)
1. `App` - 58 edges
2. `App` - 50 edges
3. `Invalid()` - 44 edges
4. `useApp()` - 40 edges
5. `Compact()` - 40 edges
6. `Scheduler` - 37 edges
7. `call()` - 35 edges
8. `Collection()` - 35 edges
9. `toAppError()` - 34 edges
10. `Is()` - 34 edges

## Surprising Connections (you probably didn't know these)
- `newApp()` --calls--> `NewStore()`  [INFERRED]
  main.go → internal/config/config.go
- `newApp()` --calls--> `NewManager()`  [INFERRED]
  main.go → internal/mongox/manager.go
- `TreeView()` --indirect_call--> `key()`  [INFERRED]
  frontend/src/components/DocViews.tsx → frontend/src/App.test.tsx
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

## Communities (257 total, 164 thin omitted)

### Community 0 - "transfer.go"
Cohesion: 0.13
Nodes (31): ColumnSpec, ParseDelimiter(), DetectFormat(), exportQuery(), fieldsProjection(), firstLine(), Context, D (+23 more)

### Community 1 - "Invalid"
Cohesion: 0.08
Nodes (35): Code, Error, CollectionInfo, CollectionStats, DatabaseInfo, IndexInfo, Service, Check() (+27 more)

### Community 2 - "queryBuilder.ts"
Cohesion: 0.19
Nodes (20): Harness(), BuiltQuery, QueryBuilder(), BSON_TYPES, BuilderError, buildFilter(), buildProjection(), buildSort() (+12 more)

### Community 3 - "parser"
Cohesion: 0.08
Nodes (42): mqlCompact(), errAt(), lex(), isLiteralFunc(), parse(), compact(), T, TestErrorPosition() (+34 more)

### Community 5 - "Parser"
Cohesion: 0.07
Nodes (44): Decimal128, T, mustDec(), mustHex(), mustOID(), TestErrorPosition(), TestFormatShape(), TestParseDocumentVariants() (+36 more)

### Community 6 - "WriteJSON"
Cohesion: 0.05
Nodes (62): Int32, WriteJSON(), Mutex, Stage, Time, NewHistory(), NewStore(), sameRun() (+54 more)

### Community 7 - ".evalColl"
Cohesion: 0.13
Nodes (35): A, Collection, Cursor, D, Database, Stage, ParseQuery(), T (+27 more)

### Community 8 - "codegen.go"
Cohesion: 0.18
Nodes (35): Language, renderer, Spec, b64(), block(), csRenderer(), escapeRegex(), floatLit() (+27 more)

### Community 9 - "App"
Cohesion: 0.08
Nodes (18): CodeRequest, SQLRequest, SQLResult, SQLTranslation, ImportOptions, codeSpec(), errorMessage(), findRequest() (+10 more)

### Community 11 - "Compile"
Cohesion: 0.09
Nodes (33): canonical(), clip(), Compile(), D, Rand, info(), randomizeInt(), randomizeString() (+25 more)

### Community 12 - "DocViews.tsx"
Cohesion: 0.13
Nodes (36): CellEditing, CellEditor(), CellRef, EditState, ResultsNotice, sameRef(), scrollIntoView(), step() (+28 more)

### Community 13 - "Manager"
Cohesion: 0.12
Nodes (14): CancelFunc, Context, Duration, Logger, Mutex, Time, WaitGroup, NewManager() (+6 more)

### Community 14 - "Analyze"
Cohesion: 0.08
Nodes (52): Request, Service, Stage, Detail, Diagnostic, m, Plan, Request (+44 more)

### Community 15 - "Mango3S (MongoDB desktop IDE)"
Cohesion: 0.06
Nodes (38): app service (GUI, Linux X11), integration-tests service (test profile), mongo service (MongoDB 8.0, seeded), mongo-auth service (auth profile), docker/mongo-init seed scripts, /src/main.tsx entry script, index.html #root mount, Aggregation Editor (+30 more)

### Community 16 - "types.ts"
Cohesion: 0.03
Nodes (104): AIDialog(), app, backend, base, Checks(), ConfirmInput(), EndpointPicker(), JobStatus() (+96 more)

### Community 17 - "ShellView.tsx"
Cohesion: 0.09
Nodes (31): KIND_LABELS, needsCollection(), openAction(), timeAgo(), ctx, labels(), COLLECTION_METHODS, currentCollection() (+23 more)

### Community 18 - "schema.go"
Cohesion: 0.12
Nodes (25): Field, Context, D, DateTime, lessPath(), New(), NewAnalyzer(), ptr() (+17 more)

### Community 19 - "SchemaView.tsx"
Cohesion: 0.16
Nodes (19): Metric(), stageClass(), StageNode(), elapsed(), jobProgress(), JobRow(), JobsIndicator(), formatBytes() (+11 more)

### Community 20 - "compilerOptions"
Cohesion: 0.07
Nodes (28): compilerOptions, allowJs, allowSyntheticDefaultImports, esModuleInterop, forceConsistentCasingInFileNames, isolatedModules, jsx, lib (+20 more)

### Community 21 - "CollectionView.tsx"
Cohesion: 0.10
Nodes (46): CodeDialog(), CodeEditor(), CodeEditorProps, HELPERS, OPERATORS, selectionOrAll(), theme, ErrorBanner() (+38 more)

### Community 22 - "ConnectionManager.tsx"
Cohesion: 0.08
Nodes (33): blankProfile(), CONNECTION_TABS, ConnectionManager(), emptyAdvanced(), ERROR_TABS, FolderNode, folderTree(), fromProfile() (+25 more)

### Community 23 - "logging.go"
Cohesion: 0.14
Nodes (17): Attr, Handler, Context, LevelVar, Logger, isSecretKey(), NewRedactingHandler(), ParseLevel() (+9 more)

### Community 25 - ".convertValues"
Cohesion: 0.04
Nodes (15): AIProviders, Field, Job, Output, ProfileInput, QuerySpec, Report, Request (+7 more)

### Community 26 - "AppContext.tsx"
Cohesion: 0.16
Nodes (17): MCPSettings(), onJobUpdate(), TabsAction, TabsState, AppInfo, Job, SessionInfo, Settings (+9 more)

### Community 28 - ".cancel"
Cohesion: 0.15
Nodes (10): AppInfo, Lifecycle, opRegistry, runningOp, CancelFunc, callWithTimeout(), Duration, Mutex (+2 more)

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
Nodes (19): Deps, Bool, CollectionInfo, CollectionStats, DatabaseInfo, History, call(), App (+11 more)

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
Cohesion: 0.12
Nodes (37): Clone(), CloneDoc(), Delete(), Exists(), Get(), D, index(), Parent() (+29 more)

### Community 38 - "compilerOptions"
Cohesion: 0.22
Nodes (8): compilerOptions, allowSyntheticDefaultImports, composite, module, moduleResolution, skipLibCheck, include, vite.config.ts

### Community 39 - "runtime.d.ts"
Cohesion: 0.25
Nodes (7): EnvironmentInfo, NotificationAction, NotificationCategory, NotificationOptions, Position, Screen, Size

### Community 40 - "Scheduler"
Cohesion: 0.16
Nodes (16): CancelFunc, Context, Duration, Logger, Mutex, RawMessage, Time, WaitGroup (+8 more)

### Community 41 - "scripts"
Cohesion: 0.29
Nodes (7): scripts, build, dev, preview, test, test:watch, typecheck

### Community 42 - "01-sample-data.js"
Cohesion: 0.33
Nodes (5): customers, orders, products, statuses, store

### Community 43 - "frontend/package.json"
Cohesion: 0.40
Nodes (4): name, private, type, version

### Community 44 - "tabs.ts"
Cohesion: 0.15
Nodes (21): EditorStage, newStage(), PipelineAction, pipelineReducer(), STAGE_TEMPLATES, stageKey(), templateFor(), toStages() (+13 more)

### Community 45 - "App.tsx"
Cohesion: 0.05
Nodes (72): backend, jobListeners, key(), profile, session, ConfirmDialog(), PromptDialog(), Toasts() (+64 more)

### Community 47 - "Wrap"
Cohesion: 0.14
Nodes (24): ClientConfig, HostKeyCallback, Wrap(), buildDialer(), classifySSH(), containsAny(), Client, Conn (+16 more)

### Community 48 - "App"
Cohesion: 0.05
Nodes (10): Info, App, Duration, Job, Provider, SetAIProvider(), Task, TaskType (+2 more)

### Community 50 - "compare.go"
Cohesion: 0.05
Nodes (73): CollectionRow, DBOptions, DBResult, DocDiff, Endpoint, Entry, FieldChange, fingerprint (+65 more)

### Community 51 - "ai.go"
Cohesion: 0.10
Nodes (34): answer, Check, Claude, Context, FieldSummary, Proposal, Provider, Request (+26 more)

### Community 52 - "p2.go"
Cohesion: 0.11
Nodes (19): AIProviders, AIStatus, exportTask, maskCopyTask, MCPInfo, p2, syncTask, xformPlan (+11 more)

### Community 53 - "migrate.go"
Cohesion: 0.23
Nodes (24): contains(), copyTable(), embed(), Execute(), fieldFor(), Collection, Context, D (+16 more)

### Community 54 - "Session"
Cohesion: 0.21
Nodes (12): describe(), Client, Logger, Profile, RWMutex, Time, open(), via() (+4 more)

### Community 55 - "Compact"
Cohesion: 0.25
Nodes (19): allDocs(), byID(), Context, D, Job, Manager, T, representative() (+11 more)

### Community 56 - "transfer_test.go"
Cohesion: 0.12
Nodes (29): Decoder, Equal(), NewCSVReader(), D, NewJSONReader(), NewJSONWriter(), truncate(), dataset() (+21 more)

### Community 57 - "dialect.go"
Cohesion: 0.22
Nodes (21): DB, Redact(), beginRead(), dialectFor(), Context, Rows, Tx, Introspect() (+13 more)

### Community 69 - "Store"
Cohesion: 0.12
Nodes (22): Profile, profilesFile, Settings, Store, DefaultSettings(), AdvancedConfig, Mutex, ProxyConfig (+14 more)

### Community 79 - "EventsOn"
Cohesion: 0.67
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 83 - "local.go"
Cohesion: 0.20
Nodes (14): httpStatus, Local, LocalServer, Error, asStatus(), checkURL(), DetectLocalServers(), doJSON() (+6 more)

### Community 84 - ".register"
Cohesion: 0.21
Nodes (12): CheckValue(), Context, D, Logger, Policy, Profile, RawMessage, New() (+4 more)

### Community 85 - "mql/format.go"
Cohesion: 0.20
Nodes (13): Buffer, CanonicalJSON(), escapeRegexSlashes(), Format(), formatTime(), D, Time, Lookup() (+5 more)

### Community 86 - ".UpdateField"
Cohesion: 0.20
Nodes (19): FieldResult, FieldUpdate, FieldValue, checkParents(), describeField(), EditText(), D, Document (+11 more)

### Community 87 - "providers_test.go"
Cohesion: 0.25
Nodes (15): call, T, TestLiveProviders(), ExtractJSON(), argAfter(), fakeRunner(), T, hasArg() (+7 more)

### Community 88 - "New"
Cohesion: 0.23
Nodes (12): ClaudeCode, CLIInfo, Codex, cliError(), DetectClaudeCode(), DetectCodex(), emptyDir(), exeName() (+4 more)

### Community 89 - ".Find"
Cohesion: 0.20
Nodes (15): CollectionRef(), Context, D, Page, New(), Parse(), parseField(), T (+7 more)

### Community 90 - "csv.go"
Cohesion: 0.17
Nodes (13): FormatDouble(), FormatCell(), D, LeafPaths(), LookupPath(), NewCSVWriter(), setPath(), validType() (+5 more)

### Community 91 - "denied"
Cohesion: 0.24
Nodes (8): contains(), denied(), Mutex, NewStore(), toolInfo(), Policy, Store, ToolInfo

### Community 92 - "App"
Cohesion: 0.06
Nodes (32): Capability, ProfileInput, SecretInput, Credentials, FieldResult, FieldUpdate, FieldValue, NewLifecycle() (+24 more)

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

### Community 97 - "Is"
Cohesion: 0.06
Nodes (67): AEAD, env, Is(), D, T, Time, mustTime(), rawDoc() (+59 more)

### Community 98 - "Service"
Cohesion: 0.20
Nodes (8): Service, WritePreview, WriteResult, D, Provider, MaskShape(), NewService(), Rand

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

### Community 103 - "NewID"
Cohesion: 0.28
Nodes (3): NewID(), Run, runsFile

### Community 104 - "newFixture"
Cohesion: 0.31
Nodes (13): Duration, Mutex, T, Time, newFixture(), TestApprovalGatesExecution(), TestCronParseAndNext(), TestNoOverlap() (+5 more)

### Community 106 - "server.go"
Cohesion: 0.24
Nodes (13): CallToolResult, In, add(), errorResult(), textResult(), aggIn, collIn, connIn (+5 more)

### Community 110 - ".previewTransform"
Cohesion: 0.21
Nodes (7): TransformPreview, TransformRequest, Func, expandPath(), Time, NewMCPServer(), Server

### Community 111 - "codegen_test.go"
Cohesion: 0.34
Nodes (14): fakeProvider, aggSpec(), gen(), Spec, T, mustContain(), richSpec(), TestAggregateAllLanguages() (+6 more)

### Community 112 - "Apply"
Cohesion: 0.26
Nodes (16): Check, Apply(), Collection, Context, D, Source, idOf(), RunPreview() (+8 more)

### Community 115 - "tools.go"
Cohesion: 0.27
Nodes (13): commonDirs(), Detect(), detectOne(), exe(), firstLine(), ToolsConfig, isFile(), locate() (+5 more)

### Community 116 - "BuildClientOptions"
Cohesion: 0.32
Nodes (12): ClientOptions, applyAdvanced(), applyAuth(), applyTLS(), BuildClientOptions(), CheckTLSFiles(), AdvancedConfig, Profile (+4 more)

### Community 118 - "mcpserver_test.go"
Cohesion: 0.33
Nodes (12): ClientSession, call(), connectClient(), Policy, T, TestAuditLogOmitsArguments(), TestBoundaryEnforcementBeforeConnecting(), TestCheckValueRefusesJavaScriptAndWrites() (+4 more)

### Community 119 - "docviews.test.tsx"
Cohesion: 0.22
Nodes (6): ada(), fields, grace, Harness(), page(), FieldResult

### Community 120 - "options_test.go"
Cohesion: 0.33
Nodes (9): T, Time, TestAdvancedOptionsOverrideURI(), TestAllowInvalidHostnamesStillVerifiesTheChain(), TestAuthMechanismsAndProperties(), TestTLSErrorsAreClassified(), TestTLSFilesAreValidatedWithClearMessages(), writeCert() (+1 more)

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
Cohesion: 0.25
Nodes (7): Collection query editor and paging, Connection Manager, Keyboard shortcuts, Mango3S UI reference: Welcome tab, connections, editing and shortcuts, Result views and field editing, Unsupported and limitations, Welcome tab and workspace tabs

## Knowledge Gaps
- **235 isolated node(s):** `Data-integrity model`, `Compare and sync`, `Reschema and masking`, `Migration from relational databases`, `Policies` (+230 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **164 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `App`, `newFixture`, `Store`, `App`, `.cancel`, `.previewTransform`, `App`, `local.go`, `p2.go`, `providers_test.go`, `App`?**
  _High betweenness centrality (0.125) - this node is a cross-community bridge._
- **Why does `NewID()` connect `NewID` to `Is`, `backup.go`, `Store`, `WriteJSON`, `Scheduler`, `Compile`, `Manager`, `Compact`, `dialect.go`, `Store`?**
  _High betweenness centrality (0.104) - this node is a cross-community bridge._
- **Why does `Run` connect `.convertValues` to `models.ts`, `.previewTransform`?**
  _High betweenness centrality (0.090) - this node is a cross-community bridge._
- **Are the 41 inferred relationships involving `Invalid()` (e.g. with `.validate()` and `.Validate()`) actually correct?**
  _`Invalid()` has 41 INFERRED edges - model-reasoned connections that need verification._
- **Are the 38 inferred relationships involving `Compact()` (e.g. with `.Insert()` and `.UpdateField()`) actually correct?**
  _`Compact()` has 38 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Data-integrity model`, `Compare and sync`, `Reschema and masking` to the rest of the system?**
  _235 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `transfer.go` be split into smaller, more focused modules?**
  _Cohesion score 0.13333333333333333 - nodes in this community are weakly interconnected._