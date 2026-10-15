# Mango3S

A desktop IDE for MongoDB, built with Go, [Wails](https://wails.io) and React. The core workflow
(connection manager, explorer, query editor, visual query builder, aggregation editor and
command shell) is inspired by Studio 3T. Mango3S is an independent, original implementation and
shares no code, branding or assets with that product.

This release covers the **P0 scope**: browsing data, querying, editing documents and running
pipelines and shell commands safely.

---

## Features

| Area | What you get |
|---|---|
| **Shell** | Desktop window with explorer sidebar, draggable tabs (state kept when you switch tabs), status bar, light/dark/system theme, settings page |
| **Connections** | Saved profiles; connection test showing server version, topology and latency; passwords in the OS keychain; read-only profiles; TLS, replica-set and SRV through the connection string |
| **Explorer** | Lazy tree of connections → databases → collections and views, with a filter. Context menus to create or drop databases and collections (drops require typing the name) and to show collection stats and indexes |
| **Documents** | JSON view (syntax-highlighted, round-trip-exact shell syntax) and table view. Insert one document or an array; edit (full replace by `_id`); delete one or many, with confirmation |
| **Query editor** | Filter, projection, sort, skip and limit in mongosh syntax. Autocomplete for operators and sampled field names. Server-side paging, on-demand count, cancel, "Copy as shell" |
| **Visual query builder** | Conditions with AND/OR, 11 operators, typed values (auto, string, number, boolean, null, ObjectId, date), sort and projection. Writes into the query editor |
| **Aggregation editor** | Stages built from 19 operator templates. Edit, enable/disable, reorder, duplicate and delete stages; preview the output of any stage; run the full pipeline with paging and `allowDiskUse`; edit the whole pipeline as text |
| **Shell (IntelliShell-style)** | `show dbs/collections`, `use`, `db.<coll>.<method>(…)` with cursor modifiers, `db.runCommand`, … Run all or just the selection, with per-statement output |
| **Robustness** | Typed errors with friendly messages, per-operation timeout and cancel, cursors always closed, credentials redacted from logs, loading states everywhere |

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
```

| Suite | Covers |
|---|---|
| `internal/mql` | Parser and formatter: every BSON type, Extended JSON wrappers, error positions, lossless round trip |
| `internal/config`, `internal/secrets`, `internal/logging` | Profile CRUD, atomic 0600 writes, URI credential splitting, AES-GCM store (no plaintext on disk, ciphertexts bound to their entry), log redaction |
| `internal/api` | Password lands in the secret store and never in `connections.json` or logs; JSON error encoding; cancellation; superseded operations; missing-secret handling |
| `internal/{query,aggregate,documents,shell,model,mongox,apperr}` | Paging arithmetic, pipeline building and validation, `_id` immutability and literal `_id` matching, shell grammar, client options, unreachable-server classification |
| `internal/integration` (tag `integration`) | Real-server CRUD with type fidelity, duplicate key → CONFLICT, paging with skip/limit, aggregation incl. stage preview and `$out`, shell scripts, read-only enforcement, cancel/timeout, the full API facade, authentication (set `MONGO_AUTH_URI`) |
| `frontend/src/**/*.test.ts(x)` | Query builder, pipeline and tab reducers, Extended JSON cell rendering, error parsing, results view (paging, selection, read-only), builder UI, full-app connect → browse → query flow |

To run the authentication test:
`docker compose --profile auth up -d mongo-auth`, then
`MONGO_AUTH_URI='mongodb://admin:change-me@localhost:27018/?authSource=admin' make test-integration`.

---

## Architecture

```
┌──────────────────────── React + TypeScript (frontend/src) ───────────────────────┐
│ views/   CollectionView · AggregationView · ShellView · SettingsView · Welcome    │
│ components/  Sidebar · ConnectionManager · ResultsView · QueryBuilder · CodeEditor│
│ state/AppContext (settings, profiles, sessions, tabs, toasts, confirm)            │
│ lib/  backend.ts (typed bindings + error normalization) · queryBuilder · pipeline │
└───────────────────────────────┬──────────────────────────────────────────────────┘
                 Wails bindings │ (frontend/wailsjs, generated)
┌───────────────────────────────▼──────────────────────────────────────────────────┐
│ internal/api      Facade bound to the UI: timeouts, cancellation registry,        │
│                   profile/credential workflow, JSON error encoding                │
├──────────────┬──────────────┬──────────────┬───────────────┬─────────────────────┤
│ explorer     │ documents    │ query        │ aggregate     │ shell               │
│ dbs, colls,  │ CRUD by _id  │ find + paging│ stages →      │ mongosh-subset      │
│ stats, fields│              │ count        │ pipeline      │ parser + executor   │
├──────────────┴──────────────┴──────────────┴───────────────┴─────────────────────┤
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

### Repository layout

```
main.go                      Wails bootstrap (window, lifecycle, dependency wiring)
internal/
  api/                       UI facade (bound methods) + tests
  apperr/ config/ logging/ secrets/ model/ mql/ mongox/
  explorer/ documents/ query/ aggregate/ shell/
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

To start from the samples, copy `config/sample-*.json` into the config directory as
`settings.json` and `connections.json`. Then enter passwords in the connection dialog; the
samples contain none.

| Environment variable | Purpose |
|---|---|
| `MANGO3S_CONFIG_DIR` | Config directory override |
| `MANGO3S_SECRET_BACKEND` | `keyring` (OS store), `file` (encrypted file), `memory` (tests). Default: keyring if available, else file |
| `MONGO_URI`, `MONGO_AUTH_URI` | Integration tests only |

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

---

## Docker

| Command | What it does |
|---|---|
| `docker compose up -d mongo` | MongoDB 8 on `:27017`, seeded with `sample_store` (customers, products, 250 orders) |
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
- The operation timeout (default 60 s) applies to every backend call, including the shell.

## Not supported in P0

- **Shell:** this is a MongoDB command interpreter, not a JavaScript engine. No variables, loops,
  functions, `it` paging, `load()`, or `print`. `help` lists what's supported.
- Import/export (CSV, JSON, BSON, SQL), SQL query, schema analysis, explain-plan visualizer
  (`find().explain()` in the shell returns the raw plan), index management UI (use the shell),
  user/role management, data comparison, task scheduler, query history and favorites, SSH
  tunnels, connection-string import from other tools, in-place cell editing in the table view, and
  multiple windows. These are P1/P2 candidates.
- SSH tunnelling and X.509 / Kerberos / LDAP authentication through dedicated form fields. Options
  the Go driver accepts in the connection string (for example `tls=true&tlsCAFile=…`,
  `authMechanism=MONGODB-X509`) still work.
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
