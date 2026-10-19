# Graph Report - Mango3S  (2026-10-19)

## Corpus Check
- 206 files · ~223,829 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 3352 nodes · 7260 edges · 311 communities (106 shown, 205 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 642 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `8ceadc94`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- transfer.go
- Store
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
- ShellView.tsx
- schema.go
- ExplainDialog.tsx
- compilerOptions
- Is
- ConnectionManager.tsx
- logging.go
- Sidebar.tsx
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
- Wrap
- App
- Item
- compare.go
- ai.go
- p2.go
- migrate.go
- BuildClientOptions
- CollectionView.tsx
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
- T
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
- server.go
- mql/format.go
- shortcuts.ts
- providers_test.go
- record
- Parse
- csv.go
- Invalid
- App
- migrate_test.go
- Mango3S P2 reference
- Store
- third-party-notices.sh
- setup
- AppContext.tsx
- ReadJSON
- wait
- TasksView.tsx
- Profile
- Info
- newFixture
- New
- appMenu
- CodeRequest
- DBResult
- DocDiff
- .backupDir
- codegen_test.go
- NewDocument
- App.test.tsx
- Outcome
- tools.go
- .aiProvider
- config.go
- KeymapSettings.tsx
- docactions.test.tsx
- App.tsx
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
- Profile
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
- profiles.go
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
- .encode
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
- .ExecuteBulkWrite
- Context
- Manager
- Collection
- Context
- NewStore
- Buffer
- Document
- Request
- LocalServer
- DBResult
- AdvancedConfig
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
- Session
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
- SessionInfo
- RuleSet
- Time
- T
- BulkPreview
- BulkResult
- BulkWriteRequest
- Duration
- ExportOptions
- Job
- Provider
- Context
- Profile
- Mutex
- Document
- WritePreview
- WriteResult
- ProxyConfig
- ServerConfig
- ShellConfig
- Profile
- Time
- Duration
- Logger
- RWMutex
- T
- Context
- Credentials
- SSHConfig
- TLSConfig
- TestResult
- App
- Output
- Report
- Request
- Result
- shell/parse.go
- ProfileInput
- ToolsConfig
- .Connect
- docviews.test.tsx
- NewJSONWriter
- keymap.test.tsx
- .UpdateField
- Profile
- Deps
- .connectProfile
- .ListConnections
- AdvancedConfig
- ProxyConfig
- ServerConfig
- ShellConfig
- SSHConfig
- TLSConfig
- ToolsConfig
- Source
- Stage
- T

## God Nodes (most connected - your core abstractions)
1. `App` - 58 edges
2. `App` - 51 edges
3. `useApp()` - 45 edges
4. `Compact()` - 37 edges
5. `Scheduler` - 37 edges
6. `App` - 34 edges
7. `toAppError()` - 33 edges
8. `Manager` - 32 edges
9. `Invalid()` - 30 edges
10. `Is()` - 29 edges

## Surprising Connections (you probably didn't know these)
- `newApp()` --calls--> `New()`  [INFERRED]
  main.go → internal/api/api.go
- `runDesktop()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go
- `runMCP()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go
- `TreeView()` --indirect_call--> `key()`  [INFERRED]
  frontend/src/components/DocViews.tsx → frontend/src/App.test.tsx
- `appMenu()` --calls--> `MenuEvent()`  [INFERRED]
  main.go → internal/api/profiles.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Editors sharing the mql value language** — readme_query_editor, readme_aggregation_editor, readme_intellishell, readme_visual_query_builder, readme_mql_shared_value_language [EXTRACTED 1.00]
- **P1 features reusing P0 query/aggregate services** — readme_sql_subset, readme_code_generation, readme_explain_plans, readme_import_export_wizard, readme_p1_reuses_p0 [EXTRACTED 1.00]
- **Docker compose dev/test stack** — docker_compose_mongo, docker_compose_mongo_auth, docker_compose_integration_tests, docker_compose_app, docker_compose_mongo_init [EXTRACTED 1.00]

## Communities (311 total, 205 thin omitted)

### Community 0 - "transfer.go"
Cohesion: 0.14
Nodes (30): ColumnSpec, ParseDelimiter(), DetectFormat(), exportQuery(), fieldsProjection(), firstLine(), Context, D (+22 more)

### Community 1 - "Store"
Cohesion: 0.41
Nodes (3): Store, sortProfiles(), New()

### Community 2 - "Collection"
Cohesion: 0.29
Nodes (18): allDocs(), byID(), Context, D, Job, Manager, T, representative() (+10 more)

### Community 3 - "parser"
Cohesion: 0.08
Nodes (42): mqlCompact(), errAt(), lex(), isLiteralFunc(), parse(), compact(), T, TestErrorPosition() (+34 more)

### Community 5 - "Parser"
Cohesion: 0.09
Nodes (32): TypeName(), T, mustDec(), mustHex(), mustOID(), TestErrorPosition(), TestFormatShape(), TestParseDocumentVariants() (+24 more)

### Community 6 - "WriteJSON"
Cohesion: 0.05
Nodes (62): Int32, WriteJSON(), Mutex, Stage, Time, NewHistory(), NewStore(), sameRun() (+54 more)

### Community 7 - ".evalColl"
Cohesion: 0.10
Nodes (38): A, Collection, Cursor, D, Database, CollectionInfo, CollectionStats, DatabaseInfo (+30 more)

### Community 8 - "codegen.go"
Cohesion: 0.18
Nodes (35): Language, renderer, Spec, b64(), block(), csRenderer(), escapeRegex(), floatLit() (+27 more)

### Community 9 - "App"
Cohesion: 0.08
Nodes (18): CodeRequest, SQLRequest, SQLResult, SQLTranslation, ImportOptions, codeSpec(), errorMessage(), findRequest() (+10 more)

### Community 11 - "Source"
Cohesion: 0.13
Nodes (24): Request, Service, Stage, Service, Build(), FromText(), Context, Page (+16 more)

### Community 12 - "DocViews.tsx"
Cohesion: 0.05
Nodes (71): page, CellEditing, CellEditor(), CellRef, EditState, FieldApi, ResultsNotice, sameRef() (+63 more)

### Community 13 - "Manager"
Cohesion: 0.12
Nodes (14): CancelFunc, Context, Duration, Logger, Mutex, Time, WaitGroup, NewManager() (+6 more)

### Community 14 - "Analyze"
Cohesion: 0.15
Nodes (30): Detail, Diagnostic, m, Plan, Request, Stage, Summary, Analyze() (+22 more)

### Community 15 - "Mango3S (MongoDB desktop IDE)"
Cohesion: 0.06
Nodes (38): app service (GUI, Linux X11), integration-tests service (test profile), mongo service (MongoDB 8.0, seeded), mongo-auth service (auth profile), docker/mongo-init seed scripts, /src/main.tsx entry script, index.html #root mount, Aggregation Editor (+30 more)

### Community 16 - "types.ts"
Cohesion: 0.04
Nodes (90): BulkWriteDialog(), Checks(), ConfirmInput(), EndpointPicker(), JobStatus(), Stat(), useJob(), useJobDone() (+82 more)

### Community 17 - "ShellView.tsx"
Cohesion: 0.09
Nodes (32): KIND_LABELS, needsCollection(), openAction(), timeAgo(), ctx, labels(), COLLECTION_METHODS, currentCollection() (+24 more)

### Community 18 - "schema.go"
Cohesion: 0.13
Nodes (22): Field, D, DateTime, lessPath(), NewAnalyzer(), ptr(), field(), Report (+14 more)

### Community 19 - "ExplainDialog.tsx"
Cohesion: 0.14
Nodes (22): Metric(), STAGE_HELP, stageClass(), StageNode(), elapsed(), jobProgress(), JobRow(), JobsIndicator() (+14 more)

### Community 20 - "compilerOptions"
Cohesion: 0.07
Nodes (28): compilerOptions, allowJs, allowSyntheticDefaultImports, esModuleInterop, forceConsistentCasingInFileNames, isolatedModules, jsx, lib (+20 more)

### Community 21 - "Is"
Cohesion: 0.31
Nodes (8): Is(), T, TestParseID(), TestWithID(), T, TestBuildClientOptions(), TestRequireWritable(), TestUnreachableServerIsClassified()

### Community 22 - "ConnectionManager.tsx"
Cohesion: 0.09
Nodes (29): blankProfile(), CONNECTION_TABS, ConnectionManager(), emptyAdvanced(), ERROR_TABS, FolderNode, folderTree(), fromProfile() (+21 more)

### Community 23 - "logging.go"
Cohesion: 0.14
Nodes (17): Attr, Handler, Context, LevelVar, Logger, isSecretKey(), NewRedactingHandler(), ParseLevel() (+9 more)

### Community 24 - "Sidebar.tsx"
Cohesion: 0.13
Nodes (20): base(), CollectionIcon(), CollectionKind, DatabaseIcon(), FolderIcon(), GearIcon(), ServerIcon(), TYPE_GLYPHS (+12 more)

### Community 25 - ".convertValues"
Cohesion: 0.04
Nodes (15): AIProviders, Entry, ImportOptions, MCPInfo, Options, Output, Page, Proposal (+7 more)

### Community 26 - "open"
Cohesion: 0.07
Nodes (39): Certificate, describe(), Client, Context, Credentials, Logger, RWMutex, via() (+31 more)

### Community 28 - ".cancel"
Cohesion: 0.17
Nodes (10): AppInfo, Lifecycle, opRegistry, runningOp, CancelFunc, Duration, callWithTimeout(), NewLifecycle() (+2 more)

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
Cohesion: 0.11
Nodes (13): Bool, CollectionInfo, CollectionStats, ConnectionStep, DatabaseInfo, History, call(), App (+5 more)

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
Cohesion: 0.05
Nodes (70): Clone(), CloneDoc(), Delete(), Exists(), Get(), D, index(), Parent() (+62 more)

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
Cohesion: 0.32
Nodes (10): D, Document, Service, Source, IDFilter(), IDsFilter(), New(), ParseID() (+2 more)

### Community 45 - "shortcuts.tsx"
Cohesion: 0.10
Nodes (24): SCOPE_LABEL, ShortcutsDialog(), detectPlatform(), formatShortcut(), isTextTarget(), Platform, ShortcutDef, SHORTCUTS (+16 more)

### Community 47 - "Wrap"
Cohesion: 0.11
Nodes (26): Addr, ClientConfig, HostKeyCallback, Wrap(), buildDialer(), classifySSH(), containsAny(), Client (+18 more)

### Community 48 - "App"
Cohesion: 0.09
Nodes (3): App, Task, TaskType

### Community 50 - "compare.go"
Cohesion: 0.05
Nodes (73): CollectionRow, DBOptions, DBResult, DocDiff, Endpoint, Entry, FieldChange, fingerprint (+65 more)

### Community 51 - "ai.go"
Cohesion: 0.06
Nodes (45): answer, Check, Claude, Context, FieldSummary, Proposal, Provider, Request (+37 more)

### Community 52 - "p2.go"
Cohesion: 0.11
Nodes (21): AIProviders, AIStatus, exportTask, maskCopyTask, MCPInfo, p2, syncTask, xformPlan (+13 more)

### Community 53 - "migrate.go"
Cohesion: 0.23
Nodes (24): contains(), copyTable(), embed(), Execute(), fieldFor(), Collection, Context, D (+16 more)

### Community 54 - "BuildClientOptions"
Cohesion: 0.17
Nodes (21): ClientOptions, applyAdvanced(), applyAuth(), applyTLS(), BuildClientOptions(), CheckTLSFiles(), AdvancedConfig, Profile (+13 more)

### Community 55 - "CollectionView.tsx"
Cohesion: 0.09
Nodes (50): AIDialog(), CodeDialog(), CodeEditor(), CodeEditorProps, HELPERS, OPERATORS, selectionOrAll(), theme (+42 more)

### Community 56 - "Compact"
Cohesion: 0.19
Nodes (25): Compact(), Equal(), TestParseRelaxedSyntax(), NewCSVReader(), NewJSONReader(), dataset(), D, T (+17 more)

### Community 57 - "dialect.go"
Cohesion: 0.22
Nodes (21): DB, Redact(), beginRead(), dialectFor(), Context, Rows, Tx, Introspect() (+13 more)

### Community 69 - "T"
Cohesion: 0.19
Nodes (14): Profile, TestBuildURI(), TestNormalizeDefaults(), TestParseURIRoundTrip(), TestProfileOrder(), TestProfileValidationNamesTheTab(), TestSecretFlags(), TestStoreKeepsNewSectionsAndLastConnected() (+6 more)

### Community 79 - "EventsOn"
Cohesion: 0.67
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 83 - "local.go"
Cohesion: 0.20
Nodes (12): httpStatus, Local, LocalServer, Error, asStatus(), checkURL(), DetectLocalServers(), doJSON() (+4 more)

### Community 84 - "server.go"
Cohesion: 0.07
Nodes (45): CallToolResult, ClientSession, In, call(), connectClient(), Policy, T, TestAuditLogOmitsArguments() (+37 more)

### Community 85 - "mql/format.go"
Cohesion: 0.20
Nodes (13): Buffer, CanonicalJSON(), escapeRegexSlashes(), Format(), formatTime(), D, Time, Lookup() (+5 more)

### Community 86 - "shortcuts.ts"
Cohesion: 0.13
Nodes (13): CODE_KEYS, Conflict, eventCombo(), eventKey(), Handler, KEY_LABELS, KeymapOverrides, MAC_SYMBOLS (+5 more)

### Community 87 - "providers_test.go"
Cohesion: 0.34
Nodes (12): argAfter(), fakeRunner(), T, hasArg(), TestClaudeCodeErrors(), TestClaudeCodeRunsWithoutToolsOrSettings(), TestCodexUsesReadOnlySandboxAndSchemaFile(), TestDetectClaudeCode() (+4 more)

### Community 88 - "record"
Cohesion: 0.40
Nodes (8): T, TestStepsAgainstRealServer(), T, record(), states(), TestStepsReportBadSRVName(), TestStepsShowWhereItFails(), TestStepsThroughSSHTunnel()

### Community 89 - "Parse"
Cohesion: 0.24
Nodes (13): CollectionRef(), D, New(), Parse(), parseField(), T, TestParse(), TestParseErrorsNameTheField() (+5 more)

### Community 90 - "csv.go"
Cohesion: 0.17
Nodes (13): FormatDouble(), FormatCell(), D, LeafPaths(), LookupPath(), NewCSVWriter(), setPath(), validType() (+5 more)

### Community 91 - "Invalid"
Cohesion: 0.18
Nodes (14): Code, Error, Check(), firstLine(), From(), Invalid(), New(), T (+6 more)

### Community 92 - "App"
Cohesion: 0.12
Nodes (5): App, FieldResult, FieldUpdate, FieldValue, ParsedURI

### Community 93 - "migrate_test.go"
Cohesion: 0.22
Nodes (14): clip(), ConvertValue(), SuggestType(), SuggestTypeFor(), toInt64(), T, SeedSQLite(), TestConvertValue() (+6 more)

### Community 94 - "Mango3S P2 reference"
Cohesion: 0.13
Nodes (14): Access control (deny by default), AI-assisted queries and pipelines, Compare and sync, Configuration files (in the config directory), Credentials and sensitive data, Data-integrity model, Mango3S P2 reference, MCP server (+6 more)

### Community 95 - "Store"
Cohesion: 0.24
Nodes (9): Duration, entry, Mutex, T, Time, New(), entry, Store (+1 more)

### Community 97 - "setup"
Cohesion: 0.15
Nodes (38): env, contains(), containsDB(), DatabaseInfo, T, nField(), seed(), setup() (+30 more)

### Community 98 - "AppContext.tsx"
Cohesion: 0.11
Nodes (31): newOpId(), onConnectionStep(), onJobUpdate(), setKeymapOverrides(), sameTarget(), SINGLETONS, tabId(), TabInit (+23 more)

### Community 99 - "ReadJSON"
Cohesion: 0.29
Nodes (8): ReadJSON(), Mutex, Source, Time, NewService(), Run, runsFile, Service

### Community 100 - "wait"
Cohesion: 0.45
Nodes (11): Job, Manager, T, newManager(), TestCancelRunningAndQueued(), TestIgnoredCancellationStillCancels(), TestJobFailureAndPanic(), TestJobSucceedsWithProgress() (+3 more)

### Community 101 - "TasksView.tsx"
Cohesion: 0.17
Nodes (13): app, backend, base, AIProposal, Task, TaskRun, TaskSchedule, TaskType (+5 more)

### Community 102 - "Profile"
Cohesion: 0.22
Nodes (4): SecretInput, Credentials, Profile, SplitCredentials()

### Community 104 - "newFixture"
Cohesion: 0.31
Nodes (13): Duration, Mutex, T, Time, newFixture(), TestApprovalGatesExecution(), TestCronParseAndNext(), TestNoOverlap() (+5 more)

### Community 105 - "New"
Cohesion: 0.25
Nodes (11): ClaudeCode, CLIInfo, Codex, cliError(), DetectClaudeCode(), DetectCodex(), emptyDir(), exeName() (+3 more)

### Community 106 - "appMenu"
Cohesion: 0.23
Nodes (10): App, BeforeClose(), HideApp(), MenuEvent(), QuitApp(), appMenu(), newApp(), runDesktop() (+2 more)

### Community 110 - ".backupDir"
Cohesion: 0.18
Nodes (5): Func, expandPath(), Time, NewMCPServer(), Server

### Community 111 - "codegen_test.go"
Cohesion: 0.34
Nodes (14): fakeProvider, aggSpec(), gen(), Spec, T, mustContain(), richSpec(), TestAggregateAllLanguages() (+6 more)

### Community 112 - "NewDocument"
Cohesion: 0.24
Nodes (9): ComputePaging(), D, NewDocument(), T, TestComputePaging(), TestNewDocument(), Document, Page (+1 more)

### Community 113 - "App.test.tsx"
Cohesion: 0.20
Nodes (9): backend, connectAndOpenOrders(), dismissConnectLog(), jobListeners, key(), menuListeners, profile, session (+1 more)

### Community 115 - "tools.go"
Cohesion: 0.27
Nodes (13): commonDirs(), Detect(), detectOne(), exe(), firstLine(), ToolsConfig, isFile(), locate() (+5 more)

### Community 117 - "config.go"
Cohesion: 0.33
Nodes (5): profilesFile, Settings, DefaultSettings(), maxPos(), normalizeKeymap()

### Community 118 - "KeymapSettings.tsx"
Cohesion: 0.30
Nodes (14): Keymap, KeymapSettings(), parseKeymapFile(), TEXT_EDITING, bindingFromEvent(), conflictsWith(), defaultKeysFor(), findConflicts() (+6 more)

### Community 119 - "docactions.test.tsx"
Cohesion: 0.19
Nodes (9): api(), cell(), doc, Extra, focusCell(), fv(), page, result() (+1 more)

### Community 120 - "App.tsx"
Cohesion: 0.10
Nodes (27): GlobalShortcuts(), Shell(), StatusBar(), TabBar(), TabErrorBoundary, AboutDialog(), Component, ConfirmDialog() (+19 more)

### Community 150 - "Profile"
Cohesion: 0.17
Nodes (15): AdvancedConfig, HostPort, ParsedURI, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig (+7 more)

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

### Community 171 - "profiles.go"
Cohesion: 0.36
Nodes (7): Capability, ConnectionStep, agentNote(), agentSupported(), goos(), SetStepListener(), Step

### Community 201 - ".ExecuteBulkWrite"
Cohesion: 0.36
Nodes (5): BulkPreview, BulkResult, BulkWriteRequest, D, orEmpty()

### Community 206 - "NewStore"
Cohesion: 0.39
Nodes (8): NewStore(), T, TestDefaultViewIsTreeUntilChosen(), TestKeymapNormalize(), TestSettingsNormalize(), TestSplitCredentials(), TestStoreProfilesCRUD(), TestStoreSettingsAndCorruption()

### Community 289 - "shell/parse.go"
Cohesion: 0.29
Nodes (13): describe(), Parse(), parseArgs(), parseChain(), parseStatement(), peekIdent(), T, TestHumanBytes() (+5 more)

### Community 290 - "ProfileInput"
Cohesion: 0.25
Nodes (7): ProfileInput, AdvancedConfig, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig

### Community 292 - ".Connect"
Cohesion: 0.38
Nodes (3): Reporter, SessionInfo, TestResult

### Community 293 - "docviews.test.tsx"
Cohesion: 0.20
Nodes (6): ada(), fields, grace, Harness(), page(), FieldValue

### Community 294 - "NewJSONWriter"
Cohesion: 0.24
Nodes (6): Decoder, D, NewJSONWriter(), truncate(), JSONReader, JSONWriter

### Community 295 - "keymap.test.tsx"
Cohesion: 0.29
Nodes (4): app, backend, rerender, setShortcutPlatform()

### Community 296 - ".UpdateField"
Cohesion: 0.14
Nodes (26): Decimal128, Document, FieldResult, FieldUpdate, FieldValue, service, checkParents(), describeField() (+18 more)

### Community 298 - "Deps"
Cohesion: 0.40
Nodes (5): Deps, LevelVar, Logger, Manager, Store

## Knowledge Gaps
- **257 isolated node(s):** `Welcome tab and workspace tabs`, `Connection Manager`, `Collection query editor and paging`, `Result views and field editing`, `Document and field actions (right-click menu)` (+252 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **205 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `NewID()` connect `Scheduler` to `Store`, `Writer`, `setup`, `Collection`, `.Apply`, `WriteJSON`, `.UpdateField`, `Manager`, `config.go`, `dialect.go`, `Store`?**
  _High betweenness centrality (0.069) - this node is a cross-community bridge._
- **Why does `Info` connect `Info` to `call`, `models.ts`, `Profile`, `App`, `.aiProvider`, `App`?**
  _High betweenness centrality (0.060) - this node is a cross-community bridge._
- **Why does `Run` connect `.convertValues` to `models.ts`, `.backupDir`?**
  _High betweenness centrality (0.054) - this node is a cross-community bridge._
- **Are the 35 inferred relationships involving `Compact()` (e.g. with `.Insert()` and `TestBuild()`) actually correct?**
  _`Compact()` has 35 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Welcome tab and workspace tabs`, `Connection Manager`, `Collection query editor and paging` to the rest of the system?**
  _257 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `transfer.go` be split into smaller, more focused modules?**
  _Cohesion score 0.1361344537815126 - nodes in this community are weakly interconnected._
- **Should `parser` be split into smaller, more focused modules?**
  _Cohesion score 0.08128772635814889 - nodes in this community are weakly interconnected._