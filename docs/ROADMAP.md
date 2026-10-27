# Roadmap

Planned features, in six phases. Each phase depends only on the ones before it, and each item can ship on its own. Mark items as they are done:

- `[ ]` to do
- `[~]` in progress
- `[x]` done

**Rules for every item** (the same as for existing features):

- Destructive actions need confirmation; read-only connections can't change anything.
- Secrets stay in the credential store and never reach logs or command lines.
- Data is streamed, never loaded whole into memory.
- Each feature gets unit tests, a test against a real server and UI tests.
- Update [FEATURES.md](FEATURES.md) and [REFERENCE.md](REFERENCE.md).
- Never mention or imply another product as the inspiration.

Sizes:

- S: about a day
- M: a few days
- L: about a week
- XL: more than that

---

## Phase 0: Foundations

Change streams, replication lag and replica set status all need a replica set, and the current test server is a single standalone node. Several later features can't be tested until this phase is done.

- [x] **0.1 CI (M).** Done: `.github/workflows/ci.yml` (unit and integration tests on every push; Linux and Windows builds; macOS on request because of runner cost). GitHub Actions workflow with three jobs:
  - `go vet`, Go unit tests with `-race`, vitest and the TypeScript check;
  - the integration suite against the docker-compose services;
  - `wails build` on macOS, Windows and Linux.
- [x] **0.2 More test servers (M).** New docker-compose profiles:
  - `rs`: a single-node replica set on 27020;
  - `sharded`: a config server, 2 shards and a mongos on 27021.

  Integration suites skip when their server isn't running, as they already do for `MONGO_AUTH_URI`.
- [x] **0.3 Fix outdated docs (S).** `docs/P2.md` still says "no task dependencies or chains" (multi-step tasks now exist) and "Anthropic Claude only; no local models" (local models are now supported).
- [x] **0.4 Split large frontend files (M).** Done: explorer menus in `components/explorer/menus.tsx`, its dialogs in `ExplorerDialogs.tsx` (Sidebar 788 → 573 lines); paging and document editing of `CollectionView` in `views/collection/usePagedQuery.ts` and `useDocumentActions.tsx`.
  - Move the explorer menus and dialog state out of `Sidebar.tsx` (748 lines) into `explorer/menus.ts` and `ExplorerDialogs.tsx`.
  - Move the editor and paging logic out of `CollectionView.tsx` (656 lines) into hooks.
  - No behavior change: the existing tests must still pass.

## Phase 1: Quick wins

- [x] **1.1 Validation rules from the schema explorer (S–M).** Done as an option, off by default (Settings → General).
  - **Backend:** add `schema.ToJSONSchema(report, opts)` to `internal/schema`. It builds a `$jsonSchema` from the observed types and presence:
    - fields present in every sampled document become `required`;
    - mixed types become `bsonType: [..]`;
    - nested documents and arrays are handled recursively.
  - **Options:** the presence threshold for `required`, and whether fields that weren't seen are rejected.
  - **UI:** a "Create validation rules…" button in the Schema tab. It opens the validation editor pre-filled, runs "Check existing documents", then saves.
  - **Tests:** a known dataset gives the expected schema; on a real server, the generated rule accepts every existing document.
- [x] **1.2 Visual explain plan (M).**
  - **Backend:** none needed; `internal/explain` already returns the stage tree with its statistics.
  - **UI:** an SVG tree with one box per stage, showing documents and keys examined and time. Collection scans are highlighted.
  - **Compare plans:** run explain twice (for example before and after an index) and show the two plans side by side with the differences.
  - **Index advisor:** offer "Compare plan" after **Create**.
- [x] **1.3 Export aggregation results (S).** Done: `ExportOptions.Pipeline`, *Export results…* in the aggregation editor.
  - **Backend:** add an optional pipeline to `transfer.ExportOptions`. When it is set, `RunExport` streams the aggregation cursor instead of a find; writing to the file is unchanged.
  - **UI:** an "Export results…" button in the aggregation editor.
  - **Tests:** export a pipeline result, then import it.
- [x] **1.4 Notifications for scheduled tasks (S–M).** Done: `internal/notify`, per-task settings, webhook URL in the credential store.
  - **Webhook:** set per task. Mango3S POSTs JSON on failure, or on every run if chosen. The URL is stored as a secret.
  - **Desktop notifications:** Wails v2 has no notification API, so they go through the OS:
    - macOS: `osascript`;
    - Linux: `notify-send`;
    - Windows: a PowerShell toast.
  - **Email:** not planned unless asked for (it needs SMTP settings).

## Phase 2: Operations and administration

- [x] **2.1 Live server monitor (L).** Build it together with 4.1 (charts).
  - **Backend:** new package `internal/monitor`.
    - A sampler job runs `serverStatus` every 1–5 seconds and computes rates: operations per second, network, cache, connections and queues. About 15 minutes of history is kept in memory.
    - On a replica set it also reads `replSetGetStatus`: member state, lag and oplog window.
    - Running operations come from `$currentOp`, filtered by duration, namespace and client.
    - `killOp` stops an operation. It needs typed confirmation, shows the operation first, and is refused on read-only connections.
  - **Events:** `monitor:sample`, pushed only while a monitor tab is open.
  - **UI:** a Monitor tab per connection with small charts, the running-operations table, and replica set members with their lag.
  - **Privileges:** if the user can't run `serverStatus` or see `inprog`, explain why instead of failing.
  - **Tests:**
    - unit tests for the rate calculations;
    - a real server lists and kills a slow `$where` operation;
    - lag is shown on the `rs` test server.
- [x] **2.2 Users and roles (L).** Done: `internal/users`, the *Users and roles* tab, shell helpers; shell history masks passwords.
  - **Backend:** new package `internal/users`.
    - Read with `usersInfo` and `rolesInfo`, including built-in roles and their privileges.
    - Create users and change passwords. A typed password is never logged; a password generator is optional.
    - Grant and revoke roles; create, update and drop custom roles.
    - Dropping a user needs the user name typed.
  - **"What can this user do":** inherited privileges, reusing `mongox/privileges.go`.
  - **UI:** Explorer → connection → "Users and roles…":
    - users per database;
    - roles assigned with checkboxes;
    - a privilege matrix for custom roles.
  - **Safety:** warn before you remove your own `userAdmin` rights or the last admin user.
  - **Shell:** add `db.createUser`, `db.getUsers` and similar commands to the built-in shell.
  - **Tests (on the auth test server):** create a user, grant a role, check the user can log in, revoke, drop.

## Phase 3: Shell

- [x] **3.1 Run scripts with mongosh (M).** Do this first.
  - **Backend:**
    - Run scripts through the detected `mongosh`.
    - Pass credentials in a private temporary file or environment variables, as the backups do; never on the command line.
    - Stream the output into the shell tab; the run can be canceled.
  - **UI:** the shell tab gets a mode switch: "Built-in (paged, editable results)" or "mongosh (full JavaScript)".
  - **Limits:** SSH and proxy connections can't use it (the same limit as mongodump). When mongosh is missing, show how to install it.
  - **Tests:** with a password-protected server, run a script with variables, loops and `print`; the password must not appear in the output.
- [x] **3.2 Built-in JavaScript engine (XL).** Optional, later.
  - Embed `goja` (MIT license) with a `db` object backed by the existing services.
  - Unlike 3.1, it keeps paged, editable results and works without mongosh installed.
  - Only needed if 3.1 isn't enough.

## Phase 4: Working with data

- [x] **4.1 Charts from results (L).** Done with a small built-in SVG chart component instead of uPlot (no new dependency).
  - **UI:** a Chart view next to Tree, Table and JSON.
    - Chart types: bar, line, area, pie, scatter and time series.
    - Pick the X and Y fields from the sampled fields.
    - Aggregate with count, sum, avg, min or max, and group by day, week or month.
  - **Backend:** the grouping runs on the server as a generated `$group` pipeline through `RunAggregation`, so it covers all matching documents, not just one page. No new package.
  - **Library:** charts can be saved with their query.
  - **Library choice:** a small bundled chart library (for example uPlot, MIT license), added to the third-party notices.
  - **Tests:** unit tests for the pipeline builder, UI tests for rendering.
- [x] **4.2 Change stream viewer (M).** Needs a replica set or sharded cluster.
  - **Backend:** `internal/streams`.
    - Watch a collection, a database or the whole deployment, with an optional `$match` filter and the full document.
    - Events go to the UI in batches.
    - Pause and reconnect resume from the last resume token.
    - A tab keeps at most 10,000 events.
  - **UI:** a live list showing operation type, key and changed fields as a diff, with pause, clear and export to JSON.
  - **Standalone server:** explains why change streams aren't available.
  - **Tests (on `rs`):** inserts, updates and deletes appear; pause and resume continues from the token.
- [x] **4.3 GridFS browser (M).**
  - **Backend:** `internal/gridfs`, using the driver's `GridFSBucket`.
    - Find buckets from matching `*.files` / `*.chunks` pairs.
    - List files page by page.
    - Upload and download as streamed jobs with progress.
    - Preview images and text up to a size limit.
    - Delete (confirmed) and rename.
  - **UI:** a "GridFS buckets" folder in the explorer, plus a file browser tab.
  - **Tests:** a 50 MB upload and download comes back byte-identical; deleting a file removes its chunks.
- [x] **4.4 Test data generator (M).**
  - **Backend:** `internal/datagen`.
    - Generate from a schema report, or from a template with generator markers, for example `{ name: $name, email: $email, age: $int(18,90), created: $date("-1y","now") }`.
    - A seed makes the output repeatable.
    - Writes in batches as a background job.
    - Reuses `internal/masking` generators where they overlap.
  - **UI:** collection → "Generate test data…". Preview 5 documents, choose how many, then run.
  - **Safety:** writable connections only; asks for confirmation when the collection isn't empty.
  - **Tests:** the generated types match the template; the same seed gives the same data.

## Phase 5: Scale and cloud

- [x] **5.1 Sharding awareness (L).** Needs the `sharded` test server from 0.2.
  - **Collection info:**
    - the shard key;
    - chunks per shard (from `config.chunks`);
    - jumbo chunks.
  - **Database view:** the primary shard and the balancer state (`balancerStatus`).
  - **Backups:**
    - The backup manifest records each sharded collection's shard key.
    - Restoring onto a sharded cluster can optionally run `shardCollection` first. It asks for confirmation.
    - Copying gets the same option.
  - **Profiler:** offers to connect to individual shards, since profiling runs per shard.
  - **Tests:** back up a sharded collection, restore it, and check it is sharded again with the same key.
- [x] **5.2 Compare and sync improvements (M).** Done: `compare.Options.MatchKey`; `syncer.DiffSchema`/`ApplySchema` with a stale-preview check.
  - Match documents by a custom key, not only `_id`.
  - Sync indexes and validation rules too, after a preview and confirmation. Today they are only reported.
- [x] **5.3 Atlas Search and Vector Search indexes (M).** Done: `manage.*SearchIndex*`, compose profile `search` (`make search-up`, port 27022), CI runs it.
  - **Backend:** list, create, update and drop search indexes (`listSearchIndexes`, `createSearchIndex`, `updateSearchIndex`, `dropSearchIndex`).
  - **UI:**
    - a Search indexes section in the index manager;
    - `$search` and `$vectorSearch` templates in the aggregation editor.
  - **Tests:** add a compose profile with `mongodb/mongodb-atlas-local`.
- [ ] **5.4 Client-side field-level encryption (L).** Deferred until someone needs it.
  - Connection settings for the key vault and KMS provider: local, AWS, Azure or GCP.
  - Key secrets are kept in the credential store.
  - Needs the `mongo_crypt` shared library, a separate download that also changes how the app is built.

## Phase 6: Platform

- [x] **6.1 Excel import and export (M).** Done: `transfer/excel.go`; numbers and dates import from the stored value, so they round-trip exactly.
  - Use `excelize` (BSD-3 license).
  - **Export:** one sheet per collection or query, with typed cells. Values that don't fit a cell are written as Extended JSON.
  - **Import:** the same column mapping as CSV, reusing the type conversion in `transfer`.
  - **Tests:** a round trip covering every supported type.
- [ ] **6.2 Multiple windows (XL).** Deferred.
  - Wails v2 supports only one window, so this needs a migration to Wails v3.
  - Revisit when Wails v3 is stable.
- [x] **6.3 Windows and Linux (M).** Done through CI: the Go suite and the OS credential store run on Windows and Linux; the build workflow makes a Windows installer, a .deb and a .dmg. Builds on the CI from 0.1.
  - Check the shortcuts.
  - Check the credential stores: Windows Credential Manager and the Linux secret service.
  - Check file dialogs and where tools are looked for.
  - Build an installer for each platform.

---

## Recommended order

1. Phase 0
2. 1.1, then 1.3, 1.2 and 1.4
3. 2.1 (monitor) together with 4.1 (charts)
4. 2.2 (users and roles)
5. 3.1 (shell via mongosh)
6. 4.2, 4.3 and 4.4
7. 5.1, 5.2, 6.1 and 6.3
8. Only if needed: 5.3, 5.4, 3.2 and 6.2

## Open decisions

The defaults below apply unless the user decides otherwise.

- **1.4 Notifications:** default is webhook and desktop only, no email.
- **3.1 / 3.2 Shell:** default is that running scripts through mongosh is enough.
- **4.1 Charts:** default is a bundled uPlot.
- **5.4 Encryption and 6.2 multiple windows:** both deferred.
- **0.1 CI:** default is GitHub Actions. Confirm the repository is hosted on GitHub.
