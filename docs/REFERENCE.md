# Mango3S reference

The complete feature, build and architecture reference. For an overview, downloads and how to
contribute, see the [README](../README.md) or [mango3s.com](https://mango3s.com).

A desktop IDE for MongoDB, built with Go, [Wails](https://wails.io) and React: connection
manager, explorer, query editor, visual query builder, aggregation editor and command shell.

This release covers **P0** (browsing data, querying, editing documents, running pipelines and
shell commands) and **P1**: import/export, schema analysis, SQL, code generation, explain plans,
saved queries, query history and background jobs. **P2** adds compare and sync, migration from
relational databases, reschema, data masking, scheduled tasks, AI-assisted query generation and
an optional MCP server. All of them preview before they write, back up what they change, and
validate the result. The full P2 reference is in [docs/P2.md](P2.md), including the
security model, scheduler policies, MCP setup and unsupported capabilities.

---

## Features

| Area | What you get |
|---|---|
| **App window** | Desktop window with explorer sidebar, draggable tabs (state kept when you switch tabs), status bar, light/dark/system theme, settings page. A **Welcome** tab opens first; each connection gets its own **workspace tab**; closing a tab (or quitting) with unsaved work asks first |
| **Connections** | Connection Manager with nine tabs: Server (standalone / replica set / sharded / SRV, or a connection string), Authentication (SCRAM, X.509, LDAP, AWS IAM, OIDC workload identity), SSL / TLS (CA and client certificates, validated up front), **SSH tunnel** (password, key, agent; host-key pinning), **Proxy** (SOCKS5, HTTP CONNECT), Shell (default database, init commands), MongoDB Tools (detects mongosh and the Database Tools), Advanced (timeouts, read preference, write concern, pool, compression) and Collection History. Folders, duplicate, delete; every secret in the OS credential store. See [docs/UI.md](UI.md) |
| **Explorer** | Connections → databases → **Collections / Views / System** folders (system.* collections are listed too), with distinct icons, keyboard navigation (arrows, Enter, Delete), filter and context menus |
| **Keyboard** | Central shortcut manager with a searchable reference (Help → Keyboard Shortcuts, F1). Conventional database-IDE bindings; platform-specific on macOS vs Windows/Linux |
| **Documents** | **Tree** (default), Table and JSON views with exact BSON types. **Double-click a value to edit just that field** (`$set` / `$unset` on its path, type kept, concurrent changes detected; Enter saves, Esc cancels, Tab moves); undo/redo of field edits; "Edit document (JSON)" for whole documents. Insert one document or an array; delete one or many, with confirmation |
| **Query editor** | Filter, projection, sort, skip and limit in mongosh syntax. Autocomplete for operators and sampled field names. Server-side paging, on-demand count, cancel, "Copy as shell" |
| **Visual query builder** | Conditions with AND/OR, 11 operators, typed values (auto, string, number, boolean, null, ObjectId, date), sort and projection. Writes into the query editor |
| **Aggregation editor** | Stages built from 19 operator templates. Edit, enable/disable, reorder, duplicate and delete stages; preview the output of any stage; run the full pipeline with paging and `allowDiskUse`; edit the whole pipeline as text |
| **Shell** | `show dbs/collections`, `use`, `db.<coll>.<method>(…)` with cursor modifiers, `db.runCommand`, … Run all or just the selection, with per-statement output. Three engines: *Built-in* (paged, editable results), *JavaScript* (variables, loops, functions in an embedded engine, no mongosh needed) and *mongosh* (the installed mongosh, output streamed; the password never on the command line) |
| **Robustness** | Typed errors with friendly messages, per-operation timeout and cancel, cursors always closed, credentials redacted from logs, loading states everywhere |
| **Import wizard** *(P1)* | JSON array, NDJSON/mongoexport and CSV/TSV. Preview the first 20 documents; per-column CSV types (auto, string, int, long, double, decimal, bool, date, ObjectId, JSON) or skip; insert or upsert by `_id`; optional drop-first and stop-on-error; batched writes |
| **Export wizard** *(P1)* | Filter, sort, skip and limit; pick fields from a sampled list; JSON array or NDJSON, canonical or relaxed Extended JSON, pretty-print; CSV with delimiter and header options. Written to a temp file and renamed on success, so failed or canceled exports leave nothing behind |
| **Background jobs** *(P1)* | Imports and exports run off the UI thread with live progress (bytes or documents), elapsed time, result summary, meaningful errors and Cancel, all in the status-bar Jobs panel |
| **Schema explorer** *(P1)* | `$sample`-based analysis (optional filter): every field path incl. inside arrays of documents, presence %, BSON-type distribution, array element types, numeric/date ranges, string and array lengths, distinct counts, unique hints, sample values |
| **SQL query** *(P1)* | A documented SQL `SELECT` subset translated to `find` or an aggregation pipeline. Shows the generated mongosh, runs with paging, positioned errors for unsupported constructs ([reference](#sql-subset)) |
| **Query → code** *(P1)* | Generates runnable code for mongosh, Node.js, Python (PyMongo), Java (sync driver), C# (.NET driver) and Go (driver v2) from a query, pipeline or SQL, with each driver's native BSON types |
| **Explain plans** *(P1)* | Runs `explain` with `executionStats` (pipelines with `$out`/`$merge` are only planned). Shows summary metrics, a stage tree with per-stage stats, aggregation stages, and diagnostics such as collection scans, in-memory sorts, low selectivity, unanchored regexes and covered queries, with ESR index suggestions. *Tree* draws the plan as boxes (collection scans red); *Compare* shows two plans side by side, also from the index advisor after creating an index |
| **Library & history** *(P1)* | Save queries, pipelines, shell scripts and SQL by name; rename, delete and reopen them in the right tab. Every run is recorded in a searchable history (500 entries) with duration and errors. Both persist across restarts |
| **Shell assist** *(P1)* | Context-aware completion: collections after `db.`, collection methods with signatures, cursor methods after `).`, `show` targets, operators, BSON helpers, and the referenced collection's field names. Alt+↑/↓ recalls history; a side panel lists past scripts; scripts can be saved |
| **Compare & sync** *(P2)* | Compare collections or whole databases across connections. Optional filter and ignored fields; field-level diffs; repeatable fingerprint. Sync previews inserts, replacements and deletes. Applying needs explicit confirmation, skips documents that changed since the preview, backs up before-images, and verifies the result |
| **SQL migration** *(P2)* | PostgreSQL, MySQL/MariaDB and SQLite with a `SELECT`-only user. Table → collection mapping with type suggestions, primary key or ObjectId `_id`, and child rows embedded as arrays. Runs in a consistent read-only snapshot, then checks counts and sampled rows. Every run can be rolled back |
| **Reschema** *(P2)* | Rename, remove, set, copy, convert type, flatten and nest fields. Dry-run with before/after samples, then copy to a new collection or apply in place (backed up, confirmed), then validate |
| **Data masking** *(P2)* | Redact, partial, keyed hash, email, randomize, date shift, numeric noise, null/remove/constant. Reusable rule sets; masked copies for test environments; the masked output is checked to contain none of the original values |
| **Scheduled tasks** *(P2)* | Exports, comparisons, syncs, masked copies, backups and **multi-step tasks** on cron or interval schedules. Task definitions can be exported and imported; imported tasks start unapproved. They run only after you approve them, and editing a task revokes approval. Includes retries with backoff, overlap protection, a misfire policy and run history |
| **AI assistant** *(P2)* | "Ask AI" in the query and aggregation editors (Anthropic Claude, off by default). Only field names and types are sent, never credentials. Proposals are validated by the server before you see them; writes need typed confirmation and are backed up |
| **MCP server** *(P2)* | `mango3s mcp` exposes approved, read-mostly database tools to MCP clients. Access is deny-by-default: connection, database, tool and size allowlists; results can be masked; every call can be audited |
| **Before-images** *(P2)* | Every P2 write first saves the affected documents to `backups/`. Library → Backups restores them |
| **Backup & restore** | Full logical backups of databases, collections and views in the mongodump folder layout (raw BSON, gzip, indexes, options, validators), streamed and working through SSH and proxies. Restore with keep / replace-by-`_id` / drop modes, renaming and count checks. `.archive` files, users and roles, and the oplog through mongodump/mongorestore (credentials never on the command line). Live job log. See [docs/FEATURES.md](FEATURES.md) |
| **Database administration** | Create collections with options (capped, time series, clustered, collation, validator) and views; rename collections and databases; clear; copy collections or databases within or across connections; validation rules with a check against existing documents; index manager (create, hide, TTL, drop, size and usage); statistics; query profiler; **index advisor** (suggests indexes for slow queries, finds unused and redundant indexes) |
| **Connection import/export** | Export connections with folders, without secrets unless you opt in (then passphrase-encrypted). Import from Mango3S files, MongoDB Compass exports and lists of connection strings, with conflict handling. Copy a connection string without its password |
| **MongoDB → SQL** | Export a collection as a PostgreSQL, MySQL or SQLite script: typed `CREATE TABLE` and batched `INSERT`s; data that doesn't fit the columns is kept in an `_extra` JSON column |
| **Updates & announcements** | Mango3S checks in with its update server by itself (no account or key — it's free): every 10 minutes while open it looks for updates, downloads and checksum-verifies them, and asks before installing (Settings → Updates; automatic checks on by default), and sends an activity heartbeat. Announcements from the Mango3S team appear as dialogs with their own buttons (open a link, OK, accept, decline) plus a close; the answer is reported back. When a new privacy policy needs accepting, the app asks once per launch ("Later" is always possible — local work is never blocked). About shows the website, privacy and contact links and, when offered, a donation link. Entirely best-effort: if the server is unreachable, requests are retried in the background and nothing in the app is affected. If an admin pauses updates for an installation, the status bar says so once and everything else keeps working |
| **Account & sync** *(optional)* | Settings → Account & sync: create an account (email + 6-digit code), sign in, change or reset the password, and sync connections between computers. A synced connection is a complete copy — every setting, password and passphrase, the SSH private key, TLS certificate/key and CA files, and the trusted SSH server keys — sealed to the server's transport key before it leaves the computer and stored encrypted so only your account password or recovery key can read it. Push and Pull are manual by default (Pull asks for the password); turn on Automatic to push a few seconds after a connection changes. Pick which connections sync; unticked ones stay on that computer. Deletions travel too. The recovery key is shown once after sign-up. Features the server has turned off are hidden. Nothing in the app needs an account |
| **Settings** | Tabbed: General, AI assistant, MCP server, Keymap, Account & sync, Updates, Diagnostics. Unsaved edits are kept while switching tabs |
| **Server monitor** | Live operations, network, connections, queues and cache per second; replica set members with lag and the oplog window; running operations with *Kill* (typed confirmation) |
| **Change streams** | *Watch changes* on a connection, database or collection: live inserts, updates (changed fields) and deletes, filter, pause and resume from the token, export |
| **GridFS** | *GridFS buckets* folder in the explorer: browse, upload, download, preview, rename, delete |
| **Charts** | Collection → *Chart* view (bar, line, area, pie, scatter, time series), grouped on the server over every matching document; saved to the Library |
| **Test data** | Collection → *Generate test data…* from a template (`$name`, `$int(18,90)`, `$date(…)`…) or the schema, repeatable with a seed |
| **Sharded clusters** | Shard key and chunks per shard, primary shard and balancer, connect to a single shard, shard keys kept through backup, restore and copy |
| **Validation from schema** | Optional (Settings → General): build `$jsonSchema` rules from the sampled schema, check them, save |
| **Mango** | An animated mascot at the bottom of the explorer: floats, blinks, follows the pointer, cheers on success and looks worried at errors, dozes off when idle, and shares a tip when clicked. Hide it with its × or Settings → General; motion stops under "reduce motion" |
| **Syntax highlighting** | One palette for every editor and code view, in light and dark. Generated code is highlighted per language (mongosh, Node.js, Python, Java, C#, Go); strings are orange |

### Value syntax

Every editor (filter, document, stage, shell) accepts the same syntax: mongosh literals and
Extended JSON.

```js
{
  _id: ObjectId("65a1f0c2e4b0a1b2c3d4e5f6"),
  name: 'single or "double" quotes',      // comments and trailing commas are fine
  createdAt: ISODate("2024-01-31T12:00:00Z"),
  big: NumberLong("9007199254740993"), qty: NumberInt(3), price: NumberDecimal("9.99"),
  ref: UUID("123e4567-e89b-12d3-a456-426614174000"),
  pattern: /^ab/i,
  legacy: { "$oid": "65a1f0c2e4b0a1b2c3d4e5f6" }  // Extended JSON works too
}
```

Documents are displayed in the same syntax and round-trip exactly. `int32`, `int64`, `double`
and `decimal` stay distinct, so opening and saving a document never changes a field's type.

---

## P1 reference

### Import and export formats

| Format | Import | Export | BSON types |
|---|---|---|---|
| JSON array `[ {...}, ... ]` | ✓ | ✓ | Exact with canonical Extended JSON; relaxed folds int64/double into plain numbers |
| NDJSON / JSON lines / mongoexport | ✓ | ✓ (mongoimport-compatible) | Same as above |
| CSV / TSV (header row required) | ✓ | ✓ | Lossy: types are inferred or set per column; dotted headers ↔ nested documents; arrays and sub-documents as JSON cells |

CSV `auto` inference: `true`/`false` → bool, integers → int32 or int64, decimals → double,
24-hex → ObjectId, full ISO timestamps → date, `[...]`/`{...}` → parsed value, everything else →
string. Bare dates like `2024-01-31` stay strings unless the column type is `date`. Empty cells
are omitted unless "keep blanks" is set.

Imports read with a streaming decoder and write batches of 1,000 documents (unordered). In insert
mode, duplicates and other per-document failures are counted and reported (first 20 messages),
and the rest of the batch is still written. A malformed JSON document stops the import and
reports its position (`document 1203 (byte 98231)`); CSV rows that fail conversion are skipped,
or stop the import when "stop on error" is set.

### SQL subset

```sql
SELECT [DISTINCT] * | field [AS alias], ... | COUNT(*) | COUNT(f) | SUM(f) | AVG(f) | MIN(f) | MAX(f)
FROM collection
[WHERE condition]
[GROUP BY field, ...] [HAVING condition]
[ORDER BY field | alias | aggregate [ASC|DESC], ...]
[LIMIT n [OFFSET m] | LIMIT m, n]
```

- **Conditions:** `= != <> < <= > >=`, `AND`, `OR`, `NOT`, parentheses, `IN (...)`,
  `NOT IN`, `LIKE` / `ILIKE` with `%` and `_`, `IS [NOT] NULL`, `[NOT] BETWEEN a AND b`.
  Field-to-field comparisons become `$expr`.
- **Values:** numbers, `'strings'` (`''` escapes a quote), `TRUE`, `FALSE`, `OBJECTID('…')`,
  `DATE '2024-01-31'` / `TIMESTAMP('…')`.
- **Names:** `address.city` for nested fields; `"odd name"` or `` `odd name` `` for anything else.
- **Translation:** without aggregates or `DISTINCT`, a `find` with filter, projection (aliases
  use `$field` projections), sort, skip and limit. Otherwise a pipeline:
  `$match → $group → $match (HAVING) → $project → $sort → $skip → $limit`.
- **Rejected with a positioned error:** `JOIN`, subqueries, `UNION`/`INTERSECT`/`EXCEPT`, `WITH`,
  `CASE`, functions other than the five aggregates, arithmetic, table aliases, and any
  non-`SELECT` statement.
- **Semantics to know:** `= NULL` is rejected in favor of `IS NULL`. `IS NULL` also matches
  missing fields (MongoDB semantics). `LIKE` is case-sensitive; use `ILIKE` for case-insensitive
  matching.

### Code generation targets

| Target | Output |
|---|---|
| mongosh | `use(...)` + `db.coll.find(...)`/`.aggregate(...)` |
| Node.js | ES module, `mongodb` package, `for await` over the cursor, only the BSON imports it needs |
| Python | PyMongo with `bson` types (`Int64`, `Decimal128`, `Regex`, timezone-aware `datetime`, `Binary.from_uuid`) |
| Java | Sync driver, `Document`/`append`, `Arrays.asList`, `Decimal128.parse`, `Date.from(Instant.parse(...))` |
| C# | .NET driver, `BsonDocument` initializers, `PipelineDefinition` for aggregations |
| Go | Driver v2, `bson.D{{Key, Value}}`, helper funcs for ObjectId/Decimal128, `time.Date(...)` |

All snippets read the connection string from `MONGODB_URI`. The Go output is compiled
(`go vet`) against the real driver in the test suite, and the JavaScript and Python outputs are
syntax-checked with `node --check` and `py_compile` when those tools are installed. Java and C#
are covered by golden-output tests (compiling them would need their driver packages).

---

## Getting started

### Prerequisites

| Tool | Version |
|---|---|
| Go | 1.25+ (`go.mod` sets 1.25; with `GOTOOLCHAIN=auto`, older Go downloads it automatically) |
| Node.js | 20+ (tested with 22 and 23) |
| Wails CLI | v2.10+ — `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| Platform deps | macOS: Xcode command-line tools · Windows: WebView2 (preinstalled on Windows 11) · Linux: `libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config` |
| Docker (optional) | For the sample MongoDB, integration tests and Linux builds |

Run `wails doctor` to check your system.

### Run

```bash
git clone <repo> && cd Mango3S
make mongo-up        # optional: MongoDB 8 with a seeded sample_store database on :27017
make dev             # = wails dev — hot-reloading desktop app
```

In the app, click **New connection** and enter `mongodb://localhost:27017` → **Save & connect**.
Then open `sample_store › orders` in the explorer.

> If port 27017 is already in use, run `MONGO_PORT=27019 docker compose up -d mongo` and connect to
> `mongodb://localhost:27019`.

### Build

```bash
make build           # = wails build -clean → build/bin/Mango3S.app (macOS) / mango3s.exe / mango3s
make docker-build    # Linux binary built and tested inside Docker → ./dist/mango3s
```

Set the version string with
`wails build -ldflags "-X mango3s/internal/api.Version=1.2.3"`. For a Windows installer, add
`-nsis` (requires NSIS).

---

## Testing

```bash
make test               # go vet + Go unit tests + frontend tests (vitest) + TypeScript typecheck
make mongo-up
make test-integration   # end-to-end against MongoDB (MONGO_URI, default mongodb://localhost:27017)
make docker-test        # the same integration suite, entirely inside Docker
make sql-up && make test-integration-sql   # adds the PostgreSQL and MySQL migration tests
```

| Suite | Covers |
|---|---|
| `internal/mql` | Parser and formatter: every BSON type, Extended JSON wrappers, error positions, lossless round trip |
| `internal/config`, `internal/secrets`, `internal/logging` | Profile CRUD, atomic 0600 writes, URI credential splitting, AES-GCM store (no plaintext on disk, ciphertexts bound to their entry), log redaction |
| `internal/api` | Password lands in the secret store and never in `connections.json` or logs; JSON error encoding; cancellation; superseded operations; missing-secret handling |
| `internal/{query,aggregate,documents,shell,model,mongox,apperr}` | Paging arithmetic, pipeline building and validation, `_id` immutability and literal `_id` matching, shell grammar, client options, unreachable-server classification |
| `internal/transfer` | JSON array/NDJSON/canonical/relaxed round trips over a mixed-type dataset, BOMs, malformed input positions, CSV nesting/types/blanks/TSV, CSV export → import round trip, cell formatting, type inference, previews |
| `internal/jobs` | Progress, throttled events, failure and panic capture, cancel running and queued jobs, pruning, shutdown (run with `-race`) |
| `internal/schema` | Presence, type mix, numeric/date/decimal ranges, array element types, fields inside arrays of documents, cardinality caps, ordering |
| `internal/sqlq` | 23 find translations, 5 aggregate translations (GROUP BY/HAVING/DISTINCT/ORDER BY aggregates), 27 rejected constructs with messages, error positions, LIKE → regex |
| `internal/codegen` | Type rendering in all six languages, aggregation output, balanced delimiters; Go output compiled against the driver; JS/Python syntax-checked |
| `internal/explain` | Classic, slot-based (SBE), aggregation `$cursor`, sharded and unknown explain shapes; diagnostics; ESR index suggestions; regex detection |
| `internal/library` | Saved items and history persist across restarts; validation; de-duplicated re-runs; search, limit, clear |
| `internal/integration` (tag `integration`) | Real-server CRUD with type fidelity, duplicate key → CONFLICT, paging with skip/limit, aggregation incl. stage preview and `$out`, shell scripts, read-only enforcement, cancel/timeout, the full API facade, authentication (set `MONGO_AUTH_URI`). **P1:** 2,500-document export→import round trips (canonical array, canonical NDJSON, relaxed), filtered CSV export of nested fields → re-import, insert/upsert/stop-on-error/drop-first, canceled export leaves no file, schema accuracy on known data, explain before/after creating an index, `$out` not executed by explain, SQL find + GROUP BY/HAVING, code from SQL, history and library after restart |
| **P2** `internal/{plans,backup,compare,syncer,docpath,reschema,masking,transform,migrate,scheduler,ai,mcpserver}` | Single-use plans; backup/restore round trips; order-independent fingerprints; drift-safe sync; every reschema op and type conversion; every masking method, determinism and leak checks; SQL type mapping per dialect; cron parsing, approval, retries, interrupted and missed runs; AI proposal validation and prompt contents; MCP deny-by-default, operator refusal, caps and audit logging |
| `internal/{dump,manage,connio,sqlexport}` | Backup/restore round trips with byte-identical documents, indexes, options and views (plain and gzip); compatibility with mongodump and mongorestore in both directions; truncated files detected; copy across databases; collection options, validation, indexes, renames, clear, profiler against a real server; connection files with sealed secrets, Compass and URI-list parsing; SQL scripts loaded into real SQLite, PostgreSQL and MySQL |
| `internal/integration/advanced_test.go` | Multi-step task (export + backup) through the API; task import resets approval; mongodump/mongorestore with authentication, password absent from the job log |
| **P2** `internal/integration/p2_test.go` | Repeatable compare; sync preview == apply, then restore; reschema/masking integrity; migrations from SQLite, PostgreSQL and MySQL with read-only users; scheduled export through the API; AI proposals gated (fake provider); MCP tools against a real server. Acceptance criteria are mapped to tests in [docs/P2.md](P2.md#tests) |
| Roadmap phases 1–6 `internal/{charts,datagen,gridfs,jsengine,monitor,scriptrun,streams}`, `internal/manage/sharding.go`, `internal/schema/jsonschema.go` | Chart pipelines; template generators and seeds; GridFS bucket detection; JavaScript ↔ BSON conversion; monitor rates and replica lag; mongosh connection strings; change events. Real-server tests in `internal/integration/{charts,datagen,gridfs,jsengine,monitor,scriptrun,sharding,streams}_test.go` — the replica set (`MONGO_RS_URI`, `make rs-up`) and sharded cluster (`MONGO_SHARDED_URI`, `make sharded-up`) tests skip when those servers aren't running |
| Roadmap 1.3, 1.4, 2.2, 5.2, 5.3, 6.1 `internal/{users,notify}`, `internal/transfer/excel.go`, `internal/syncer/schema.go`, `internal/manage/search.go` | Role diffs and resource matching; webhook delivery and desktop notifications with text passed as data; Excel round trip of every type; index and validator diffs. Real-server tests: `users_test.go` (auth server), `search_test.go` (Atlas local), `TestExportAggregationResults`, `TestExcelExportImport`, `TestSyncByMatchKey`, `TestSyncIndexesAndValidation` |
| `frontend/src/**/*.test.ts(x)` | Query builder, pipeline and tab reducers, Extended JSON cell rendering, error parsing, results view (paging, selection, read-only), builder UI, full-app connect → browse → query flow. **P1:** shell completion rules, history recall, library → tab mapping, job progress and cancel in the status bar, library tab |

To run the authentication, users and roles test:
`docker compose --profile auth up -d mongo-auth`, then
`MONGO_AUTH_URI='mongodb://admin:change-me@localhost:27018/?authSource=admin' make test-integration`.
The Atlas Search test needs a local Atlas deployment: `make search-up`, then
`MONGO_SEARCH_URI='mongodb://localhost:27022/?directConnection=true' make test-integration`.
The suite runs on MongoDB 4.4 and later; run it against both an old and a current server
before a release.

---

## Architecture

```
┌──────────────────────── React + TypeScript (frontend/src) ───────────────────────┐
│ views/   Collection · Aggregation · Shell · Schema · SQL · Library · Settings      │
│          Compare · Transform (reschema/mask) · Migrate · Tasks              (P2)  │
│ components/  Sidebar · ConnectionManager · ResultsView · QueryBuilder · CodeEditor│
│              ImportWizard · ExportWizard · JobsPanel · ExplainDialog · CodeDialog  │
│              AIDialog · P2Settings (AI, scheduler, MCP) · p2common         (P2)   │
│ state/AppContext (settings, profiles, sessions, tabs, jobs, toasts, dialogs)      │
│ lib/  backend.ts (typed bindings, job events) · queryBuilder · pipeline ·         │
│       shellComplete · library                                                     │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                 Wails bindings │ (frontend/wailsjs, generated)
┌───────────────────────────────▼──────────────────────────────────────────────────┐
│ internal/api      Facade bound to the UI: timeouts, cancellation registry,        │
│                   profile/credential workflow, JSON error encoding, history       │
│                   recording, "jobs:update" events                                 │
├──────────────┬──────────────┬──────────────┬───────────────┬─────────────────────┤
│ explorer     │ documents    │ query        │ aggregate     │ shell               │
│ dbs, colls,  │ CRUD by _id  │ find + paging│ stages →      │ mongosh-subset      │
│ stats, fields│              │ count        │ pipeline      │ parser + executor   │
├──────────────┬──────────────┬──────────────┬───────────────┬─────────────────────┤
│ transfer     │ schema       │ sqlq         │ explain       │ codegen             │
│ JSON/CSV     │ $sample      │ SQL → find / │ plan tree +   │ 6 languages,        │
│ adapters,    │ analyzer     │ pipeline     │ diagnostics   │ native BSON types   │
│ import/export│              │              │               │                     │
├──────────────┴──────────────┴──────────────┴───────────────┴─────────────────────┤
│ compare · syncer · reschema · masking · transform · migrate · scheduler · ai      │
│ mcpserver (`mango3s mcp`) — on plans (single-use previews) + backup (restore) (P2)│
├───────────────────────────────────────────────────────────────────────────────────┤
│ jobs     background runner: cancel, throttled progress   library  saved + history │
├───────────────────────────────────────────────────────────────────────────────────┤
│ mongox   connection manager (one client per open profile) · Source interface      │
│ mql      shared value model: parse/format shell syntax ⇄ BSON (order-preserving)  │
│ model    Document/Page DTOs, paging math       apperr  typed, user-facing errors  │
├───────────────────────────────────────────────────────────────────────────────────┤
│ config (settings.json, connections.json) · secrets (keychain | AES-GCM file)      │
│ logging (slog JSON file + stderr, redacting handler)                              │
└───────────────────────────────────────────────────────────────────────────────────┘
```

**Key decisions**

- **Services behind interfaces.** Each feature package exposes a `Service` interface and depends
  only on `mongox.Source` (connection ID → client session), not on Wails or on storage. The API
  facade wires them together. This keeps services testable and reusable, for example by a future
  CLI.
- **One shared value language (`mql`).** The query editor, document editor, aggregation stages,
  visual builder output and shell all go through one order-preserving parser into `bson.D`, and
  documents come back through the matching formatter. This removes a whole class of "works in one
  editor but not another" bugs and keeps BSON types exact.
- **Safe document identity.** Edits and deletes address documents by their exact `_id`, using
  `$eq`, so an `_id` that is itself a regex is never pattern-matched. Replace refuses to change
  `_id` and reports NOT_FOUND rather than silently upserting when the document disappeared.
- **Cancellation and resource safety.** Every UI call runs with a context that has the configured
  timeout. Long operations register under an operation ID the UI can cancel, and re-running
  supersedes the previous run. Cursors are always closed with a fresh context, and pages are
  fetched as `limit + 1` to learn whether more exist, so whole result sets are never loaded.
- **Errors are data.** Every backend error becomes an `apperr.Error{code, message, detail}`,
  serialized as JSON across the bridge. The UI shows a friendly title per code, with the driver
  detail expandable.
- **P1 reuses P0 services, it doesn't fork them.** SQL execution feeds its translation into the
  existing query and aggregate services; export reuses `query.Parse`; explain and code generation
  reuse `aggregate.Build`. So paging, timeouts, read-only checks and the value parser behave the
  same everywhere.
- **Long work is a job, not a call.** Imports and exports outlive a single UI request. The `jobs`
  manager gives each one its own cancellable context, a concurrency limit (two at a time, the
  rest queue), panic recovery and progress updates throttled to about 7 per second. These are
  pushed to the UI as `jobs:update` events, so the desktop UI never blocks.
- **Streaming everywhere.** Readers yield one document at a time and writers consume one at a
  time; imports batch writes and exports iterate a cursor. Memory use is independent of
  dataset size.

### Repository layout

```
main.go                      Wails bootstrap (window, lifecycle, dependency wiring)
internal/
  api/                       UI facade (bound methods) + tests
  apperr/ config/ logging/ secrets/ model/ mql/ mongox/
  explorer/ documents/ query/ aggregate/ shell/
  jobs/ transfer/ schema/ sqlq/ codegen/ explain/ library/     P1 features
  dump/ manage/ connio/ sqlexport/                             backup/restore/copy, administration, connection files, SQL export
  integration/               end-to-end tests (build tag: integration)
frontend/
  src/{lib,components,views,state}/   React app (+ *.test.ts[x])
  wailsjs/                   generated bindings (regenerate: make bindings)
build/                       Wails platform assets (icons, Info.plist, Windows manifest)
config/                      sample settings and connection profiles
docker/mongo-init/           sample data seeded into the compose MongoDB
Dockerfile, docker-compose.yml, Makefile
```

---

## Configuration

Mango3S stores its state in the user config directory: `~/Library/Application Support/Mango3S`
on macOS, `%AppData%\Mango3S` on Windows, `~/.config/Mango3S` on Linux. Override it with
`MANGO3S_CONFIG_DIR` for portable installs.

| File | Contents |
|---|---|
| `settings.json` | Theme, page size, operation timeout, delete confirmation, default view, editor font size, log level ([sample](config/sample-settings.json)) |
| `connections.json` | Connection profiles **without passwords** ([sample](config/sample-connections.json)) |
| `logs/mango3s.log` | JSON logs, rotated at 5 MB |
| `secrets.enc`, `secret.key` | Only when the file secret backend is used |
| `library.json` | Saved queries, pipelines, scripts and SQL |
| `history.json` | The last 500 executions (queries, pipelines, scripts, SQL) |
| `mcp.json` | MCP server policy ([sample](config/sample-mcp.json)) |
| `masking.json` | Masking rule sets ([sample](config/sample-masking.json)) |
| `tasks.json`, `task_runs.json` | Scheduled tasks ([sample](config/sample-tasks.json)) and their last 500 runs |
| `migrations.json` | SQL migration runs and reports, without DSNs ([request sample](config/sample-migration.json)) |
| `backups/*.ndjson` | Before-images of documents changed by sync, in-place transforms, AI and MCP writes (0600) |

To start from the samples, copy `config/sample-*.json` into the config directory as
`settings.json` and `connections.json`. Then enter passwords in the connection dialog; the
samples contain none.

| Environment variable | Purpose |
|---|---|
| `MANGO3S_CONFIG_DIR` | Config directory override |
| `MANGO3S_SECRET_BACKEND` | `keyring` (OS store), `file` (encrypted file), `memory` (tests). Default: keyring if available, else file |
| `ANTHROPIC_API_KEY` | AI assistant key when none is stored in Settings → AI |
| `MANGO3S_PLATFORM_URL` | Update server to use instead of the built-in one, e.g. `http://localhost:4330` for a local web platform |
| `MONGO_URI`, `MONGO_AUTH_URI`, `PG_DSN`, `MYSQL_DSN` | Integration tests only |

The update server's address is `PlatformBaseURL` in `internal/api/update.go`
(`https://mango3s.com`). `make build`/`make dev` embed it and the app version at build time
(`PLATFORM_BASE_URL` and `VERSION` Makefile variables, defaulting to that URL and `wails.json`'s
`productVersion`) — override with `make build VERSION=1.2.3 PLATFORM_BASE_URL=https://...`, or at
runtime with `MANGO3S_PLATFORM_URL`.

### Credential handling

- When you save a profile, any `user:password@` in the URI is split off. The password goes to the
  OS credential store (macOS Keychain, Windows Credential Manager, Secret Service), and only the
  stripped URI and the username are written to `connections.json` (mode 0600).
- Where no OS store exists (containers, headless Linux), passwords are stored AES-256-GCM
  encrypted in `secrets.enc`, with the key in `secret.key` (0600). This guards against casual
  disclosure (backups, screen sharing, accidental commits) but **not** against someone who can
  read your user account's files. The Settings page shows which backend is active.
- Passwords are never returned to the UI. The form shows "(saved)" and lets you replace or remove
  the password.
- Logs pass through a redacting handler that masks URI user info and
  `password=`/`"password":` values, so neither driver errors nor URIs can leak credentials.
- The installation token the app receives when it registers with the update server goes through
  the same credential store as connection passwords.

---

## Docker

| Command | What it does |
|---|---|
| `docker compose up -d mongo` | MongoDB 8 on `:27017`, seeded with `sample_store` (customers, products, 250 orders) |
| `docker compose --profile sql up -d --wait postgres mysql` | PostgreSQL 17 on `:5433` and MySQL 8.4 on `:3307` with a seeded `shop` schema and read-only `mango_reader` / `reader` users (`make sql-up`) |
| `docker compose --profile auth up -d mongo-auth` | MongoDB with auth on `:27018` (`admin` / `change-me`, see `.env.example`) |
| `docker compose --profile test up --build --exit-code-from integration-tests integration-tests` | Builds the image and runs the integration suite against the compose MongoDB |
| `docker build --target artifact --output type=local,dest=dist .` | Frontend tests + build, Go vet + unit tests, Linux binary to `./dist` |
| `docker compose --profile gui up --build app` | **Experimental, Linux + X11 only**: runs the GUI from a container (`xhost +local:` may be required) |

The image stages are `frontend` → `build` → `integration` / `artifact` / `runtime`. A desktop app
can't run inside a container on macOS or Windows; build it natively with `wails build`.

---

## Assumptions

- MongoDB **4.4+** is supported. Development and testing used 8.0. `$collStats` stats fall back to
  a document count where it is unavailable (views).
- One open connection per profile. Connection ID = profile ID.
- "Edit document" replaces the whole document by `_id`. It is last-write-wins: there is no
  optimistic concurrency check, but the editor reloads the latest version before opening, and a
  document deleted in the meantime is reported, not re-created.
- Shell results are capped at the configured page size per statement (use `.skip()`/`.limit()`).
  Execution stops at the first failing statement.
- The table view renders values from relaxed Extended JSON. 64-bit integers above 2⁵³ may display
  rounded there; the JSON view and the editors are always exact.
- The operation timeout (default 60 s) applies to every backend call, including the shell, SQL,
  schema analysis and explain. Background imports and exports have no timeout; cancel them from
  the Jobs panel.
- Explain uses `executionStats`, so it **runs** the query (writes are excluded: `$out`/`$merge`
  pipelines are explained with `queryPlanner` only). For sharded clusters it shows the first
  shard's plan.
- Schema statistics describe a random sample, not the whole collection; distinct counts are
  capped at 1,000 per field.
- History records page 0 of each query or pipeline run (not page navigation) and skips canceled
  runs. Re-running the newest entry updates it rather than adding a duplicate.
- Job history is in-memory (the last 50 finished jobs); the jobs themselves stop when the app
  exits.

## Not supported (yet)

- **Shell:** the *Built-in* mode is a MongoDB command interpreter (paged results); use the
  *JavaScript* mode (embedded engine) or the *mongosh* mode for variables, loops and functions.
  No `load()` in the JavaScript mode; mongosh mode can't go through SSH tunnels or proxies.
- **Import/export:** an Excel import reads one sheet, an Excel export writes one (at most
  1,048,575 rows). Multi-collection and BSON exports are covered by backups; see [docs/FEATURES.md](FEATURES.md#limits)
  for the limits of backup, restore, copy and SQL export.
- **SQL:** joins, subqueries, expressions and functions beyond `COUNT/SUM/AVG/MIN/MAX`, and
  `INSERT/UPDATE/DELETE` (see [SQL subset](#sql-subset)).
- **Code generation:** languages beyond the six listed; generated snippets contain the query
  only, not connection options such as TLS settings.
- **P2 features:** see [Unsupported capabilities](P2.md#unsupported-capabilities) for the
  limits of compare/sync, migration, reschema, masking, the scheduler, AI and MCP.
- **Connection, editing and shortcut limits:** see [Unsupported and limitations](UI.md#unsupported-and-limitations)
  (Kerberos, interactive OIDC, SSH keyboard-interactive 2FA, SOCKS4, socket timeout and wtimeout
  in Go driver v2, keymap customization, …).
- Multiple windows, and client-side field-level encryption.
- Code signing and notarization of release builds.

## Troubleshooting

| Symptom | Fix |
|---|---|
| "Server unreachable" | Check host/port and firewall. For a single node of a replica set, add `directConnection=true`. For Atlas, allow your IP |
| "Authentication failed" | Check username, password and **Auth database** (usually `admin`) |
| "The saved password … is missing" | The OS credential store entry was removed; edit the connection and enter the password again |
| Linux: app won't start | Install `libwebkit2gtk-4.1-0 libgtk-3-0`. When building with Wails yourself, pass `-tags webkit2_41` |
| `npm install` fails with `edgesOut` | Keep `frontend/.npmrc` (`legacy-peer-deps=true`); it works around an npm peer-resolution bug |

Logs are in `<config dir>/logs/mango3s.log`. Set the log level to Debug in Settings for
per-operation timings.

## License

Mango3S — Copyright © 2026 saman.

This program is free software: you can redistribute it and/or modify it under the terms of the
GNU General Public License as published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version. It is distributed WITHOUT ANY WARRANTY; see
[LICENSE](../LICENSE) for details.

Mango3S includes open-source components under their own (GPL-compatible) licenses: MIT, BSD,
Apache-2.0 and MPL-2.0. The full list is in About Mango3S → License, generated from the actual
dependencies by `make notices` (`scripts/third-party-notices.sh`); rerun it after changing
dependencies.
