# Graph Report - .  (2026-10-15)

## Corpus Check
- 108 files · ~88,921 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1591 nodes · 3703 edges · 83 communities (47 shown, 36 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 417 edges (avg confidence: 0.8)
- Token cost: 60,042 input · 0 output

## Community Hubs (Navigation)
- Import Export Transfer
- Errors Explorer Documents
- Secrets And Integration Tests
- SQL Translator
- MQL Value Parser
- Config And Library Persistence
- IntelliShell Executor
- Code Generation
- P1 API Facade
- Aggregation And Formatting
- Query Builder And Grid
- Background Jobs
- Explain Plan Analysis
- Docs And Docker Stack
- Frontend Backend Wrapper
- Library And Shell Completion
- Schema Analyzer
- Results View Components
- TypeScript Config
- Code Editor And Dialogs
- App State And Tabs
- Logging And Redaction
- App Shell Layout
- Generated Model Classes
- Connection Manager
- Profile And Query API
- App Lifecycle
- Editor Dependencies
- Test Dependencies
- Runtime Package Meta
- Explorer And Document API
- API Unit Tests
- Import Export Wizards
- Generated Model Namespaces
- Wails Project Config
- Connection Profile API
- Node TS Config
- Runtime Type Declarations
- Settings And Shell API
- NPM Scripts
- Sample Seed Data
- Frontend Package Meta
- App Test Fixtures
- Tab Error Boundary
- Wails Model CollectionStats
- Wails Model Entry
- Wails Model ImportOptions
- Wails Model Item
- Wails Model Job
- Wails Model Output
- Wails Model Page
- Wails Model Report
- Wails Model Request
- Wails Model SessionInfo
- Wails Model SQLResult
- Wails Model Stage
- Wails Model AppInfo
- Wails Model CollectionInfo
- Wails Model ColumnPreview
- Wails Model ColumnSpec
- Wails Model DatabaseInfo
- Wails Model Detail
- Wails Model Diagnostic
- Wails Model Document
- Wails Model ExportOptions
- Wails Model IndexInfo
- Wails Model Language
- Wails Model ProfileInput
- Wails Model QueryPayload
- Wails Model Settings
- Wails Model SQLRequest
- Wails Model SQLTranslation
- Wails Model Summary
- Wails Model TestResult
- Wails Model TypeStat
- React DOM Dependency
- Jest DOM Dependency
- Runtime Event Helpers
- Go Module

## God Nodes (most connected - your core abstractions)
1. `App` - 49 edges
2. `Invalid()` - 42 edges
3. `useApp()` - 35 edges
4. `Collection()` - 33 edges
5. `Compact()` - 32 edges
6. `From()` - 30 edges
7. `Parser` - 30 edges
8. `toAppError()` - 29 edges
9. `call()` - 28 edges
10. `Is()` - 25 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `Setup()`  [INFERRED]
  main.go → internal/logging/logging.go
- `run()` --calls--> `Open()`  [INFERRED]
  main.go → internal/secrets/secrets.go
- `run()` --calls--> `NewLifecycle()`  [INFERRED]
  main.go → internal/api/api.go
- `Mango3S (MongoDB desktop IDE)` --references--> `app service (GUI, Linux X11)`  [EXTRACTED]
  README.md → docker-compose.yml
- `Mango3S (MongoDB desktop IDE)` --references--> `integration-tests service (test profile)`  [EXTRACTED]
  README.md → docker-compose.yml

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Editors sharing the mql value language** — readme_query_editor, readme_aggregation_editor, readme_intellishell, readme_visual_query_builder, readme_mql_shared_value_language [EXTRACTED 1.00]
- **P1 features reusing P0 query/aggregate services** — readme_sql_subset, readme_code_generation, readme_explain_plans, readme_import_export_wizard, readme_p1_reuses_p0 [EXTRACTED 1.00]
- **Docker compose dev/test stack** — docker_compose_mongo, docker_compose_mongo_auth, docker_compose_integration_tests, docker_compose_app, docker_compose_mongo_init [EXTRACTED 1.00]

## Communities (83 total, 36 thin omitted)

### Community 0 - "Import Export Transfer"
Cohesion: 0.05
Nodes (73): ColumnSpec, Decoder, Compact(), FormatDouble(), FormatCell(), D, LeafPaths(), LookupPath() (+65 more)

### Community 1 - "Errors Explorer Documents"
Cohesion: 0.06
Nodes (54): Code, Error, Service, CollectionInfo, CollectionStats, DatabaseInfo, IndexInfo, Service (+46 more)

### Community 2 - "Secrets And Integration Tests"
Cohesion: 0.06
Nodes (58): AEAD, env, Is(), NewID(), contains(), containsDB(), Context, DatabaseInfo (+50 more)

### Community 3 - "SQL Translator"
Cohesion: 0.08
Nodes (43): mqlCompact(), errAt(), lex(), isLiteralFunc(), parse(), compact(), T, TestErrorPosition() (+35 more)

### Community 5 - "MQL Value Parser"
Cohesion: 0.09
Nodes (35): Decimal128, Equal(), TypeName(), T, mustDec(), mustHex(), mustOID(), TestErrorPosition() (+27 more)

### Community 6 - "Config And Library Persistence"
Cohesion: 0.07
Nodes (34): Profile, profilesFile, Settings, Store, DefaultSettings(), Mutex, Time, NewStore() (+26 more)

### Community 7 - "IntelliShell Executor"
Cohesion: 0.13
Nodes (40): Cursor, Database, Invalid(), describe(), Parse(), parseArgs(), parseChain(), parseStatement() (+32 more)

### Community 8 - "Code Generation"
Cohesion: 0.13
Nodes (46): Language, renderer, Spec, b64(), block(), csRenderer(), escapeRegex(), floatLit() (+38 more)

### Community 9 - "P1 API Facade"
Cohesion: 0.07
Nodes (25): CodeRequest, SQLRequest, SQLResult, SQLTranslation, Entry, ExportOptions, ImportOptions, codeSpec() (+17 more)

### Community 11 - "Aggregation And Formatting"
Cohesion: 0.08
Nodes (37): Request, Service, Stage, Build(), FromText(), Context, Page, indentLines() (+29 more)

### Community 12 - "Query Builder And Grid"
Cohesion: 0.10
Nodes (41): BuiltQuery, QueryBuilder(), TableView(), Cell, CellKind, columnsOf(), isObject(), JsonObject (+33 more)

### Community 13 - "Background Jobs"
Cohesion: 0.10
Nodes (26): CancelFunc, Context, Duration, Logger, Manager, Mutex, Time, NewManager() (+18 more)

### Community 14 - "Explain Plan Analysis"
Cohesion: 0.13
Nodes (33): Detail, Diagnostic, m, Plan, Request, Service, Stage, Summary (+25 more)

### Community 15 - "Docs And Docker Stack"
Cohesion: 0.06
Nodes (38): app service (GUI, Linux X11), integration-tests service (test profile), mongo service (MongoDB 8.0, seeded), mongo-auth service (auth profile), docker/mongo-init seed scripts, /src/main.tsx entry script, index.html #root mount, Aggregation Editor (+30 more)

### Community 16 - "Frontend Backend Wrapper"
Cohesion: 0.10
Nodes (31): a, wrap(), AggregateRequest, CodeLanguage, CollectionInfo, CollectionStats, ColumnPreview, DatabaseInfo (+23 more)

### Community 17 - "Library And Shell Completion"
Cohesion: 0.11
Nodes (27): selectionOrAll(), KIND_LABELS, needsCollection(), openAction(), timeAgo(), ctx, labels(), COLLECTION_METHODS (+19 more)

### Community 18 - "Schema Analyzer"
Cohesion: 0.13
Nodes (22): Field, D, DateTime, lessPath(), NewAnalyzer(), ptr(), field(), Report (+14 more)

### Community 19 - "Results View Components"
Cohesion: 0.14
Nodes (24): Segmented(), Harness(), page, ExplainDialog(), Metric(), STAGE_HELP, stageClass(), StageNode() (+16 more)

### Community 20 - "TypeScript Config"
Cohesion: 0.07
Nodes (27): compilerOptions, allowJs, allowSyntheticDefaultImports, esModuleInterop, forceConsistentCasingInFileNames, isolatedModules, jsx, lib (+19 more)

### Community 21 - "Code Editor And Dialogs"
Cohesion: 0.21
Nodes (20): CodeDialog(), CodeEditor(), CodeEditorProps, HELPERS, OPERATORS, theme, Spinner(), DocumentEditor() (+12 more)

### Community 22 - "App State And Tabs"
Cohesion: 0.13
Nodes (25): onJobUpdate(), errorTitle(), sameTarget(), tabId(), TabInit, TabKind, TabsAction, tabsReducer() (+17 more)

### Community 23 - "Logging And Redaction"
Cohesion: 0.14
Nodes (18): Attr, Handler, Context, LevelVar, Logger, isSecretKey(), NewRedactingHandler(), ParseLevel() (+10 more)

### Community 24 - "App Shell Layout"
Cohesion: 0.18
Nodes (16): Shell(), StatusBar(), TabBar(), ConfirmDialog(), PromptDialog(), Toasts(), blank(), ConnectionManager() (+8 more)

### Community 25 - "Generated Model Classes"
Cohesion: 0.11
Nodes (6): CodeRequest, Field, Plan, Preview, Profile, Result

### Community 26 - "Connection Manager"
Cohesion: 0.20
Nodes (15): Client, ClientOptions, BuildClientOptions(), describe(), Context, Logger, Profile, RWMutex (+7 more)

### Community 27 - "Profile And Query API"
Cohesion: 0.16
Nodes (11): Deps, App, LevelVar, Logger, Manager, Page, Request, RWMutex (+3 more)

### Community 28 - "App Lifecycle"
Cohesion: 0.15
Nodes (12): AppInfo, Lifecycle, opRegistry, runningOp, CancelFunc, Context, Mutex, New() (+4 more)

### Community 29 - "Editor Dependencies"
Cohesion: 0.11
Nodes (19): codemirror, @codemirror/autocomplete, @codemirror/commands, @codemirror/lang-javascript, @codemirror/lang-sql, @codemirror/language, @codemirror/state, @codemirror/view (+11 more)

### Community 30 - "Test Dependencies"
Cohesion: 0.11
Nodes (19): devDependencies, jsdom, @testing-library/dom, @testing-library/react, @types/react, @types/react-dom, typescript, vite (+11 more)

### Community 31 - "Runtime Package Meta"
Cohesion: 0.11
Nodes (18): author, bugs, url, description, homepage, keywords, license, main (+10 more)

### Community 32 - "Explorer And Document API"
Cohesion: 0.12
Nodes (6): CollectionInfo, CollectionStats, call(), DatabaseInfo, Document, T

### Community 33 - "API Unit Tests"
Cohesion: 0.30
Nodes (16): fixture, App, Error, decodeErr(), Buffer, T, newFixture(), TestCancelOperation() (+8 more)

### Community 34 - "Import Export Wizards"
Cohesion: 0.21
Nodes (12): ErrorBanner(), Modal(), ExportWizard(), COLUMN_TYPES, DELIMITERS, ImportWizard(), Loadable, MenuItem (+4 more)

### Community 35 - "Generated Model Namespaces"
Cohesion: 0.24
Nodes (14): aggregate, api, codegen, config, explain, explorer, jobs, library (+6 more)

### Community 36 - "Wails Project Config"
Cohesion: 0.12
Nodes (15): author, email, name, frontend:build, frontend:dev:serverUrl, frontend:dev:watcher, frontend:install, info (+7 more)

### Community 37 - "Connection Profile API"
Cohesion: 0.29
Nodes (4): ProfileInput, Profile, SessionInfo, TestResult

### Community 38 - "Node TS Config"
Cohesion: 0.22
Nodes (8): compilerOptions, allowSyntheticDefaultImports, composite, module, moduleResolution, skipLibCheck, include, vite.config.ts

### Community 39 - "Runtime Type Declarations"
Cohesion: 0.25
Nodes (7): EnvironmentInfo, NotificationAction, NotificationCategory, NotificationOptions, Position, Screen, Size

### Community 40 - "Settings And Shell API"
Cohesion: 0.33
Nodes (3): Duration, Result, Settings

### Community 41 - "NPM Scripts"
Cohesion: 0.29
Nodes (7): scripts, build, dev, preview, test, test:watch, typecheck

### Community 42 - "Sample Seed Data"
Cohesion: 0.33
Nodes (5): customers, orders, products, statuses, store

### Community 43 - "Frontend Package Meta"
Cohesion: 0.40
Nodes (4): name, private, type, version

### Community 44 - "App Test Fixtures"
Cohesion: 0.40
Nodes (4): backend, jobListeners, profile, session

## Knowledge Gaps
- **159 isolated node(s):** `store`, `customers`, `products`, `statuses`, `orders` (+154 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **36 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Invalid()` connect `IntelliShell Executor` to `Import Export Transfer`, `Errors Explorer Documents`, `SQL Translator`, `Connection Profile API`, `Config And Library Persistence`, `Code Generation`, `P1 API Facade`, `Aggregation And Formatting`, `Background Jobs`, `Explain Plan Analysis`?**
  _High betweenness centrality (0.070) - this node is a cross-community bridge._
- **Why does `From()` connect `Errors Explorer Documents` to `Import Export Transfer`, `Secrets And Integration Tests`, `IntelliShell Executor`, `Aggregation And Formatting`, `Background Jobs`, `Explain Plan Analysis`, `Connection Manager`, `Profile And Query API`?**
  _High betweenness centrality (0.035) - this node is a cross-community bridge._
- **Why does `Is()` connect `Secrets And Integration Tests` to `Errors Explorer Documents`, `SQL Translator`, `Connection Profile API`, `Config And Library Persistence`, `P1 API Facade`, `Aggregation And Formatting`, `Background Jobs`?**
  _High betweenness centrality (0.030) - this node is a cross-community bridge._
- **Are the 39 inferred relationships involving `Invalid()` (e.g. with `.toProfile()` and `.Validate()`) actually correct?**
  _`Invalid()` has 39 INFERRED edges - model-reasoned connections that need verification._
- **Are the 12 inferred relationships involving `Collection()` (e.g. with `.Run()` and `.Delete()`) actually correct?**
  _`Collection()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **Are the 30 inferred relationships involving `Compact()` (e.g. with `.Insert()` and `.CollectionStats()`) actually correct?**
  _`Compact()` has 30 INFERRED edges - model-reasoned connections that need verification._
- **What connects `store`, `customers`, `products` to the rest of the system?**
  _159 weakly-connected nodes found - possible documentation gaps or missing edges._