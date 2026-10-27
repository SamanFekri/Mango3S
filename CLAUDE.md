# Mango3S — notes for Claude

A free desktop IDE for MongoDB: Go backend (Wails v2) + React/TypeScript frontend.
Go module `mango3s`; the UI calls Go through generated bindings.

## Layout

- `main.go`, `platformcheck.go`, `platformapi.go` — app entry and the `platform-check` / `platform-api` CLIs.
- `internal/api` — the Wails-bound `App`: every frontend call lands here. Wrap work in
  `call(a, "Op", opID, func(ctx) …)` (timeouts, cancel, error encoding); errors go through `a.encode`.
- `internal/<feature>` — one package per feature (`transfer`, `compare`, `syncer`, `users`, `notify`,
  `manage`, `scheduler`, `shell`, `jsengine`, …). `internal/mongox` owns sessions and privilege checks.
- `frontend/src/lib/backend.ts` — typed wrapper over `frontend/wailsjs`; UI code never imports the
  bindings directly. Types the UI sees live in `frontend/src/lib/types.ts` (kept by hand).
- `frontend/src/views/<name>/`, `frontend/src/components/` — views and dialogs; tests sit next to them (`*.test.tsx`).
- `docs/ROADMAP.md` (tick items when done), `docs/FEATURES.md` (feature matrix), `docs/REFERENCE.md`, `docs/RELEASING.md`.

## Commands

```bash
make dev                 # app with hot reload in test mode (request log, API console)
make test                # go vet + Go unit tests + vitest + typecheck
make bindings            # after changing bound Go methods; then: chmod 644 frontend/wailsjs/runtime/*
make notices             # after adding a Go or npm dependency (About › License)
make mongo-up | rs-up | sharded-up | search-up   # test servers (27017, 27020, 27021, 27022)
MONGO_URI=… make test-integration   # real-server suite (build tag `integration`)
```

Integration env vars: `MONGO_AUTH_URI` (mongodb://admin:change-me@localhost:27018/?authSource=admin),
`MONGO_RS_URI`, `MONGO_SHARDED_URI`, `MONGO_SEARCH_URI` (…:27022/?directConnection=true); tests skip
without them. Go tests on Linux need `-tags webkit2_41`.

## Conventions

- **Support MongoDB 4.4+.** Run the integration suite against 4.4 *and* a current server. 4.4 traps
  already hit: `{$eq: ["$$ROOT", …]}` crashes its optimizer (use `mongox.UnchangedFilter`, which goes
  through `$let`); no `$dateTrunc` (use `$dateFromParts`).
- **Safety rules** (from the roadmap, apply to every feature): destructive actions need confirmation,
  often a typed name; read-only connections change nothing (`sess.RequireWritable`); secrets stay in the
  credential store (`internal/secrets`), never in config files, logs, history or command lines; stream
  data instead of loading it whole; writes re-check what was previewed.
- `apperr.From(nil)` returns a typed nil that is a non-nil `error` — use `apperr.Check(err)` when
  returning a possibly-nil error.
- Parse connection strings with `config.ParseConnString`, not `url.Parse` (multi-host URIs).
- **Frontend style:** single quotes, no semicolons, long lines. There is no Prettier config and the
  code is not Prettier-formatted — don't run Prettier on existing files; match the surrounding code.
- Every feature gets a unit test, a real-server test and a UI test, plus a FEATURES.md row.
- Copy in the UI and docs: plain words, no comparisons to other products.

## Releases and repo

- Builds and releases happen only in GitHub Actions (`build.yml`, `build-platform.yml`,
  `publish-release.yml`; see docs/RELEASING.md). Don't add local release scripts.
- The version lives in `wails.json`, `frontend/package.json`, the Dockerfile, the Makefile and `api.Version`.
- Commit or push only when asked. `brag-output*/` is git-ignored.
- Launch videos (/brag) must not show the SQL feature.
