# Graph Report - Mango3S  (2026-10-19)

## Corpus Check
- 240 files · ~278,762 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 4049 nodes · 8695 edges · 369 communities (130 shown, 239 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 744 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `c0b0472a`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- transfer.go
- Profile
- .UpdateField
- parser
- Parser
- WriteJSON
- .evalColl
- codegen.go
- .RunSQL
- Source
- DocViews.tsx
- Manager
- Analyze
- Mango3S (MongoDB desktop IDE)
- types.ts
- ShellView.tsx
- schema.go
- manage/manage.go
- compilerOptions
- ManageDialogs.tsx
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
- CreateOptions
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
- sqlexport.go
- shortcuts.tsx
- Apply
- dial_test.go
- App
- .writableSession
- compare.go
- ai.go
- p2.go
- migrate.go
- BuildClientOptions
- CollectionView.tsx
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
- .Find
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
- restore.go
- ProfileInput
- Info
- advisor.go
- cli.go
- profiles.go
- CodeRequest
- Run
- BackupDialogs.tsx
- .backupDir
- codegen_test.go
- Compact
- App.test.tsx
- Outcome
- detectOne
- .aiProvider
- NewStore
- KeymapSettings.tsx
- docactions.test.tsx
- dial.go
- Task
- TestShellResultsArePaged
- App
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
- dump_integration_test.go
- ParseValue
- Manager
- api/connio.go
- DBOptions
- WaitGroup
- Document
- .previewTransform
- .Replace
- profiles_test.go
- dump/format.go
- connio/connio.go
- Field
- ParsedURI
- Plan
- dump/tools.go
- NewAnalyzer
- Profile
- Writer
- denied
- TransformPreview
- .Connect
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
- .ExecuteBulkWrite
- Buffer
- Document
- .registerChainTask
- LocalServer
- DBResult
- AdvancedConfig
- TestEditTextRoundTrips
- Parse
- Session
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
- NewDocument
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
- NewJSONWriter
- p2.test.tsx
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
- WelcomeView.tsx
- docviews.test.tsx
- .Open
- NewID
- Logger
- call
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
- HashDoc
- .StartCopy
- TestBuildPlan
- CopyRequest
- Entry
- Manifest
- Preview
- Advice
- Proposal
- Report
- RestoreOptions
- RestoreRequest
- BackupTools
- Run
- Context
- DocDiff
- TableMapping
- FieldResult
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
- Item
- Job
- Options
- Output
- QueryShape
- Request
- Result
- AdviceOptions
- IndexFinding
- Index
- Job
- Options
- Session
- Client

## God Nodes (most connected - your core abstractions)
1. `App` - 58 edges
2. `App` - 56 edges
3. `App` - 51 edges
4. `useApp()` - 42 edges
5. `Scheduler` - 39 edges
6. `toAppError()` - 37 edges
7. `Compact()` - 37 edges
8. `Manager` - 35 edges
9. `Backend` - 32 edges
10. `Spinner()` - 31 edges

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

## Communities (369 total, 239 thin omitted)

### Community 0 - "transfer.go"
Cohesion: 0.12
Nodes (31): ColumnSpec, ParseDelimiter(), DetectFormat(), exportQuery(), fieldsProjection(), firstLine(), Context, D (+23 more)

### Community 1 - "Profile"
Cohesion: 0.23
Nodes (9): profilesFile, Settings, Store, DefaultSettings(), Profile, maxPos(), normalizeKeymap(), sortProfiles() (+1 more)

### Community 2 - ".UpdateField"
Cohesion: 0.24
Nodes (16): Document, FieldResult, FieldUpdate, FieldValue, service, checkParents(), describeField(), EditText() (+8 more)

### Community 3 - "parser"
Cohesion: 0.08
Nodes (42): errAt(), lex(), isLiteralFunc(), parse(), compact(), T, TestErrorPosition(), TestLikeToRegex() (+34 more)

### Community 5 - "Parser"
Cohesion: 0.17
Nodes (7): construct(), Time, isIdentPart(), isIdentStart(), ParseDate(), regexOptions(), Parser

### Community 6 - "WriteJSON"
Cohesion: 0.06
Nodes (34): WriteJSON(), Mutex, Stage, Time, NewHistory(), NewStore(), sameRun(), T (+26 more)

### Community 7 - ".evalColl"
Cohesion: 0.07
Nodes (51): A, Collection, Cursor, D, Database, CollectionInfo, CollectionStats, DatabaseInfo (+43 more)

### Community 8 - "codegen.go"
Cohesion: 0.18
Nodes (35): Language, renderer, Spec, b64(), block(), csRenderer(), escapeRegex(), floatLit() (+27 more)

### Community 9 - ".RunSQL"
Cohesion: 0.15
Nodes (14): CodeRequest, SQLRequest, SQLResult, SQLTranslation, codeSpec(), errorMessage(), findRequest(), D (+6 more)

### Community 11 - "Source"
Cohesion: 0.18
Nodes (19): Request, Service, Stage, Build(), FromText(), Context, Page, indentLines() (+11 more)

### Community 12 - "DocViews.tsx"
Cohesion: 0.06
Nodes (70): CellEditing, CellEditor(), CellRef, EditState, FieldApi, ResultsNotice, sameRef(), scrollIntoView() (+62 more)

### Community 13 - "Manager"
Cohesion: 0.13
Nodes (11): CancelFunc, Duration, Mutex, Time, WaitGroup, entry, Func, Job (+3 more)

### Community 14 - "Analyze"
Cohesion: 0.13
Nodes (33): Detail, Diagnostic, m, Plan, Request, Service, Stage, Summary (+25 more)

### Community 15 - "Mango3S (MongoDB desktop IDE)"
Cohesion: 0.06
Nodes (38): app service (GUI, Linux X11), integration-tests service (test profile), mongo service (MongoDB 8.0, seeded), mongo-auth service (auth profile), docker/mongo-init seed scripts, /src/main.tsx entry script, index.html #root mount, Aggregation Editor (+30 more)

### Community 16 - "types.ts"
Cohesion: 0.03
Nodes (110): Checks(), ConfirmInput(), EndpointPicker(), JobStatus(), Stat(), useJob(), useJobDone(), AI_FIELDS (+102 more)

### Community 17 - "ShellView.tsx"
Cohesion: 0.08
Nodes (39): KIND_LABELS, needsCollection(), openAction(), timeAgo(), ctx, labels(), COLLECTION_METHODS, currentCollection() (+31 more)

### Community 18 - "schema.go"
Cohesion: 0.16
Nodes (14): D, DateTime, lessPath(), New(), ptr(), toFloat(), typeStats(), acc (+6 more)

### Community 19 - "manage/manage.go"
Cohesion: 0.09
Nodes (47): FindOptionsBuilder, CheckValidator(), ClearCollection(), CollectionOptions(), CreateCollection(), CreateCommand(), CreateIndex(), CreateView() (+39 more)

### Community 20 - "compilerOptions"
Cohesion: 0.07
Nodes (28): compilerOptions, allowJs, allowSyntheticDefaultImports, esModuleInterop, forceConsistentCasingInFileNames, isolatedModules, jsx, lib (+20 more)

### Community 21 - "ManageDialogs.tsx"
Cohesion: 0.06
Nodes (40): app, backend, conn, addIndexKey(), CollectionInfoDialog(), CopyDialog(), CreateCollectionDialog(), emptyCreate() (+32 more)

### Community 22 - "ConnectionManager.tsx"
Cohesion: 0.07
Nodes (36): ConnectionExportDialog(), ConnectionImportDialog(), defaultAction(), blankProfile(), CONNECTION_TABS, ConnectionManager(), emptyAdvanced(), ERROR_TABS (+28 more)

### Community 23 - "logging.go"
Cohesion: 0.14
Nodes (17): Attr, Handler, Context, LevelVar, Logger, isSecretKey(), NewRedactingHandler(), ParseLevel() (+9 more)

### Community 24 - "App.tsx"
Cohesion: 0.08
Nodes (30): GlobalShortcuts(), TabBar(), AboutDialog(), Component, base(), CollectionIcon(), CollectionKind, DatabaseIcon() (+22 more)

### Community 25 - ".convertValues"
Cohesion: 0.03
Nodes (20): AIProviders, BackupRequest, CollectionStats, DBResult, ImportOptions, ImportPreview, Inspection, MCPInfo (+12 more)

### Community 26 - "open"
Cohesion: 0.06
Nodes (52): Certificate, Check, describe(), Client, Context, Credentials, RWMutex, contains() (+44 more)

### Community 28 - ".cancel"
Cohesion: 0.17
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

### Community 33 - "newFixture"
Cohesion: 0.08
Nodes (35): fixture, App, decodeErr(), Buffer, T, newFixture(), TestCancelOperation(), TestConnectFailureIsReadable() (+27 more)

### Community 34 - "backup.go"
Cohesion: 0.18
Nodes (15): Header, Info, ManyResult, RestoreStats, Create(), Collection, Context, Time (+7 more)

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
Cohesion: 0.12
Nodes (21): detectPlatform(), isTextTarget(), matchIndex(), Platform, ShortcutDef, Probe(), BY_PRIORITY, dialogOpen() (+13 more)

### Community 46 - "Apply"
Cohesion: 0.12
Nodes (25): Counts, DocDiff, Apply(), BuildPlan(), bulk(), Collection, Context, D (+17 more)

### Community 47 - "dial_test.go"
Cohesion: 0.13
Nodes (35): Int32, asAppErr(), dialer(), freePort(), Conn, Profile, T, roundTrip() (+27 more)

### Community 48 - "App"
Cohesion: 0.09
Nodes (3): App, Task, TaskType

### Community 49 - ".writableSession"
Cohesion: 0.16
Nodes (9): IndexOptions, client(), T, TestIndexAdvisor(), TestManageAgainstRealServer(), TestProfiler(), tlog, Profiling (+1 more)

### Community 50 - "compare.go"
Cohesion: 0.14
Nodes (30): CollectionRow, DBOptions, DBResult, DocDiff, Endpoint, Entry, FieldChange, fingerprint (+22 more)

### Community 51 - "ai.go"
Cohesion: 0.06
Nodes (42): answer, Check, Claude, Context, FieldSummary, Proposal, Provider, Request (+34 more)

### Community 52 - "p2.go"
Cohesion: 0.11
Nodes (21): AIProviders, AIStatus, exportTask, maskCopyTask, MCPInfo, p2, syncTask, xformPlan (+13 more)

### Community 53 - "migrate.go"
Cohesion: 0.23
Nodes (24): contains(), copyTable(), embed(), Execute(), fieldFor(), Collection, Context, D (+16 more)

### Community 54 - "BuildClientOptions"
Cohesion: 0.16
Nodes (22): ClientOptions, ParseTags(), applyAdvanced(), applyAuth(), applyTLS(), BuildClientOptions(), CheckTLSFiles(), AdvancedConfig (+14 more)

### Community 55 - "CollectionView.tsx"
Cohesion: 0.08
Nodes (63): AIDialog(), BulkWriteDialog(), CodeDialog(), CodeEditor(), CodeEditorProps, HELPERS, OPERATORS, selectionOrAll() (+55 more)

### Community 56 - "transfer_test.go"
Cohesion: 0.24
Nodes (21): NewCSVReader(), NewJSONReader(), dataset(), D, T, readAll(), TestCSVExportImportRoundTrip(), TestCSVReaderErrors() (+13 more)

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
Cohesion: 0.22
Nodes (12): Buffer, CanonicalJSON(), escapeRegexSlashes(), FormatDouble(), formatTime(), D, Time, Quote() (+4 more)

### Community 86 - "shortcuts.ts"
Cohesion: 0.13
Nodes (12): CODE_KEYS, Conflict, eventCombo(), eventKey(), Handler, KEY_LABELS, KeymapOverrides, MAC_SYMBOLS (+4 more)

### Community 87 - "providers_test.go"
Cohesion: 0.34
Nodes (12): argAfter(), fakeRunner(), T, hasArg(), TestClaudeCodeErrors(), TestClaudeCodeRunsWithoutToolsOrSettings(), TestCodexUsesReadOnlySandboxAndSchemaFile(), TestDetectClaudeCode() (+4 more)

### Community 88 - "New"
Cohesion: 0.19
Nodes (16): ClaudeCode, Codex, cliError(), emptyDir(), Usage, New(), Logger, NewManager() (+8 more)

### Community 89 - ".Find"
Cohesion: 0.20
Nodes (15): CollectionRef(), Context, D, Page, New(), Parse(), parseField(), T (+7 more)

### Community 90 - "csv.go"
Cohesion: 0.18
Nodes (12): ConvertCell(), D, InferType(), LeafPaths(), LookupPath(), NewCSVWriter(), setPath(), validType() (+4 more)

### Community 91 - "Invalid"
Cohesion: 0.16
Nodes (20): Code, Error, Check(), firstLine(), From(), Invalid(), Is(), New() (+12 more)

### Community 92 - "App"
Cohesion: 0.11
Nodes (6): App, FieldResult, FieldUpdate, FieldValue, ParsedURI, ValidationReport

### Community 93 - "migrate_test.go"
Cohesion: 0.22
Nodes (14): clip(), ConvertValue(), SuggestType(), SuggestTypeFor(), toInt64(), T, SeedSQLite(), TestConvertValue() (+6 more)

### Community 94 - "Mango3S P2 reference"
Cohesion: 0.05
Nodes (39): Advanced features: matrix, behavior and limits, Feature matrix, Limits, Logical exports vs full backups, Safety, Access control (deny by default), AI-assisted queries and pipelines, Compare and sync (+31 more)

### Community 95 - "Store"
Cohesion: 0.24
Nodes (9): Duration, entry, Mutex, T, Time, New(), entry, Store (+1 more)

### Community 97 - "setup"
Cohesion: 0.08
Nodes (64): env, D, T, Time, mustTime(), rawDoc(), TestFieldLevelEditing(), TestSystemCollectionsAreListedSeparately() (+56 more)

### Community 98 - "AppContext.tsx"
Cohesion: 0.12
Nodes (25): ConnectionLogDialog(), ICON, logText(), FolderNode, onConnectionStep(), onJobUpdate(), TabsAction, TabsState (+17 more)

### Community 99 - "ReadJSON"
Cohesion: 0.29
Nodes (8): ReadJSON(), Mutex, Source, Time, NewService(), Run, runsFile, Service

### Community 100 - "wait"
Cohesion: 0.36
Nodes (13): Logger, NewManager(), Job, T, newManager(), TestCancelRunningAndQueued(), TestIgnoredCancellationStillCancels(), TestJobFailureAndPanic() (+5 more)

### Community 101 - "restore.go"
Cohesion: 0.18
Nodes (23): Entry, Inspection, Rename, RestoreOptions, RestoreReport, Result, collectionExists(), Client (+15 more)

### Community 102 - "ProfileInput"
Cohesion: 0.13
Nodes (11): ProfileInput, SecretInput, Credentials, AdvancedConfig, Profile, ProxyConfig, ServerConfig, ShellConfig (+3 more)

### Community 104 - "advisor.go"
Cohesion: 0.15
Nodes (39): Analyze(), classify(), decide(), defaultName(), docAt(), extract(), D, keyFields() (+31 more)

### Community 105 - "cli.go"
Cohesion: 0.31
Nodes (8): CLIInfo, DetectClaudeCode(), DetectCodex(), exeName(), findCLI(), T, TestLiveProviders(), ExtractJSON()

### Community 106 - "profiles.go"
Cohesion: 0.16
Nodes (16): Capability, ConnectionStep, agentNote(), agentSupported(), BeforeClose(), goos(), HideApp(), MenuEvent() (+8 more)

### Community 108 - "Run"
Cohesion: 0.21
Nodes (20): Binary, collInfo, Logger, Options, Selection, collectionInfo(), dumpOne(), glob() (+12 more)

### Community 109 - "BackupDialogs.tsx"
Cohesion: 0.07
Nodes (34): app, backend, conn, inspection, BackupDialog(), joinPath(), RestoreDialog(), safeName() (+26 more)

### Community 110 - ".backupDir"
Cohesion: 0.18
Nodes (5): Func, expandPath(), Time, NewMCPServer(), Server

### Community 111 - "codegen_test.go"
Cohesion: 0.34
Nodes (14): fakeProvider, aggSpec(), gen(), Spec, T, mustContain(), richSpec(), TestAggregateAllLanguages() (+6 more)

### Community 112 - "Compact"
Cohesion: 0.22
Nodes (16): Compact(), Equal(), Format(), T, mustDec(), mustHex(), mustOID(), TestErrorPosition() (+8 more)

### Community 113 - "App.test.tsx"
Cohesion: 0.13
Nodes (11): TabErrorBoundary, backend, connectAndOpenOrders(), dismissConnectLog(), jobListeners, key(), menuListeners, profile (+3 more)

### Community 115 - "detectOne"
Cohesion: 0.28
Nodes (13): Detect(), detectOne(), exe(), firstLine(), isFile(), locate(), majorMinor(), T (+5 more)

### Community 116 - ".aiProvider"
Cohesion: 0.33
Nodes (3): Duration, SetAIProvider(), Provider

### Community 117 - "NewStore"
Cohesion: 0.39
Nodes (8): NewStore(), T, TestDefaultViewIsTreeUntilChosen(), TestKeymapNormalize(), TestSettingsNormalize(), TestSplitCredentials(), TestStoreProfilesCRUD(), TestStoreSettingsAndCorruption()

### Community 118 - "KeymapSettings.tsx"
Cohesion: 0.21
Nodes (16): app, backend, rerender, Keymap, KeymapSettings(), parseKeymapFile(), TEXT_EDITING, bindingFromEvent() (+8 more)

### Community 119 - "docactions.test.tsx"
Cohesion: 0.19
Nodes (9): api(), cell(), doc, Extra, focusCell(), fv(), page, result() (+1 more)

### Community 120 - "dial.go"
Cohesion: 0.12
Nodes (23): Addr, ClientConfig, HostKeyCallback, buildDialer(), classifySSH(), containsAny(), Client, Conn (+15 more)

### Community 123 - "App"
Cohesion: 0.12
Nodes (4): ImportOptions, App, ExportOptions, Job

### Community 150 - "profile.go"
Cohesion: 0.12
Nodes (14): AdvancedConfig, HostPort, ParsedURI, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig (+6 more)

### Community 152 - "dump_integration_test.go"
Cohesion: 0.27
Nodes (16): tlog, client(), Client, Collection, D, Raw, T, indexNames() (+8 more)

### Community 153 - "ParseValue"
Cohesion: 0.24
Nodes (12): TypeName(), TestParseDocumentVariants(), fromExtJSONWrapper(), D, narrowInt(), NewParser(), ParseDocument(), ParseDocuments() (+4 more)

### Community 155 - "api/connio.go"
Cohesion: 0.17
Nodes (12): ConnectionExport, ImportChoice, ImportItem, ImportOutcome, ImportPreview, App, Profile, hostsSummary() (+4 more)

### Community 160 - ".Replace"
Cohesion: 0.24
Nodes (13): D, Document, Service, Source, IDFilter(), IDsFilter(), New(), ParseID() (+5 more)

### Community 161 - "profiles_test.go"
Cohesion: 0.57
Nodes (6): fullInput(), T, TestDuplicateAndDeleteProfile(), TestParseConnectionStringNeverReturnsThePassword(), TestProfileValidationErrorsReachTheUI(), TestSaveProfileAllTabsAndSecrets()

### Community 162 - "dump/format.go"
Cohesion: 0.13
Nodes (15): closers, fileWriter, Manifest, Metadata, Namespace, TestEscapeNames(), escapeName(), ext() (+7 more)

### Community 163 - "connio/connio.go"
Cohesion: 0.15
Nodes (27): AEAD, Candidate, compassConn, File, Parsed, Sealed, Secrets, checkURI() (+19 more)

### Community 167 - "dump/tools.go"
Cohesion: 0.27
Nodes (14): ToolArgs, ToolDump, ToolRestore, ToolSummary, CanUseTools(), connArgs(), DumpArgs(), Credentials (+6 more)

### Community 168 - "NewAnalyzer"
Cohesion: 0.44
Nodes (10): Field, NewAnalyzer(), field(), Report, T, TestAnalyzer(), TestDecimalRange(), TestEmpty() (+2 more)

### Community 170 - "Writer"
Cohesion: 0.27
Nodes (8): line, Writer, File, canonical(), D, Mutex, RawMessage, WriteManyWithBackup()

### Community 171 - "denied"
Cohesion: 0.24
Nodes (8): contains(), denied(), Mutex, NewStore(), toolInfo(), Policy, Store, ToolInfo

### Community 173 - ".Connect"
Cohesion: 0.31
Nodes (3): Reporter, SessionInfo, TestResult

### Community 189 - "server.go"
Cohesion: 0.24
Nodes (13): CallToolResult, In, add(), errorResult(), textResult(), aggIn, collIn, connIn (+5 more)

### Community 190 - "App"
Cohesion: 0.07
Nodes (20): Deps, Bool, CollectionInfo, CollectionStats, DatabaseInfo, History, call(), App (+12 more)

### Community 193 - "dump_test.go"
Cohesion: 0.23
Nodes (12): docReader, drain(), T, TestDocReader(), TestGlobAndSelection(), TestInspectMissingMetadataWarns(), TestPrepareDirRefusesNonEmpty(), TestRename() (+4 more)

### Community 195 - "mcpserver_test.go"
Cohesion: 0.33
Nodes (12): ClientSession, call(), connectClient(), Policy, T, TestAuditLogOmitsArguments(), TestBoundaryEnforcementBeforeConnecting(), TestCheckValueRefusesJavaScriptAndWrites() (+4 more)

### Community 201 - "Service"
Cohesion: 0.33
Nodes (5): Service, Job, Mutex, Source, New()

### Community 206 - ".ExecuteBulkWrite"
Cohesion: 0.36
Nodes (5): BulkPreview, BulkResult, BulkWriteRequest, D, orEmpty()

### Community 209 - ".registerChainTask"
Cohesion: 0.21
Nodes (8): backupTask, chainStep, chainTask, ImportResult, App, Options, RawMessage, stepName()

### Community 213 - "TestEditTextRoundTrips"
Cohesion: 0.46
Nodes (7): Decimal128, ParseTyped(), T, mustDec(), TestEditTextRoundTrips(), TestParseTypedErrorsAreUseful(), TestParseTypedPreservesTypes()

### Community 214 - "Parse"
Cohesion: 0.38
Nodes (10): Parse(), Profile, T, profiles(), TestExportedSecretsNeedThePassphrase(), TestExportWithoutSecretsHasNoSecrets(), TestParseCompassExport(), TestParseRejectsUnknownFiles() (+2 more)

### Community 215 - "Session"
Cohesion: 0.13
Nodes (8): Advice, AdviceOptions, CollectionDetails, DatabaseDetails, ToolsConfig, ProfileEntry, ProfileQuery, Session

### Community 250 - ".StartRestoreBackup"
Cohesion: 0.14
Nodes (16): BackupRequest, BackupTools, RestoreRequest, Info, Inspection, App, Credentials, Job (+8 more)

### Community 252 - "NewDocument"
Cohesion: 0.24
Nodes (9): ComputePaging(), D, NewDocument(), T, TestComputePaging(), TestNewDocument(), Document, Page (+1 more)

### Community 275 - "NewJSONWriter"
Cohesion: 0.23
Nodes (7): Decoder, D, NewJSONWriter(), truncate(), JSONReader, JSONWriter, Writer

### Community 276 - "p2.test.tsx"
Cohesion: 0.20
Nodes (8): app, backend, base, AIProposal, Task, isApproved(), TasksView(), toDraft()

### Community 278 - "copyOne"
Cohesion: 0.36
Nodes (9): CopyOptions, CopyReport, Copy(), copyOne(), Client, D, Logger, Namespace (+1 more)

### Community 291 - "WelcomeView.tsx"
Cohesion: 0.27
Nodes (9): SCOPE_LABEL, ShortcutsDialog(), formatBinding(), formatShortcut(), SHORTCUT_BY_ID, SHORTCUTS, getShortcutPlatform(), TIPS (+1 more)

### Community 293 - "docviews.test.tsx"
Cohesion: 0.18
Nodes (7): ada(), fields, grace, Harness(), page(), FieldResult, FieldValue

### Community 294 - ".Open"
Cohesion: 0.49
Nodes (9): check(), export(), Collection, Report, T, seed(), TestMySQLRoundTrip(), TestPostgresRoundTrip() (+1 more)

### Community 296 - "NewID"
Cohesion: 0.47
Nodes (5): NewID(), T, queryReq(), TestRenameAndAppendField(), TestUpdateAndRemoveDialogs()

### Community 312 - "HashDoc"
Cohesion: 0.42
Nodes (8): HashDoc(), doc(), D, T, TestDiff(), TestFingerprintOrderIndependent(), TestIDKeyRoundTrip(), TestNormalizeAndHash()

### Community 313 - ".StartCopy"
Cohesion: 0.25
Nodes (4): CopyRequest, IndexChange, SQLExportRequest, CopyOptions

### Community 314 - "TestBuildPlan"
Cohesion: 0.47
Nodes (5): Result, T, result(), TestBuildPlan(), TestPlanStoreSingleUseAndExpiry()

## Knowledge Gaps
- **297 isolated node(s):** `Phase 0: Foundations`, `Phase 1: Quick wins`, `Phase 2: Operations and administration`, `Phase 3: Shell`, `Phase 4: Working with data` (+292 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **239 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Info` connect `Info` to `models.ts`, `ProfileInput`, `call`, `App`, `.aiProvider`, `App`?**
  _High betweenness centrality (0.102) - this node is a cross-community bridge._
- **Why does `App` connect `App` to `Manager`, `.Connect`, `.aiProvider`, `p2.go`, `New`, `.cancel`?**
  _High betweenness centrality (0.090) - this node is a cross-community bridge._
- **Why does `Manager` connect `Manager` to `ReadJSON`, `wait`, `Scheduler`, `Service`, `Apply`, `App`?**
  _High betweenness centrality (0.058) - this node is a cross-community bridge._
- **What connects `Phase 0: Foundations`, `Phase 1: Quick wins`, `Phase 2: Operations and administration` to the rest of the system?**
  _297 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `transfer.go` be split into smaller, more focused modules?**
  _Cohesion score 0.12462462462462462 - nodes in this community are weakly interconnected._
- **Should `parser` be split into smaller, more focused modules?**
  _Cohesion score 0.08169014084507042 - nodes in this community are weakly interconnected._
- **Should `runtime.js` be split into smaller, more focused modules?**
  _Cohesion score 0.03076923076923077 - nodes in this community are weakly interconnected._