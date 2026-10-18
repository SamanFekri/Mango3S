# Graph Report - Mango3S  (2026-10-18)

## Corpus Check
- 204 files · ~220,873 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 3337 nodes · 7267 edges · 289 communities (97 shown, 192 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 668 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `5517c6b4`
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
- NewDocument
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
- App.tsx
- .convertValues
- open
- Manager
- .cancel
- dependencies
- devDependencies
- runtime/package.json
- App
- newFixture
- Writer
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
- shortcuts.tsx
- CollectionStats
- dial_test.go
- App
- Item
- compare.go
- ai.go
- p2.go
- migrate.go
- BuildClientOptions
- AppContext.tsx
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
- EventsOnMultiple
- mango3s
- local.go
- server.go
- mql/format.go
- shortcuts.ts
- providers_test.go
- New
- .Find
- csv.go
- profiles.go
- App
- migrate_test.go
- Mango3S P2 reference
- Store
- TasksView.tsx
- p2_test.go
- Service
- ReadJSON
- wait
- setup
- ai_test.go
- Info
- newFixture
- cli.go
- privileges.go
- CodeRequest
- DBResult
- DocDiff
- .backupDir
- codegen_test.go
- .Generate
- App.test.tsx
- Outcome
- tools.go
- .aiProvider
- TestLiveProviders
- KeymapSettings.tsx
- docactions.test.tsx
- WelcomeView.tsx
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
- TestFieldLevelEditing
- Manager
- Preview
- DBOptions
- WaitGroup
- Document
- .previewTransform
- call
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
- keymap.test.tsx
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
- Document
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
- Session
- Buffer
- Document
- Request
- LocalServer
- DBResult
- .encode
- SessionInfo
- Mutex
- SQLResult
- Page
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
- Provider
- Op
- Options
- Outcome
- Entry
- Stage
- Info
- Preview
- Table
- AdvancedConfig
- RuleSet
- ProxyConfig
- ServerConfig
- BulkPreview
- BulkResult
- BulkWriteRequest
- Duration
- ExportOptions
- Job
- Provider
- Context
- ShellConfig
- Mutex
- Document
- WritePreview
- WriteResult
- SSHConfig
- TLSConfig
- ToolsConfig
- Profile
- Time
- Duration
- Logger
- RWMutex
- T
- Context
- Credentials
- Session
- SessionInfo
- TestResult
- App
- Output
- Report
- Request
- Result

## God Nodes (most connected - your core abstractions)
1. `App` - 58 edges
2. `useApp()` - 51 edges
3. `App` - 51 edges
4. `Invalid()` - 38 edges
5. `Compact()` - 38 edges
6. `Scheduler` - 37 edges
7. `toAppError()` - 33 edges
8. `Is()` - 32 edges
9. `Manager` - 32 edges
10. `Collection()` - 31 edges

## Surprising Connections (you probably didn't know these)
- `newApp()` --calls--> `New()`  [INFERRED]
  main.go → internal/api/api.go
- `runDesktop()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go
- `runMCP()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go
- `appMenu()` --calls--> `MenuEvent()`  [INFERRED]
  main.go → internal/api/profiles.go
- `runDesktop()` --calls--> `BeforeClose()`  [INFERRED]
  main.go → internal/api/profiles.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Editors sharing the mql value language** — readme_query_editor, readme_aggregation_editor, readme_intellishell, readme_visual_query_builder, readme_mql_shared_value_language [EXTRACTED 1.00]
- **P1 features reusing P0 query/aggregate services** — readme_sql_subset, readme_code_generation, readme_explain_plans, readme_import_export_wizard, readme_p1_reuses_p0 [EXTRACTED 1.00]
- **Docker compose dev/test stack** — docker_compose_mongo, docker_compose_mongo_auth, docker_compose_integration_tests, docker_compose_app, docker_compose_mongo_init [EXTRACTED 1.00]

## Communities (289 total, 192 thin omitted)

### Community 0 - "transfer.go"
Cohesion: 0.14
Nodes (30): ColumnSpec, ParseDelimiter(), DetectFormat(), exportQuery(), fieldsProjection(), firstLine(), Context, D (+22 more)

### Community 1 - "Invalid"
Cohesion: 0.09
Nodes (30): Code, Error, CollectionInfo, CollectionStats, DatabaseInfo, IndexInfo, Service, Check() (+22 more)

### Community 2 - "Collection"
Cohesion: 0.29
Nodes (18): allDocs(), byID(), Context, D, Job, Manager, T, representative() (+10 more)

### Community 3 - "parser"
Cohesion: 0.08
Nodes (43): mqlCompact(), errAt(), lex(), isLiteralFunc(), parse(), compact(), T, TestErrorPosition() (+35 more)

### Community 5 - "Parser"
Cohesion: 0.07
Nodes (46): Decimal128, TypeName(), T, mustDec(), mustHex(), mustOID(), TestErrorPosition(), TestFormatShape() (+38 more)

### Community 6 - "WriteJSON"
Cohesion: 0.06
Nodes (34): WriteJSON(), Mutex, Stage, Time, NewHistory(), NewStore(), sameRun(), T (+26 more)

### Community 7 - ".evalColl"
Cohesion: 0.06
Nodes (64): A, BulkPreview, BulkResult, BulkWriteRequest, Collection, Cursor, D, Database (+56 more)

### Community 8 - "codegen.go"
Cohesion: 0.18
Nodes (35): Language, renderer, Spec, b64(), block(), csRenderer(), escapeRegex(), floatLit() (+27 more)

### Community 9 - "App"
Cohesion: 0.08
Nodes (18): CodeRequest, SQLRequest, SQLResult, SQLTranslation, ImportOptions, codeSpec(), errorMessage(), findRequest() (+10 more)

### Community 11 - "NewDocument"
Cohesion: 0.11
Nodes (28): Request, Service, Stage, Build(), FromText(), Context, Page, indentLines() (+20 more)

### Community 12 - "DocViews.tsx"
Cohesion: 0.06
Nodes (69): key(), CellEditing, CellEditor(), CellRef, EditState, ResultsNotice, sameRef(), scrollIntoView() (+61 more)

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
Nodes (101): AIDialog(), app, backend, base, Checks(), ConfirmInput(), EndpointPicker(), JobStatus() (+93 more)

### Community 17 - "ShellView.tsx"
Cohesion: 0.07
Nodes (41): KIND_LABELS, needsCollection(), openAction(), timeAgo(), ctx, labels(), COLLECTION_METHODS, currentCollection() (+33 more)

### Community 18 - "schema.go"
Cohesion: 0.12
Nodes (24): Field, D, DateTime, lessPath(), New(), NewAnalyzer(), ptr(), field() (+16 more)

### Community 19 - "SchemaView.tsx"
Cohesion: 0.17
Nodes (18): Metric(), stageClass(), StageNode(), elapsed(), jobProgress(), JobRow(), JobsIndicator(), formatBytes() (+10 more)

### Community 20 - "compilerOptions"
Cohesion: 0.07
Nodes (28): compilerOptions, allowJs, allowSyntheticDefaultImports, esModuleInterop, forceConsistentCasingInFileNames, isolatedModules, jsx, lib (+20 more)

### Community 21 - "CollectionView.tsx"
Cohesion: 0.10
Nodes (51): BulkWriteDialog(), CodeDialog(), CodeEditor(), CodeEditorProps, HELPERS, OPERATORS, selectionOrAll(), theme (+43 more)

### Community 22 - "ConnectionManager.tsx"
Cohesion: 0.09
Nodes (29): blankProfile(), CONNECTION_TABS, ConnectionManager(), emptyAdvanced(), ERROR_TABS, FolderNode, folderTree(), fromProfile() (+21 more)

### Community 23 - "logging.go"
Cohesion: 0.14
Nodes (17): Attr, Handler, Context, LevelVar, Logger, isSecretKey(), NewRedactingHandler(), ParseLevel() (+9 more)

### Community 24 - "App.tsx"
Cohesion: 0.09
Nodes (39): GlobalShortcuts(), Shell(), StatusBar(), TabBar(), AboutDialog(), aboutText(), SECRET_STORE, ConfirmDialog() (+31 more)

### Community 25 - ".convertValues"
Cohesion: 0.04
Nodes (15): AIProviders, Entry, ImportOptions, MCPInfo, Options, Output, Page, Proposal (+7 more)

### Community 26 - "open"
Cohesion: 0.14
Nodes (19): Certificate, via(), certSummary(), describeProfile(), Client, Context, Credentials, Duration (+11 more)

### Community 28 - ".cancel"
Cohesion: 0.16
Nodes (10): AppInfo, Lifecycle, opRegistry, runningOp, CancelFunc, Duration, callWithTimeout(), newOpRegistry() (+2 more)

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
Nodes (19): Deps, Bool, CollectionInfo, CollectionStats, ConnectionStep, DatabaseInfo, History, call() (+11 more)

### Community 33 - "newFixture"
Cohesion: 0.09
Nodes (30): AEAD, fixture, decodeErr(), Buffer, T, newFixture(), TestCancelOperation(), TestConnectFailureIsReadable() (+22 more)

### Community 34 - "Writer"
Cohesion: 0.09
Nodes (39): Header, Info, line, ManyResult, RestoreStats, Writer, Check, File (+31 more)

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

### Community 45 - "shortcuts.tsx"
Cohesion: 0.12
Nodes (19): detectPlatform(), isTextTarget(), Platform, ShortcutDef, SHORTCUTS, BY_PRIORITY, dialogOpen(), handleKeyDown() (+11 more)

### Community 47 - "dial_test.go"
Cohesion: 0.06
Nodes (58): Addr, ClientConfig, HostKeyCallback, Int32, buildDialer(), classifySSH(), containsAny(), Client (+50 more)

### Community 48 - "App"
Cohesion: 0.09
Nodes (3): App, Task, TaskType

### Community 50 - "compare.go"
Cohesion: 0.05
Nodes (73): CollectionRow, DBOptions, DBResult, DocDiff, Endpoint, Entry, FieldChange, fingerprint (+65 more)

### Community 51 - "ai.go"
Cohesion: 0.21
Nodes (16): answer, Check, Context, FieldSummary, Proposal, Provider, Request, BuildContext() (+8 more)

### Community 52 - "p2.go"
Cohesion: 0.11
Nodes (21): AIProviders, AIStatus, exportTask, maskCopyTask, MCPInfo, p2, syncTask, xformPlan (+13 more)

### Community 53 - "migrate.go"
Cohesion: 0.23
Nodes (24): contains(), copyTable(), embed(), Execute(), fieldFor(), Collection, Context, D (+16 more)

### Community 54 - "BuildClientOptions"
Cohesion: 0.17
Nodes (21): ClientOptions, applyAdvanced(), applyAuth(), applyTLS(), BuildClientOptions(), CheckTLSFiles(), AdvancedConfig, Profile (+13 more)

### Community 55 - "AppContext.tsx"
Cohesion: 0.12
Nodes (25): ConnectionLogDialog(), ICON, logText(), TabProps, onConnectionStep(), onJobUpdate(), onMenu(), TabsAction (+17 more)

### Community 56 - "Compact"
Cohesion: 0.12
Nodes (30): Decoder, Compact(), Equal(), NewCSVReader(), D, NewJSONReader(), NewJSONWriter(), truncate() (+22 more)

### Community 57 - "dialect.go"
Cohesion: 0.22
Nodes (21): DB, Redact(), beginRead(), dialectFor(), Context, Rows, Tx, Introspect() (+13 more)

### Community 69 - "Store"
Cohesion: 0.11
Nodes (23): Profile, profilesFile, Settings, Store, DefaultSettings(), AdvancedConfig, ProxyConfig, ServerConfig (+15 more)

### Community 83 - "local.go"
Cohesion: 0.20
Nodes (12): httpStatus, Local, LocalServer, Error, asStatus(), checkURL(), DetectLocalServers(), doJSON() (+4 more)

### Community 84 - "server.go"
Cohesion: 0.07
Nodes (45): CallToolResult, ClientSession, In, call(), connectClient(), Policy, T, TestAuditLogOmitsArguments() (+37 more)

### Community 85 - "mql/format.go"
Cohesion: 0.24
Nodes (12): Buffer, CanonicalJSON(), escapeRegexSlashes(), Format(), formatTime(), D, Time, Lookup() (+4 more)

### Community 86 - "shortcuts.ts"
Cohesion: 0.13
Nodes (13): CODE_KEYS, Conflict, eventCombo(), eventKey(), Handler, KEY_LABELS, KeymapOverrides, MAC_SYMBOLS (+5 more)

### Community 87 - "providers_test.go"
Cohesion: 0.34
Nodes (12): argAfter(), fakeRunner(), T, hasArg(), TestClaudeCodeErrors(), TestClaudeCodeRunsWithoutToolsOrSettings(), TestCodexUsesReadOnlySandboxAndSchemaFile(), TestDetectClaudeCode() (+4 more)

### Community 88 - "New"
Cohesion: 0.31
Nodes (11): New(), Logger, NewManager(), T, TestStepsAgainstRealServer(), T, record(), states() (+3 more)

### Community 89 - ".Find"
Cohesion: 0.20
Nodes (15): CollectionRef(), Context, D, Page, New(), Parse(), parseField(), T (+7 more)

### Community 90 - "csv.go"
Cohesion: 0.17
Nodes (13): FormatDouble(), FormatCell(), D, LeafPaths(), LookupPath(), NewCSVWriter(), setPath(), validType() (+5 more)

### Community 91 - "profiles.go"
Cohesion: 0.15
Nodes (18): Capability, ConnectionStep, App, NewLifecycle(), agentNote(), agentSupported(), BeforeClose(), goos() (+10 more)

### Community 92 - "App"
Cohesion: 0.07
Nodes (20): AdvancedConfig, ProfileInput, SecretInput, Credentials, FieldResult, FieldUpdate, FieldValue, App (+12 more)

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
Cohesion: 0.27
Nodes (9): Task, TaskRun, TaskSchedule, TaskType, Draft, isApproved(), TasksView(), TEMPLATES (+1 more)

### Community 97 - "p2_test.go"
Cohesion: 0.25
Nodes (22): env, coll(), App, Source, T, mongoURI(), newApp(), runMigration() (+14 more)

### Community 98 - "Service"
Cohesion: 0.20
Nodes (8): Service, WritePreview, WriteResult, D, Provider, MaskShape(), NewService(), Rand

### Community 99 - "ReadJSON"
Cohesion: 0.29
Nodes (8): ReadJSON(), Mutex, Source, Time, NewService(), Run, runsFile, Service

### Community 100 - "wait"
Cohesion: 0.45
Nodes (11): Job, Manager, T, newManager(), TestCancelRunningAndQueued(), TestIgnoredCancellationStillCancels(), TestJobFailureAndPanic(), TestJobSucceedsWithProgress() (+3 more)

### Community 101 - "setup"
Cohesion: 0.31
Nodes (15): contains(), containsDB(), DatabaseInfo, T, nField(), seed(), setup(), TestAggregation() (+7 more)

### Community 102 - "ai_test.go"
Cohesion: 0.32
Nodes (11): AnswerSchema(), answerJSON(), checkNames(), contains(), Proposal, T, TestContextNeverLeaksValues(), TestSystemPromptHasNoConnectionDetails() (+3 more)

### Community 104 - "newFixture"
Cohesion: 0.31
Nodes (13): Duration, Mutex, T, Time, newFixture(), TestApprovalGatesExecution(), TestCronParseAndNext(), TestNoOverlap() (+5 more)

### Community 105 - "cli.go"
Cohesion: 0.28
Nodes (10): ClaudeCode, CLIInfo, Codex, cliError(), DetectClaudeCode(), DetectCodex(), emptyDir(), exeName() (+2 more)

### Community 106 - "privileges.go"
Cohesion: 0.31
Nodes (9): contains(), D, granted(), MissingActions(), T, priv(), TestMissingActions(), connStatus (+1 more)

### Community 110 - ".backupDir"
Cohesion: 0.18
Nodes (5): Func, expandPath(), Time, NewMCPServer(), Server

### Community 111 - "codegen_test.go"
Cohesion: 0.34
Nodes (14): fakeProvider, aggSpec(), gen(), Spec, T, mustContain(), richSpec(), TestAggregateAllLanguages() (+6 more)

### Community 112 - ".Generate"
Cohesion: 0.31
Nodes (7): Claude, Usage, Client, classify(), Context, NewClaude(), supportsDefaultFallback()

### Community 113 - "App.test.tsx"
Cohesion: 0.14
Nodes (10): TabErrorBoundary, backend, connectAndOpenOrders(), dismissConnectLog(), jobListeners, menuListeners, profile, session (+2 more)

### Community 115 - "tools.go"
Cohesion: 0.27
Nodes (13): commonDirs(), Detect(), detectOne(), exe(), firstLine(), ToolsConfig, isFile(), locate() (+5 more)

### Community 117 - "TestLiveProviders"
Cohesion: 0.50
Nodes (3): T, TestLiveProviders(), ExtractJSON()

### Community 118 - "KeymapSettings.tsx"
Cohesion: 0.32
Nodes (13): Keymap, KeymapSettings(), parseKeymapFile(), TEXT_EDITING, bindingFromEvent(), conflictsWith(), defaultKeysFor(), findConflicts() (+5 more)

### Community 119 - "docactions.test.tsx"
Cohesion: 0.09
Nodes (19): api(), cell(), doc, Extra, focusCell(), fv(), Harness(), page (+11 more)

### Community 120 - "WelcomeView.tsx"
Cohesion: 0.42
Nodes (7): SCOPE_LABEL, ShortcutsDialog(), formatBinding(), formatShortcut(), getShortcutPlatform(), TIPS, WelcomeView()

### Community 150 - "profile.go"
Cohesion: 0.09
Nodes (25): AdvancedConfig, HostPort, ParsedURI, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig (+17 more)

### Community 152 - "ParseCron"
Cohesion: 0.39
Nodes (4): Time, ParseCron(), parseField(), Cron

### Community 153 - "TestFieldLevelEditing"
Cohesion: 0.36
Nodes (8): D, T, Time, mustTime(), rawDoc(), TestFieldLevelEditing(), TestSystemCollectionsAreListedSeparately(), uriFromEnv()

### Community 161 - "profiles_test.go"
Cohesion: 0.46
Nodes (7): fullInput(), T, TestDuplicateAndDeleteProfile(), TestParseConnectionStringNeverReturnsThePassword(), TestProfileValidationErrorsReachTheUI(), TestSaveProfileAllTabsAndSecrets(), ProfileInput

### Community 163 - "Mango3S UI reference: Welcome tab, connections, editing and shortcuts"
Cohesion: 0.20
Nodes (9): Collection query editor and paging, Connection Manager, Document and field actions (right-click menu), Keyboard shortcuts, Mango3S UI reference: Welcome tab, connections, editing and shortcuts, Preferences → Keymap, Result views and field editing, Unsupported and limitations (+1 more)

### Community 171 - "keymap.test.tsx"
Cohesion: 0.29
Nodes (4): app, backend, rerender, SHORTCUT_BY_ID

### Community 206 - "Session"
Cohesion: 0.21
Nodes (11): describe(), Client, Context, Credentials, RWMutex, Context, Manager, Session (+3 more)

## Knowledge Gaps
- **258 isolated node(s):** `profile`, `session`, `backend`, `jobListeners`, `stepListeners` (+253 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **192 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Info` connect `Info` to `call`, `models.ts`, `App`, `.aiProvider`, `App`?**
  _High betweenness centrality (0.092) - this node is a cross-community bridge._
- **Why does `NewID()` connect `Scheduler` to `Collection`, `Writer`, `setup`, `Store`, `.evalColl`, `WriteJSON`, `Manager`, `dialect.go`, `Store`?**
  _High betweenness centrality (0.053) - this node is a cross-community bridge._
- **Why does `New()` connect `New` to `App`, `newFixture`, `cli.go`, `App`, `.cancel`, `Session`, `local.go`, `.encode`, `open`, `profiles.go`, `App`?**
  _High betweenness centrality (0.050) - this node is a cross-community bridge._
- **Are the 35 inferred relationships involving `Invalid()` (e.g. with `.validate()` and `.Validate()`) actually correct?**
  _`Invalid()` has 35 INFERRED edges - model-reasoned connections that need verification._
- **Are the 36 inferred relationships involving `Compact()` (e.g. with `.Insert()` and `.CollectionStats()`) actually correct?**
  _`Compact()` has 36 INFERRED edges - model-reasoned connections that need verification._
- **What connects `profile`, `session`, `backend` to the rest of the system?**
  _258 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `transfer.go` be split into smaller, more focused modules?**
  _Cohesion score 0.1361344537815126 - nodes in this community are weakly interconnected._