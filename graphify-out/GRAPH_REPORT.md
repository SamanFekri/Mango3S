# Graph Report - Mango3S  (2026-10-16)

## Corpus Check
- 183 files · ~191,393 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 3025 nodes · 6876 edges · 210 communities (100 shown, 110 thin omitted)
- Extraction: 90% EXTRACTED · 10% INFERRED · 0% AMBIGUOUS · INFERRED: 717 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `30c6bd0b`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- transfer.go
- Service
- p1_test.go
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
- Session
- Manager
- .cancel
- dependencies
- devDependencies
- runtime/package.json
- App
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
- App.test.tsx
- shortcuts.tsx
- CollectionStats
- dial_test.go
- App
- Item
- compare.go
- ai.go
- p2.go
- migrate.go
- Request
- SessionInfo
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
- Profile
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
- server.go
- mql/format.go
- Collection
- dial.go
- .Replace
- .Find
- csv.go
- ParseValue
- App
- migrate_test.go
- Mango3S P2 reference
- Store
- privileges.go
- p2_test.go
- profile.go
- Service
- wait
- NewDocument
- NewJSONWriter
- newFixture
- setup
- MCPInfo
- Is
- CodeRequest
- DBResult
- DocDiff
- Field
- codegen_test.go
- newFixture
- Options
- Outcome
- tools.go
- BuildClientOptions
- docviews.test.tsx
- newApp
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
- BuildURI
- options_test.go
- Page
- ProfileInput
- Manager
- p2.test.tsx
- New
- WaitGroup
- TestFieldLevelEditing
- buildDialer
- ParseTyped
- profiles_test.go
- ParseCron
- Mango3S UI reference: Welcome tab, connections, editing and shortcuts
- profiles.go
- Profile
- Source
- Context
- ImportOptions
- Output
- Page
- Profile
- Run
- ServerConfig
- Stage
- TableMapping
- .IntrospectSQL
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
- DatabaseInfo
- env
- CancelFunc
- Context
- LevelVar
- Profile
- Report
- Request
- Result
- Service
- Stage
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
- RWMutex
- SessionInfo

## God Nodes (most connected - your core abstractions)
1. `Invalid()` - 57 edges
2. `App` - 52 edges
3. `App` - 47 edges
4. `useApp()` - 46 edges
5. `Compact()` - 42 edges
6. `Is()` - 39 edges
7. `Collection()` - 39 edges
8. `Scheduler` - 38 edges
9. `toAppError()` - 36 edges
10. `call()` - 36 edges

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

## Communities (210 total, 110 thin omitted)

### Community 0 - "transfer.go"
Cohesion: 0.14
Nodes (30): ColumnSpec, ParseDelimiter(), DetectFormat(), exportQuery(), fieldsProjection(), firstLine(), Context, D (+22 more)

### Community 1 - "Service"
Cohesion: 0.16
Nodes (11): CollectionInfo, CollectionStats, DatabaseInfo, IndexInfo, Service, Check(), CollectPaths(), D (+3 more)

### Community 2 - "p1_test.go"
Cohesion: 0.26
Nodes (18): allDocs(), byID(), Context, D, Job, Manager, T, representative() (+10 more)

### Community 3 - "parser"
Cohesion: 0.08
Nodes (43): mqlCompact(), errAt(), lex(), isLiteralFunc(), parse(), compact(), T, TestErrorPosition() (+35 more)

### Community 5 - "Parser"
Cohesion: 0.13
Nodes (13): construct(), fromExtJSONWrapper(), Time, isIdentPart(), isIdentStart(), narrowInt(), NewParser(), ParseDate() (+5 more)

### Community 6 - "WriteJSON"
Cohesion: 0.06
Nodes (34): WriteJSON(), Mutex, Stage, Time, NewHistory(), NewStore(), sameRun(), T (+26 more)

### Community 7 - ".evalColl"
Cohesion: 0.12
Nodes (39): Cursor, Database, describe(), Parse(), parseArgs(), parseChain(), parseStatement(), peekIdent() (+31 more)

### Community 8 - "codegen.go"
Cohesion: 0.18
Nodes (35): Language, renderer, Spec, b64(), block(), csRenderer(), escapeRegex(), floatLit() (+27 more)

### Community 9 - "App"
Cohesion: 0.07
Nodes (25): CodeRequest, exportTask, SQLRequest, SQLResult, SQLTranslation, Entry, ExportOptions, ImportOptions (+17 more)

### Community 11 - "Build"
Cohesion: 0.19
Nodes (17): Request, Service, Stage, Build(), FromText(), Context, Page, indentLines() (+9 more)

### Community 12 - "DocViews.tsx"
Cohesion: 0.07
Nodes (64): Harness(), CellEditing, CellEditor(), CellRef, ResultsNotice, sameRef(), scrollIntoView(), step() (+56 more)

### Community 13 - "Manager"
Cohesion: 0.12
Nodes (14): CancelFunc, Context, Duration, Logger, Mutex, Time, WaitGroup, NewManager() (+6 more)

### Community 14 - "Analyze"
Cohesion: 0.14
Nodes (31): Detail, Diagnostic, m, Plan, Request, Stage, Summary, Analyze() (+23 more)

### Community 15 - "Mango3S (MongoDB desktop IDE)"
Cohesion: 0.06
Nodes (38): app service (GUI, Linux X11), integration-tests service (test profile), mongo service (MongoDB 8.0, seeded), mongo-auth service (auth profile), docker/mongo-init seed scripts, /src/main.tsx entry script, index.html #root mount, Aggregation Editor (+30 more)

### Community 16 - "types.ts"
Cohesion: 0.04
Nodes (88): Checks(), ConfirmInput(), EndpointPicker(), JobStatus(), Stat(), useJob(), useJobDone(), AISettings() (+80 more)

### Community 17 - "AppContext.tsx"
Cohesion: 0.07
Nodes (48): onJobUpdate(), KIND_LABELS, needsCollection(), openAction(), timeAgo(), ctx, labels(), COLLECTION_METHODS (+40 more)

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
Nodes (55): AIDialog(), CodeDialog(), CodeEditor(), CodeEditorProps, HELPERS, OPERATORS, selectionOrAll(), theme (+47 more)

### Community 22 - "ConnectionManager.tsx"
Cohesion: 0.08
Nodes (33): blankProfile(), CONNECTION_TABS, ConnectionManager(), emptyAdvanced(), ERROR_TABS, FolderNode, folderTree(), fromProfile() (+25 more)

### Community 23 - "logging.go"
Cohesion: 0.14
Nodes (17): Attr, Handler, Context, LevelVar, Logger, isSecretKey(), NewRedactingHandler(), ParseLevel() (+9 more)

### Community 24 - "App.tsx"
Cohesion: 0.11
Nodes (27): GlobalShortcuts(), Shell(), StatusBar(), TabBar(), ConfirmDialog(), PromptDialog(), Toasts(), base() (+19 more)

### Community 25 - ".convertValues"
Cohesion: 0.04
Nodes (14): Entry, FieldResult, Info, Job, MCPInfo, ParsedURI, Plan, Preview (+6 more)

### Community 26 - "Session"
Cohesion: 0.20
Nodes (12): describe(), Client, Logger, Profile, RWMutex, Time, open(), via() (+4 more)

### Community 28 - ".cancel"
Cohesion: 0.18
Nodes (7): AppInfo, Lifecycle, opRegistry, runningOp, CancelFunc, Mutex, newOpRegistry()

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
Nodes (18): Bool, CollectionInfo, CollectionStats, History, call(), App, DatabaseInfo, Document (+10 more)

### Community 33 - "Open"
Cohesion: 0.13
Nodes (11): AEAD, Mutex, loadOrCreateKey(), NewFileStore(), NewMemoryStore(), Open(), probeKeyring(), FileStore (+3 more)

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

### Community 44 - "App.test.tsx"
Cohesion: 0.18
Nodes (7): TabErrorBoundary, backend, connectAndOpenOrders(), jobListeners, key(), profile, session

### Community 45 - "shortcuts.tsx"
Cohesion: 0.08
Nodes (40): SCOPE_LABEL, ShortcutsDialog(), CODE_KEYS, detectPlatform(), eventCombo(), eventKey(), formatBinding(), formatShortcut() (+32 more)

### Community 47 - "dial_test.go"
Cohesion: 0.14
Nodes (35): Int32, asAppErr(), dialer(), freePort(), Conn, Profile, T, roundTrip() (+27 more)

### Community 48 - "App"
Cohesion: 0.06
Nodes (18): Provider, DBOptions, DBResult, Info, App, Context, Job, Proposal (+10 more)

### Community 50 - "compare.go"
Cohesion: 0.05
Nodes (73): CollectionRow, DBOptions, DBResult, DocDiff, Endpoint, Entry, FieldChange, fingerprint (+65 more)

### Community 51 - "ai.go"
Cohesion: 0.07
Nodes (45): answer, Check, Claude, Context, FieldSummary, Proposal, Request, Service (+37 more)

### Community 52 - "p2.go"
Cohesion: 0.11
Nodes (19): AIStatus, maskCopyTask, p2, syncTask, TransformPreview, TransformRequest, xformPlan, Func (+11 more)

### Community 53 - "migrate.go"
Cohesion: 0.23
Nodes (24): contains(), copyTable(), embed(), Execute(), fieldFor(), Collection, Context, D (+16 more)

### Community 56 - "Compact"
Cohesion: 0.26
Nodes (20): Compact(), NewJSONReader(), dataset(), D, T, readAll(), TestCSVExportImportRoundTrip(), TestCSVReaderErrors() (+12 more)

### Community 57 - "dialect.go"
Cohesion: 0.22
Nodes (21): DB, Redact(), beginRead(), dialectFor(), Context, Rows, Tx, Introspect() (+13 more)

### Community 69 - "Profile"
Cohesion: 0.12
Nodes (22): profilesFile, Settings, Store, DefaultSettings(), AdvancedConfig, Profile, Mutex, ProxyConfig (+14 more)

### Community 79 - "EventsOn"
Cohesion: 0.67
Nodes (3): EventsOn(), EventsOnce(), EventsOnMultiple()

### Community 83 - "Invalid"
Cohesion: 0.22
Nodes (13): Code, Error, firstLine(), From(), Invalid(), New(), T, TestCheckNilIsUntypedNil() (+5 more)

### Community 84 - "server.go"
Cohesion: 0.07
Nodes (45): CallToolResult, ClientSession, In, call(), connectClient(), Policy, T, TestAuditLogOmitsArguments() (+37 more)

### Community 85 - "mql/format.go"
Cohesion: 0.19
Nodes (15): Buffer, EditText(), CanonicalJSON(), escapeRegexSlashes(), Format(), FormatDouble(), formatTime(), D (+7 more)

### Community 86 - "Collection"
Cohesion: 0.23
Nodes (15): FieldResult, FieldUpdate, FieldValue, ParseID(), checkParents(), describeField(), D, Document (+7 more)

### Community 87 - "dial.go"
Cohesion: 0.16
Nodes (15): Addr, classifySSH(), containsAny(), Client, Conn, Mutex, Reader, keepAlive() (+7 more)

### Community 88 - ".Replace"
Cohesion: 0.23
Nodes (13): A, D, Document, Service, Source, IDFilter(), IDsFilter(), New() (+5 more)

### Community 89 - ".Find"
Cohesion: 0.19
Nodes (16): CollectionRef(), Context, D, Page, New(), Parse(), parseField(), T (+8 more)

### Community 90 - "csv.go"
Cohesion: 0.15
Nodes (16): ConvertCell(), D, InferType(), LeafPaths(), LookupPath(), NewCSVReader(), NewCSVWriter(), setPath() (+8 more)

### Community 91 - "ParseValue"
Cohesion: 0.23
Nodes (18): Equal(), T, mustDec(), mustHex(), mustOID(), TestErrorPosition(), TestFormatShape(), TestParseDocumentVariants() (+10 more)

### Community 92 - "App"
Cohesion: 0.13
Nodes (9): SecretInput, FieldResult, FieldUpdate, FieldValue, App, Profile, SessionInfo, SecretKey() (+1 more)

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

### Community 97 - "p2_test.go"
Cohesion: 0.25
Nodes (22): env, coll(), App, Source, T, mongoURI(), newApp(), runMigration() (+14 more)

### Community 98 - "profile.go"
Cohesion: 0.15
Nodes (13): AdvancedConfig, HostPort, ParsedURI, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig (+5 more)

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

### Community 103 - "newFixture"
Cohesion: 0.30
Nodes (16): fixture, App, Error, decodeErr(), Buffer, T, newFixture(), TestCancelOperation() (+8 more)

### Community 104 - "setup"
Cohesion: 0.29
Nodes (16): contains(), containsDB(), DatabaseInfo, T, nField(), seed(), setup(), TestAggregation() (+8 more)

### Community 105 - "MCPInfo"
Cohesion: 0.40
Nodes (3): MCPInfo, Policy, ToolInfo

### Community 106 - "Is"
Cohesion: 0.29
Nodes (10): Is(), T, TestBuildClientOptions(), TestRequireWritable(), TestUnreachableServerIsClassified(), T, TestFileStore(), TestFileStoreRejectsSwappedCiphertext() (+2 more)

### Community 111 - "codegen_test.go"
Cohesion: 0.34
Nodes (14): fakeProvider, aggSpec(), gen(), Spec, T, mustContain(), richSpec(), TestAggregateAllLanguages() (+6 more)

### Community 112 - "newFixture"
Cohesion: 0.31
Nodes (13): Duration, Mutex, T, Time, newFixture(), TestApprovalGatesExecution(), TestCronParseAndNext(), TestNoOverlap() (+5 more)

### Community 115 - "tools.go"
Cohesion: 0.27
Nodes (13): commonDirs(), Detect(), detectOne(), exe(), firstLine(), ToolsConfig, isFile(), locate() (+5 more)

### Community 116 - "BuildClientOptions"
Cohesion: 0.32
Nodes (12): ClientOptions, applyAdvanced(), applyAuth(), applyTLS(), BuildClientOptions(), CheckTLSFiles(), AdvancedConfig, Profile (+4 more)

### Community 117 - "docviews.test.tsx"
Cohesion: 0.18
Nodes (8): EditState, ada(), fields, grace, Harness(), page(), FieldResult, FieldValue

### Community 118 - "newApp"
Cohesion: 0.27
Nodes (9): NewLifecycle(), BeforeClose(), MenuEvent(), appMenu(), App, newApp(), runDesktop(), runMCP() (+1 more)

### Community 150 - "BuildURI"
Cohesion: 0.31
Nodes (10): BuildURI(), Profile, T, TestBuildURI(), TestNormalizeDefaults(), TestParseURIRoundTrip(), TestProfileValidationNamesTheTab(), TestSecretFlags() (+2 more)

### Community 151 - "options_test.go"
Cohesion: 0.33
Nodes (9): T, Time, TestAdvancedOptionsOverrideURI(), TestAllowInvalidHostnamesStillVerifiesTheChain(), TestAuthMechanismsAndProperties(), TestTLSErrorsAreClassified(), TestTLSFilesAreValidatedWithClearMessages(), writeCert() (+1 more)

### Community 153 - "ProfileInput"
Cohesion: 0.20
Nodes (8): ProfileInput, AdvancedConfig, ProxyConfig, ServerConfig, ShellConfig, SSHConfig, TLSConfig, ToolsConfig

### Community 155 - "p2.test.tsx"
Cohesion: 0.20
Nodes (8): app, backend, base, AIProposal, Task, isApproved(), TasksView(), toDraft()

### Community 156 - "New"
Cohesion: 0.16
Nodes (9): Deps, Logger, New(), expandPath(), Report, Time, NewMCPServer(), LevelVar (+1 more)

### Community 158 - "TestFieldLevelEditing"
Cohesion: 0.36
Nodes (8): D, T, Time, mustTime(), rawDoc(), TestFieldLevelEditing(), TestSystemCollectionsAreListedSeparately(), uriFromEnv()

### Community 159 - "buildDialer"
Cohesion: 0.29
Nodes (8): ClientConfig, HostKeyCallback, buildDialer(), Duration, Profile, SSHConfig, hostKeyCallback(), sshClientConfig()

### Community 160 - "ParseTyped"
Cohesion: 0.46
Nodes (7): Decimal128, ParseTyped(), T, mustDec(), TestEditTextRoundTrips(), TestParseTypedErrorsAreUseful(), TestParseTypedPreservesTypes()

### Community 161 - "profiles_test.go"
Cohesion: 0.46
Nodes (7): fullInput(), T, TestDuplicateAndDeleteProfile(), TestParseConnectionStringNeverReturnsThePassword(), TestProfileValidationErrorsReachTheUI(), TestSaveProfileAllTabsAndSecrets(), ProfileInput

### Community 162 - "ParseCron"
Cohesion: 0.39
Nodes (4): Time, ParseCron(), parseField(), Cron

### Community 163 - "Mango3S UI reference: Welcome tab, connections, editing and shortcuts"
Cohesion: 0.29
Nodes (6): Connection Manager, Keyboard shortcuts, Mango3S UI reference: Welcome tab, connections, editing and shortcuts, Result views and field editing, Unsupported and limitations, Welcome tab and workspace tabs

### Community 164 - "profiles.go"
Cohesion: 0.60
Nodes (4): Capability, agentNote(), agentSupported(), goos()

### Community 166 - "Source"
Cohesion: 0.60
Nodes (5): Service, New(), New(), Source, Service

## Knowledge Gaps
- **227 isolated node(s):** `Welcome tab and workspace tabs`, `Connection Manager`, `Result views and field editing`, `Keyboard shortcuts`, `Unsupported and limitations` (+222 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **110 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Invalid()` connect `Invalid` to `transfer.go`, `Service`, `parser`, `WriteJSON`, `.evalColl`, `codegen.go`, `App`, `Build`, `Analyze`, `buildDialer`, `ParseTyped`, `Profile`, `Collection`, `.Replace`, `.Find`, `App`, `profile.go`, `wait`, `BuildClientOptions`?**
  _High betweenness centrality (0.079) - this node is a cross-community bridge._
- **Why does `NewID()` connect `Scheduler` to `Writer`, `p1_test.go`, `Profile`, `WriteJSON`, `setup`, `Manager`, `dial.go`, `dialect.go`, `Store`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Why does `Is()` connect `Is` to `ParseTyped`, `.Find`, `Open`, `Service`, `parser`, `setup`, `App`, `Build`, `dial_test.go`, `Invalid`, `BuildClientOptions`, `Collection`, `BuildURI`, `.Replace`, `options_test.go`, `App`, `TestFieldLevelEditing`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Are the 54 inferred relationships involving `Invalid()` (e.g. with `.toProfile()` and `.validate()`) actually correct?**
  _`Invalid()` has 54 INFERRED edges - model-reasoned connections that need verification._
- **Are the 40 inferred relationships involving `Compact()` (e.g. with `.Insert()` and `.UpdateField()`) actually correct?**
  _`Compact()` has 40 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Welcome tab and workspace tabs`, `Connection Manager`, `Result views and field editing` to the rest of the system?**
  _227 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `transfer.go` be split into smaller, more focused modules?**
  _Cohesion score 0.1408199643493761 - nodes in this community are weakly interconnected._