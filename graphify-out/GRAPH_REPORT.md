# Graph Report - Mango3S  (2026-10-19)

## Corpus Check
- 237 files · ~270,040 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 3973 nodes · 8542 edges · 364 communities (134 shown, 230 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 755 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `693024a4`
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
- NewDocument
- DocViews.tsx
- Manager
- Analyze
- Mango3S (MongoDB desktop IDE)
- types.ts
- ShellView.tsx
- schema.go
- manage/manage.go
- compilerOptions
- TransformView.tsx
- ConnectionManager.tsx
- logging.go
- Sidebar.tsx
- .convertValues
- Session
- Manager
- .cancel
- dependencies
- devDependencies
- runtime/package.json
- call
- Open
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
- sqlexport.go
- shortcuts.tsx
- Apply
- dial_test.go
- App
- App
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
- .register
- mql/format.go
- shortcuts.ts
- providers_test.go
- New
- Source
- csv.go
- Invalid
- App
- migrate_test.go
- Mango3S P2 reference
- Store
- third-party-notices.sh
- p2_test.go
- AppContext.tsx
- ReadJSON
- wait
- restore.go
- Profile
- Info
- open
- cli.go
- profiles.go
- CodeRequest
- Run
- BackupDialogs.tsx
- .backupDir
- codegen_test.go
- ParseValue
- App.test.tsx
- Outcome
- detectOne
- .aiProvider
- config.go
- KeymapSettings.tsx
- ResultsView.tsx
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
- dump_integration_test.go
- TestFieldLevelEditing
- Manager
- api/connio.go
- DBOptions
- WaitGroup
- Document
- .previewTransform
- Apply
- profiles_test.go
- dump/format.go
- connio/connio.go
- Field
- ParsedURI
- Plan
- dump/tools.go
- setup
- Profile
- ProfileInput
- denied
- TransformPreview
- Service
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
- server.go
- App
- LevelVar
- Profile
- dump_test.go
- Request
- mcpserver_test.go
- Service
- D
- Store
- A
- Context
- Service
- Context
- Manager
- Collection
- Context
- newFixture
- Buffer
- Document
- .registerChainTask
- LocalServer
- DBResult
- AdvancedConfig
- ai_test.go
- Parse
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
- .StartRestoreBackup
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
- privileges.go
- .Open
- RWMutex
- copyOne
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
- WelcomeView.tsx
- .Connect
- docviews.test.tsx
- export
- keymap.test.tsx
- NewID
- Profile
- Logger
- .Generate
- SessionInfo
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
- fixture
- compare_test.go
- CopyRequest
- TestBuildPlan
- AIProviders
- CopyRequest
- Entry
- ImportOptions
- ImportPreview
- Manifest
- MCPInfo
- ProfileEntry
- Proposal
- Report
- RestoreOptions
- RestoreRequest
- RuleSet
- Run
- SQLExportRequest
- Stage
- TableMapping
- TestLiveProviders
- tlog
- CollectionDetails
- ConnectionExport
- CopyOptions
- CreateOptions
- DatabaseDetails
- ImportChoice
- ImportItem
- ImportOutcome
- ImportResult
- Index
- IndexChange
- IndexOptions
- Namespace
- ProfileQuery
- Profiling
- Rename
- Selection
- ToolDump
- ToolRestore
- Validation
- ValidationReport
- CancelFunc
- ConnectionStep
- Context
- Manager
- Context
- TestResult
- Context
- ToolsConfig
- Manager

## God Nodes (most connected - your core abstractions)
1. `App` - 58 edges
2. `App` - 51 edges
3. `useApp()` - 42 edges
4. `call()` - 41 edges
5. `Scheduler` - 39 edges
6. `toAppError()` - 37 edges
7. `Compact()` - 37 edges
8. `Manager` - 35 edges
9. `App` - 34 edges
10. `Backend` - 32 edges

## Surprising Connections (you probably didn't know these)
- `newApp()` --calls--> `New()`  [INFERRED]
  main.go → internal/api/api.go
- `runDesktop()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go
- `runMCP()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go
- `TreeView()` --indirect_call--> `key()`  [INFERRED]
  frontend/src/components/DocViews.tsx → frontend/src/App.test.tsx
- `dumpOne()` --calls--> `ext()`  [INFERRED]
  internal/dump/dump.go → internal/dump/format.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Editors sharing the mql value language** — readme_query_editor, readme_aggregation_editor, readme_intellishell, readme_visual_query_builder, readme_mql_shared_value_language [EXTRACTED 1.00]
- **P1 features reusing P0 query/aggregate services** — readme_sql_subset, readme_code_generation, readme_explain_plans, readme_import_export_wizard, readme_p1_reuses_p0 [EXTRACTED 1.00]
- **Docker compose dev/test stack** — docker_compose_mongo, docker_compose_mongo_auth, docker_compose_integration_tests, docker_compose_app, docker_compose_mongo_init [EXTRACTED 1.00]

## Communities (364 total, 230 thin omitted)

### Community 0 - "transfer.go"
Cohesion: 0.13
Nodes (31): ColumnSpec, ParseDelimiter(), DetectFormat(), exportQuery(), fieldsProjection(), firstLine(), Context, D (+23 more)

### Community 1 - "Store"
Cohesion: 0.36
Nodes (4): Store, sortProfiles(), New(), Mutex

### Community 2 - "Collection"
Cohesion: 0.29
Nodes (18): allDocs(), byID(), Context, D, Job, Manager, T, representative() (+10 more)

### Community 3 - "parser"
Cohesion: 0.08
Nodes (43): mqlCompact(), errAt(), lex(), isLiteralFunc(), parse(), compact(), T, TestErrorPosition() (+35 more)

### Community 5 - "Parser"
Cohesion: 0.14
Nodes (12): construct(), Time, isIdentPart(), isIdentStart(), narrowInt(), NewParser(), ParseDate(), regexOptions() (+4 more)

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

### Community 11 - "NewDocument"
Cohesion: 0.11
Nodes (27): Request, Service, Stage, Build(), FromText(), Context, Page, indentLines() (+19 more)

### Community 12 - "DocViews.tsx"
Cohesion: 0.06
Nodes (69): CellEditing, CellEditor(), CellRef, EditState, FieldApi, ResultsNotice, sameRef(), scrollIntoView() (+61 more)

### Community 13 - "Manager"
Cohesion: 0.12
Nodes (13): CancelFunc, Duration, Logger, Mutex, Time, WaitGroup, NewManager(), entry (+5 more)

### Community 14 - "Analyze"
Cohesion: 0.14
Nodes (31): Detail, Diagnostic, m, Plan, Request, Stage, Summary, Analyze() (+23 more)

### Community 15 - "Mango3S (MongoDB desktop IDE)"
Cohesion: 0.06
Nodes (38): app service (GUI, Linux X11), integration-tests service (test profile), mongo service (MongoDB 8.0, seeded), mongo-auth service (auth profile), docker/mongo-init seed scripts, /src/main.tsx entry script, index.html #root mount, Aggregation Editor (+30 more)

### Community 16 - "types.ts"
Cohesion: 0.03
Nodes (117): AIDialog(), app, backend, conn, addIndexKey(), CollectionInfoDialog(), CopyDialog(), CreateCollectionDialog() (+109 more)

### Community 17 - "ShellView.tsx"
Cohesion: 0.09
Nodes (32): KIND_LABELS, needsCollection(), openAction(), timeAgo(), ctx, labels(), COLLECTION_METHODS, currentCollection() (+24 more)

### Community 18 - "schema.go"
Cohesion: 0.12
Nodes (24): Field, D, DateTime, lessPath(), New(), NewAnalyzer(), ptr(), field() (+16 more)

### Community 19 - "manage/manage.go"
Cohesion: 0.09
Nodes (53): FindOptionsBuilder, CheckValidator(), ClearCollection(), CollectionOptions(), CreateCollection(), CreateCommand(), CreateIndex(), CreateView() (+45 more)

### Community 20 - "compilerOptions"
Cohesion: 0.07
Nodes (28): compilerOptions, allowJs, allowSyntheticDefaultImports, esModuleInterop, forceConsistentCasingInFileNames, isolatedModules, jsx, lib (+20 more)

### Community 21 - "TransformView.tsx"
Cohesion: 0.08
Nodes (34): Checks(), ConfirmInput(), EndpointPicker(), JobStatus(), Stat(), useJob(), useJobDone(), CompareResult (+26 more)

### Community 22 - "ConnectionManager.tsx"
Cohesion: 0.07
Nodes (35): ConnectionExportDialog(), ConnectionImportDialog(), defaultAction(), blankProfile(), CONNECTION_TABS, ConnectionManager(), emptyAdvanced(), ERROR_TABS (+27 more)

### Community 23 - "logging.go"
Cohesion: 0.14
Nodes (17): Attr, Handler, Context, LevelVar, Logger, isSecretKey(), NewRedactingHandler(), ParseLevel() (+9 more)

### Community 24 - "Sidebar.tsx"
Cohesion: 0.10
Nodes (25): base(), CollectionIcon(), CollectionKind, DatabaseIcon(), FolderIcon(), GearIcon(), ServerIcon(), TYPE_GLYPHS (+17 more)

### Community 25 - ".convertValues"
Cohesion: 0.03
Nodes (19): BackupRequest, BackupTools, CollectionStats, Context, DBResult, DocDiff, FieldResult, Inspection (+11 more)

### Community 26 - "Session"
Cohesion: 0.20
Nodes (10): describe(), Client, Context, Credentials, RWMutex, Context, Manager, Session (+2 more)

### Community 28 - ".cancel"
Cohesion: 0.16
Nodes (11): AppInfo, Lifecycle, opRegistry, runningOp, callWithTimeout(), CancelFunc, Duration, Mutex (+3 more)

### Community 29 - "dependencies"
Cohesion: 0.11
Nodes (19): codemirror, @codemirror/autocomplete, @codemirror/commands, @codemirror/lang-javascript, @codemirror/lang-sql, @codemirror/language, @codemirror/state, @codemirror/view (+11 more)

### Community 30 - "devDependencies"
Cohesion: 0.11
Nodes (19): devDependencies, jsdom, @testing-library/dom, @testing-library/react, @types/react, @types/react-dom, typescript, vite (+11 more)

### Community 31 - "runtime/package.json"
Cohesion: 0.11
Nodes (18): author, bugs, url, description, homepage, keywords, license, main (+10 more)

### Community 32 - "call"
Cohesion: 0.09
Nodes (7): CollectionInfo, CollectionStats, CreateOptions, DatabaseInfo, call(), Page, ValidationReport

### Community 33 - "Open"
Cohesion: 0.11
Nodes (16): AEAD, Mutex, loadOrCreateKey(), NewFileStore(), NewMemoryStore(), Open(), probeKeyring(), T (+8 more)

### Community 34 - "Writer"
Cohesion: 0.13
Nodes (23): Header, Info, line, ManyResult, RestoreStats, Writer, File, canonical() (+15 more)

### Community 35 - "models.ts"
Cohesion: 0.12
Nodes (30): aggregate, ai, api, backup, codegen, compare, config, documents (+22 more)

### Community 36 - "wails.json"
Cohesion: 0.12
Nodes (15): author, email, name, frontend:build, frontend:dev:serverUrl, frontend:dev:watcher, frontend:install, info (+7 more)

### Community 37 - ".Apply"
Cohesion: 0.05
Nodes (67): Clone(), CloneDoc(), Delete(), Exists(), Get(), D, index(), Parent() (+59 more)

### Community 38 - "compilerOptions"
Cohesion: 0.22
Nodes (8): compilerOptions, allowSyntheticDefaultImports, composite, module, moduleResolution, skipLibCheck, include, vite.config.ts

### Community 39 - "runtime.d.ts"
Cohesion: 0.25
Nodes (7): EnvironmentInfo, NotificationAction, NotificationCategory, NotificationOptions, Position, Screen, Size

### Community 40 - "Scheduler"
Cohesion: 0.08
Nodes (36): T, TestConnectionExportImportRoundTrip(), CancelFunc, Duration, Logger, Mutex, RawMessage, Time (+28 more)

### Community 41 - "scripts"
Cohesion: 0.29
Nodes (7): scripts, build, dev, preview, test, test:watch, typecheck

### Community 42 - "01-sample-data.js"
Cohesion: 0.33
Nodes (5): customers, orders, products, statuses, store

### Community 43 - "frontend/package.json"
Cohesion: 0.40
Nodes (4): name, private, type, version

### Community 44 - "sqlexport.go"
Cohesion: 0.13
Nodes (24): Column, appendOnce(), collect(), Export(), extJSON(), Collection, D, Raw (+16 more)

### Community 45 - "shortcuts.tsx"
Cohesion: 0.11
Nodes (24): detectPlatform(), eventCombo(), eventKey(), isTextTarget(), matchIndex(), Platform, SHORTCUT_BY_ID, ShortcutDef (+16 more)

### Community 46 - "Apply"
Cohesion: 0.12
Nodes (25): Counts, DocDiff, Apply(), BuildPlan(), bulk(), Collection, Context, D (+17 more)

### Community 47 - "dial_test.go"
Cohesion: 0.07
Nodes (58): Addr, ClientConfig, HostKeyCallback, Int32, buildDialer(), classifySSH(), containsAny(), Client (+50 more)

### Community 48 - "App"
Cohesion: 0.08
Nodes (5): call, App, Source, Task, TaskType

### Community 49 - "App"
Cohesion: 0.12
Nodes (11): CollectionDetails, DatabaseDetails, Index, IndexOptions, App, Job, Session, ProfileEntry (+3 more)

### Community 50 - "compare.go"
Cohesion: 0.15
Nodes (28): DocDiff, Endpoint, Entry, FieldChange, fingerprint, Hash, Options, Result (+20 more)

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

### Community 55 - "CollectionView.tsx"
Cohesion: 0.10
Nodes (55): BulkWriteDialog(), CodeDialog(), CodeEditor(), CodeEditorProps, HELPERS, OPERATORS, selectionOrAll(), theme (+47 more)

### Community 56 - "Compact"
Cohesion: 0.13
Nodes (29): Decoder, Compact(), NewCSVReader(), D, NewJSONReader(), NewJSONWriter(), truncate(), dataset() (+21 more)

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

### Community 84 - ".register"
Cohesion: 0.21
Nodes (12): CheckValue(), Context, D, Logger, Policy, Profile, RawMessage, New() (+4 more)

### Community 85 - "mql/format.go"
Cohesion: 0.20
Nodes (14): Buffer, CanonicalJSON(), escapeRegexSlashes(), Format(), FormatDouble(), formatTime(), D, Time (+6 more)

### Community 86 - "shortcuts.ts"
Cohesion: 0.14
Nodes (10): CODE_KEYS, Conflict, Handler, KEY_LABELS, KeymapOverrides, MAC_SYMBOLS, Mods, overrides (+2 more)

### Community 87 - "providers_test.go"
Cohesion: 0.34
Nodes (12): argAfter(), fakeRunner(), T, hasArg(), TestClaudeCodeErrors(), TestClaudeCodeRunsWithoutToolsOrSettings(), TestCodexUsesReadOnlySandboxAndSchemaFile(), TestDetectClaudeCode() (+4 more)

### Community 88 - "New"
Cohesion: 0.31
Nodes (11): New(), Logger, NewManager(), T, TestStepsAgainstRealServer(), T, record(), states() (+3 more)

### Community 89 - "Source"
Cohesion: 0.17
Nodes (18): Service, New(), CollectionRef(), Context, D, Page, New(), Parse() (+10 more)

### Community 90 - "csv.go"
Cohesion: 0.17
Nodes (13): ConvertCell(), D, InferType(), LeafPaths(), LookupPath(), NewCSVWriter(), setPath(), validType() (+5 more)

### Community 91 - "Invalid"
Cohesion: 0.11
Nodes (32): Code, Error, Check(), firstLine(), From(), Invalid(), Is(), New() (+24 more)

### Community 92 - "App"
Cohesion: 0.12
Nodes (5): App, FieldResult, FieldUpdate, FieldValue, ParsedURI

### Community 93 - "migrate_test.go"
Cohesion: 0.22
Nodes (14): clip(), ConvertValue(), SuggestType(), SuggestTypeFor(), toInt64(), T, SeedSQLite(), TestConvertValue() (+6 more)

### Community 94 - "Mango3S P2 reference"
Cohesion: 0.06
Nodes (29): Advanced features: matrix, behavior and limits, Feature matrix, Limits, Logical exports vs full backups, Safety, Access control (deny by default), AI-assisted queries and pipelines, Compare and sync (+21 more)

### Community 95 - "Store"
Cohesion: 0.24
Nodes (9): Duration, entry, Mutex, T, Time, New(), entry, Store (+1 more)

### Community 97 - "p2_test.go"
Cohesion: 0.25
Nodes (22): env, coll(), App, Source, T, mongoURI(), newApp(), runMigration() (+14 more)

### Community 98 - "AppContext.tsx"
Cohesion: 0.11
Nodes (31): FolderNode, onConnectionStep(), onJobUpdate(), setKeymapOverrides(), sameTarget(), SINGLETONS, tabId(), TabInit (+23 more)

### Community 99 - "ReadJSON"
Cohesion: 0.29
Nodes (8): ReadJSON(), Mutex, Source, Time, NewService(), Run, runsFile, Service

### Community 100 - "wait"
Cohesion: 0.45
Nodes (11): Job, T, newManager(), TestCancelRunningAndQueued(), TestIgnoredCancellationStillCancels(), TestJobFailureAndPanic(), TestJobLog(), TestJobSucceedsWithProgress() (+3 more)

### Community 101 - "restore.go"
Cohesion: 0.18
Nodes (23): Entry, Inspection, Rename, RestoreOptions, RestoreReport, Result, collectionExists(), Client (+15 more)

### Community 102 - "Profile"
Cohesion: 0.20
Nodes (5): SecretInput, Credentials, Profile, SplitCredentials(), Session

### Community 104 - "open"
Cohesion: 0.17
Nodes (17): Certificate, certSummary(), describeProfile(), Client, Credentials, Duration, Profile, minDur() (+9 more)

### Community 105 - "cli.go"
Cohesion: 0.28
Nodes (10): ClaudeCode, CLIInfo, Codex, cliError(), DetectClaudeCode(), DetectCodex(), emptyDir(), exeName() (+2 more)

### Community 106 - "profiles.go"
Cohesion: 0.16
Nodes (16): Capability, ConnectionStep, agentNote(), agentSupported(), BeforeClose(), goos(), HideApp(), MenuEvent() (+8 more)

### Community 108 - "Run"
Cohesion: 0.21
Nodes (20): Binary, collInfo, Logger, Options, Selection, collectionInfo(), dumpOne(), glob() (+12 more)

### Community 109 - "BackupDialogs.tsx"
Cohesion: 0.13
Nodes (15): app, backend, conn, inspection, BackupDialog(), joinPath(), RestoreDialog(), safeName() (+7 more)

### Community 110 - ".backupDir"
Cohesion: 0.18
Nodes (5): Func, expandPath(), Time, NewMCPServer(), Server

### Community 111 - "codegen_test.go"
Cohesion: 0.34
Nodes (14): fakeProvider, aggSpec(), gen(), Spec, T, mustContain(), richSpec(), TestAggregateAllLanguages() (+6 more)

### Community 112 - "ParseValue"
Cohesion: 0.20
Nodes (20): Equal(), TypeName(), T, mustDec(), mustHex(), mustOID(), TestErrorPosition(), TestFormatShape() (+12 more)

### Community 113 - "App.test.tsx"
Cohesion: 0.14
Nodes (10): TabErrorBoundary, backend, connectAndOpenOrders(), dismissConnectLog(), jobListeners, key(), menuListeners, profile (+2 more)

### Community 115 - "detectOne"
Cohesion: 0.28
Nodes (13): Detect(), detectOne(), exe(), firstLine(), isFile(), locate(), majorMinor(), T (+5 more)

### Community 116 - ".aiProvider"
Cohesion: 0.33
Nodes (3): Duration, SetAIProvider(), Provider

### Community 117 - "config.go"
Cohesion: 0.19
Nodes (13): profilesFile, Settings, DefaultSettings(), maxPos(), NewStore(), normalizeKeymap(), T, TestDefaultViewIsTreeUntilChosen() (+5 more)

### Community 118 - "KeymapSettings.tsx"
Cohesion: 0.32
Nodes (13): Keymap, KeymapSettings(), parseKeymapFile(), TEXT_EDITING, bindingFromEvent(), conflictsWith(), defaultKeysFor(), findConflicts() (+5 more)

### Community 119 - "ResultsView.tsx"
Cohesion: 0.08
Nodes (30): TabBar(), page, api(), cell(), doc, Extra, focusCell(), fv() (+22 more)

### Community 120 - "App.tsx"
Cohesion: 0.12
Nodes (17): GlobalShortcuts(), AboutDialog(), Component, ConfirmDialog(), PromptDialog(), Toasts(), ConnectionLogDialog(), logText() (+9 more)

### Community 150 - "Profile"
Cohesion: 0.17
Nodes (15): AdvancedConfig, HostPort, ParsedURI, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig (+7 more)

### Community 152 - "dump_integration_test.go"
Cohesion: 0.27
Nodes (16): tlog, client(), Client, Collection, D, Raw, T, indexNames() (+8 more)

### Community 153 - "TestFieldLevelEditing"
Cohesion: 0.36
Nodes (8): D, T, Time, mustTime(), rawDoc(), TestFieldLevelEditing(), TestSystemCollectionsAreListedSeparately(), uriFromEnv()

### Community 155 - "api/connio.go"
Cohesion: 0.17
Nodes (12): ConnectionExport, ImportChoice, ImportItem, ImportOutcome, ImportPreview, App, Profile, hostsSummary() (+4 more)

### Community 160 - "Apply"
Cohesion: 0.26
Nodes (16): Check, Apply(), Collection, Context, D, Source, idOf(), RunPreview() (+8 more)

### Community 161 - "profiles_test.go"
Cohesion: 0.57
Nodes (6): fullInput(), T, TestDuplicateAndDeleteProfile(), TestParseConnectionStringNeverReturnsThePassword(), TestProfileValidationErrorsReachTheUI(), TestSaveProfileAllTabsAndSecrets()

### Community 162 - "dump/format.go"
Cohesion: 0.13
Nodes (15): closers, fileWriter, Manifest, Metadata, Namespace, TestEscapeNames(), escapeName(), ext() (+7 more)

### Community 163 - "connio/connio.go"
Cohesion: 0.22
Nodes (17): Candidate, compassConn, Parsed, checkURI(), fromURI(), RawMessage, hasCompassShape(), hostsOf() (+9 more)

### Community 167 - "dump/tools.go"
Cohesion: 0.27
Nodes (14): ToolArgs, ToolDump, ToolRestore, ToolSummary, CanUseTools(), connArgs(), DumpArgs(), Credentials (+6 more)

### Community 168 - "setup"
Cohesion: 0.31
Nodes (15): contains(), containsDB(), DatabaseInfo, T, nField(), seed(), setup(), TestAggregation() (+7 more)

### Community 171 - "denied"
Cohesion: 0.24
Nodes (8): contains(), denied(), Mutex, NewStore(), toolInfo(), Policy, Store, ToolInfo

### Community 173 - "Service"
Cohesion: 0.20
Nodes (8): Service, WritePreview, WriteResult, D, Provider, MaskShape(), NewService(), Rand

### Community 189 - "server.go"
Cohesion: 0.24
Nodes (13): CallToolResult, In, add(), errorResult(), textResult(), aggIn, collIn, connIn (+5 more)

### Community 190 - "App"
Cohesion: 0.11
Nodes (15): Deps, Bool, History, App, Logger, Report, Result, Session (+7 more)

### Community 193 - "dump_test.go"
Cohesion: 0.23
Nodes (12): docReader, drain(), T, TestDocReader(), TestGlobAndSelection(), TestInspectMissingMetadataWarns(), TestPrepareDirRefusesNonEmpty(), TestRename() (+4 more)

### Community 195 - "mcpserver_test.go"
Cohesion: 0.33
Nodes (12): ClientSession, call(), connectClient(), Policy, T, TestAuditLogOmitsArguments(), TestBoundaryEnforcementBeforeConnecting(), TestCheckValueRefusesJavaScriptAndWrites() (+4 more)

### Community 201 - "Service"
Cohesion: 0.26
Nodes (9): CollectionRow, DBOptions, DBResult, Service, Job, Mutex, Source, New() (+1 more)

### Community 206 - "newFixture"
Cohesion: 0.46
Nodes (12): decodeErr(), T, newFixture(), TestCancelOperation(), TestConnectFailureIsReadable(), TestFormatDocument(), TestMissingSecretIsReported(), TestOperationsRequireConnection() (+4 more)

### Community 209 - ".registerChainTask"
Cohesion: 0.21
Nodes (8): backupTask, chainStep, chainTask, ImportResult, App, Options, RawMessage, stepName()

### Community 213 - "ai_test.go"
Cohesion: 0.32
Nodes (11): AnswerSchema(), answerJSON(), checkNames(), contains(), Proposal, T, TestContextNeverLeaksValues(), TestSystemPromptHasNoConnectionDetails() (+3 more)

### Community 214 - "Parse"
Cohesion: 0.38
Nodes (10): Parse(), Profile, T, profiles(), TestExportedSecretsNeedThePassphrase(), TestExportWithoutSecretsHasNoSecrets(), TestParseCompassExport(), TestParseRejectsUnknownFiles() (+2 more)

### Community 250 - ".StartRestoreBackup"
Cohesion: 0.14
Nodes (16): BackupRequest, BackupTools, RestoreRequest, Info, Inspection, App, Credentials, Job (+8 more)

### Community 275 - "privileges.go"
Cohesion: 0.31
Nodes (9): contains(), D, granted(), MissingActions(), T, priv(), TestMissingActions(), connStatus (+1 more)

### Community 276 - ".Open"
Cohesion: 0.33
Nodes (9): File, Sealed, Secrets, deriveKey(), Export(), Profile, Time, newGCM() (+1 more)

### Community 278 - "copyOne"
Cohesion: 0.36
Nodes (9): CopyOptions, CopyReport, Copy(), copyOne(), Client, D, Logger, Namespace (+1 more)

### Community 289 - "shell/parse.go"
Cohesion: 0.29
Nodes (13): describe(), Parse(), parseArgs(), parseChain(), parseStatement(), peekIdent(), T, TestHumanBytes() (+5 more)

### Community 290 - "ProfileInput"
Cohesion: 0.20
Nodes (8): ProfileInput, AdvancedConfig, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig, ToolsConfig

### Community 291 - "WelcomeView.tsx"
Cohesion: 0.31
Nodes (8): SCOPE_LABEL, ShortcutsDialog(), formatBinding(), formatShortcut(), SHORTCUTS, getShortcutPlatform(), TIPS, WelcomeView()

### Community 292 - ".Connect"
Cohesion: 0.31
Nodes (3): Reporter, SessionInfo, TestResult

### Community 293 - "docviews.test.tsx"
Cohesion: 0.22
Nodes (5): ada(), fields, grace, Harness(), page()

### Community 294 - "export"
Cohesion: 0.53
Nodes (9): check(), export(), Collection, Report, T, seed(), TestMySQLRoundTrip(), TestPostgresRoundTrip() (+1 more)

### Community 295 - "keymap.test.tsx"
Cohesion: 0.29
Nodes (4): app, backend, rerender, setShortcutPlatform()

### Community 296 - "NewID"
Cohesion: 0.47
Nodes (5): NewID(), T, queryReq(), TestRenameAndAppendField(), TestUpdateAndRemoveDialogs()

### Community 299 - ".Generate"
Cohesion: 0.31
Nodes (7): Claude, Usage, Client, classify(), Context, NewClaude(), supportsDefaultFallback()

### Community 311 - "fixture"
Cohesion: 0.31
Nodes (8): fixture, App, Buffer, Job, T, TestMultiStepTaskAndTaskImport(), TestToolBackupWithAuthentication(), waitJob()

### Community 312 - "compare_test.go"
Cohesion: 0.52
Nodes (6): doc(), D, T, TestDiff(), TestFingerprintOrderIndependent(), TestNormalizeAndHash()

### Community 313 - "CopyRequest"
Cohesion: 0.33
Nodes (5): CopyRequest, IndexChange, SQLExportRequest, CopyOptions, Options

### Community 314 - "TestBuildPlan"
Cohesion: 0.47
Nodes (5): Result, T, result(), TestBuildPlan(), TestPlanStoreSingleUseAndExpiry()

### Community 332 - "TestLiveProviders"
Cohesion: 0.50
Nodes (3): T, TestLiveProviders(), ExtractJSON()

## Knowledge Gaps
- **286 isolated node(s):** `Feature matrix`, `Logical exports vs full backups`, `Safety`, `Limits`, `Welcome tab and workspace tabs` (+281 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **230 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Info` connect `Info` to `models.ts`, `Profile`, `App`, `.aiProvider`, `App`?**
  _High betweenness centrality (0.091) - this node is a cross-community bridge._
- **Why does `App` connect `App` to `call`, `.Connect`, `Manager`, `.aiProvider`, `p2.go`, `New`, `.cancel`?**
  _High betweenness centrality (0.086) - this node is a cross-community bridge._
- **Why does `Manager` connect `Manager` to `ReadJSON`, `wait`, `Scheduler`, `Service`, `Apply`, `App`?**
  _High betweenness centrality (0.052) - this node is a cross-community bridge._
- **Are the 20 inferred relationships involving `call()` (e.g. with `.AnalyzeSchema()` and `.CheckValidator()`) actually correct?**
  _`call()` has 20 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Feature matrix`, `Logical exports vs full backups`, `Safety` to the rest of the system?**
  _286 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `transfer.go` be split into smaller, more focused modules?**
  _Cohesion score 0.13333333333333333 - nodes in this community are weakly interconnected._
- **Should `parser` be split into smaller, more focused modules?**
  _Cohesion score 0.07800608828006088 - nodes in this community are weakly interconnected._