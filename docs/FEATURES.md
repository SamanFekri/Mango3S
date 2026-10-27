# Advanced features: matrix, behavior and limits

This page lists the administration and data-movement features. For each one it gives where to find it, what it depends on, how it is verified, and its limits.

Legend:

- ✅ available
- ◐ available with documented limits
- ✗ not available

## Feature matrix

| Feature | Status | Where | Depends on | Verified by |
|---|---|---|---|---|
| **Backup and restore** | | | | |
| Full logical backup (built-in), as a mongodump-compatible folder | ✅ | Explorer → connection / database / collection → *Back up…* | Go driver only; works through SSH tunnels and proxies | `internal/dump` integration tests: every BSON type is byte-identical after a round trip, indexes (unique, partial, TTL), collation, validator, views, names with `/` and `%`, plain and gzip |
| Restore of built-in or mongodump folders | ✅ | *Restore…* on a connection or database | Go driver only | Same tests, plus mongodump folder → built-in restore and built-in folder → mongorestore |
| Archive backups (`.archive`, gzip) with mongodump | ✅ | *Back up…* → *Archive file (mongodump)* | MongoDB Database Tools 100.x | `TestToolArchiveRoundTrip`, and `TestToolBackupWithAuthentication` against a server with authentication |
| Archive restore with mongorestore | ✅ | *Restore…* → *Archive file (mongorestore)* | MongoDB Database Tools 100.x | Same tests |
| Users and roles in a backup | ◐ | Archive format, one database | mongodump `--dumpDbUsersAndRoles` | `TestToolBackupWithAuthentication` |
| Point-in-time backup (oplog) | ◐ | Archive format, all databases | Replica set, mongodump `--oplog` | Option passed to the tool (not run in tests: the test server is standalone) |
| Selecting what to back up | ✅ | Databases, include/exclude patterns (`shop.*`, `*.tmp_*`), system collections, views, a filter | | `TestGlobAndSelection`, filtered backup in the round-trip test |
| Restore modes | ✅ | Keep existing (skip duplicate `_id`), replace by `_id`, drop first | | Round-trip test runs all three modes |
| Restore renaming | ✅ | Per-database target (`shop` → `shop_copy`) | | `TestRename`, round-trip test |
| Progress, cancel, job log | ✅ | Status bar → Jobs → *Show log* | | `TestJobLog`, `JobRow log` UI test |
| Validation after restore | ✅ | Document counts are checked against the manifest; truncated or corrupt files fail | | `TestRestoreDetectsTruncatedBackup`, `TestDocReader` |
| Scheduled backups | ✅ | Tasks → type *Backup* (folder with `{date}`/`{time}`) | Scheduler approval | `TestMultiStepTaskAndTaskImport` |
| **Connections** | | | | |
| Export connections with folders | ✅ | Connection Manager → *Export…* | | `connio` and API tests |
| Exporting passwords | ◐ | Opt-in; passphrase-encrypted (scrypt + AES-256-GCM) | Passphrase of 10 or more characters | `TestExportedSecretsNeedThePassphrase` (wrong passphrase and tampering are rejected) |
| Importing Mango3S files, MongoDB Compass exports and lists of connection strings | ✅ | Connection Manager → *Import…* (preview, per-row add/replace/skip, rename, folder) | No network access while reading | `TestParseCompassExport`, `TestParseURIList`, `TestConnectionExportImportRoundTrip` |
| Copying a connection string | ✅ | Connection Manager → *Copy URI*, explorer → *Copy connection string* | The password is never included | API test |
| **Databases and collections** | | | | |
| Create a collection with options | ✅ | Database → *Create collection or view…* → Options | Capped; time series (5.0+); clustered (5.3+); collation; validator with level and action; storage engine | `TestCreateCommand`, `TestManageAgainstRealServer` |
| Create a view | ✅ | Same dialog → *View* | | Integration test (views on views) |
| Rename a collection, also into another database | ✅ | Collection → *Rename…* (replacing a target needs typed confirmation) | Same server; not views or time series | Integration test |
| Rename a database | ◐ | Database → *Rename database…* (typed confirmation, runs as a job) | Moves each collection and recreates views; refused if it contains time series collections | Integration test, including the refusal before anything moves |
| Clear a collection (keeps its indexes and options) | ✅ | Collection → *Clear…* (type `db.coll` to confirm) | | Integration test |
| Copy a collection or database, within or across connections | ✅ | Collection → *Copy collection…*, database → *Copy database…* | Target connection must be writable | `TestCopyCollectionsAndDatabases` (filtered copy, refusal to copy onto itself, missing collections reported) |
| Drop a database or collection | ✅ | Explorer (typed confirmation) | | Existing tests |
| Validation rules | ✅ | Collection → *Validation rules…*: edit, check existing documents, save | `collMod` | Integration and UI tests |
| Validation rules from the schema | ✅ | Settings → General → *Suggest validation rules from the schema* (off by default) adds *Create validation rules…* to the Schema tab: a `$jsonSchema` built from the sampled types and presence (required threshold, reject unseen fields, level, action), checked against existing documents before saving | Schema sample | `TestToJSONSchema*`, `TestValidatorFromSchemaAcceptsExistingDocuments` (the rule accepts all 400 documents, with and without rejecting unknown fields), UI tests |
| Indexes | ✅ | Collection → *Indexes…*: list with size and usage; create (unique, sparse, hidden, TTL, partial, collation, extra options); hide or unhide (4.4+); change TTL; drop (typed confirmation) | | `TestIndexSpec`, integration and UI tests |
| Collection and database statistics | ✅ | *Collection info…* (stats, options, UUID, recreate command); *Database statistics…* | `collStats`, `dbStats` | Integration and UI tests |
| Sharding awareness | ✅ | On a sharded cluster: *Collection info…* shows the shard key, chunks per shard and jumbo chunks; *Database statistics…* the primary shard, sharded collections, shards and balancer, with *Connect* per shard. Backups record shard keys; *Restore…* and *Copy…* can shard collections again (confirmed) | mongos; `config.collections`, `config.chunks`, `balancerStatus` | `TestShardingAwarenessBackupRestoreAndCopy` (sharded test cluster), `TestShardURI`, UI tests |
| **Data workflows** | | | | |
| MongoDB → SQL export (PostgreSQL, MySQL, SQLite) | ✅ | Collection → *Export to SQL…* | | `internal/sqlexport` integration tests load the script into real SQLite, PostgreSQL 17 and MySQL 8.4 |
| SQL → MongoDB migration | ✅ | Existing (see [P2.md](P2.md)) | | Existing tests |
| Multi-step tasks | ✅ | Tasks → type *Multi-step* (up to 20 steps; stop or continue on error) | Approval covers every step | `TestMultiStepTaskAndTaskImport` |
| Task definition import and export | ✅ | Tasks → *Import…* / *Export…* | Imported tasks always start unapproved | `TestExportImportDefinitionsResetApproval`, UI test |
| Index advisor | ✅ | Database or collection → *Index advisor…*. Groups profiled queries by shape (fields matched exactly, ranges, sort) and ranks them by total time. Suggests an index for each (fields matched exactly, then the sort, then ranges) and says whether an existing index already serves it. Lists unused indexes and indexes made redundant by a longer one. Create, unhide, hide and drop from the dialog | Profiler data (it offers to turn the profiler on); `$indexStats` | `TestClassifyAndSuggest`, `TestServes`, `TestRedundant`, `TestDecide`, `TestIndexAdvisor` (suggestion created, then reported as served), UI tests |
| Query profiler | ✅ | Database → *Query profiler…*: level, slow threshold, sample rate, filter by collection, operation and duration, show the command, clear. On a sharded cluster it offers a connection to each shard | mongod (not through mongos) | `TestProfiler`, UI tests |
| Visual explain plan | ✅ | Explain → *Tree* (SVG, one box per stage with documents/keys examined and time; collection scans red), *Compare* (two plans side by side with the changes), *Run again to compare*; Index advisor → *Compare plan* after *Create* | | `TestCommandToExplain…`, `TestExplainCommandBeforeAndAfterIndex`, UI tests |
| Live server monitor | ✅ | Connection → *Server monitor*: operations, network, connections, queues and cache per second (15 minutes kept); replica set members with lag and the oplog window; running operations with filters and *Kill* (typed confirmation, refused on read-only connections) | `serverStatus`, `replSetGetStatus`, `$currentOp`, `killOp`; explains missing privileges | `TestRates`, `TestReplicationLag`, `TestMonitor*` (kills a slow `$where`; lag on the replica set), UI tests |
| Explain, schema, compare/sync, reschema, masking, AI, MCP | ✅ | Existing (see [P2.md](P2.md) and [REFERENCE.md](REFERENCE.md)) | | Existing tests |
| GridFS browser | ✅ | Explorer → database → *GridFS buckets*: files page by page (search, sort), upload and download as jobs with progress, image and text preview, rename, delete (typed confirmation; chunks removed) | Read-only connections can only browse and download | `TestBucketNames`, `TestGridFSRoundTripPreviewRenameDelete` (50 MB byte-identical), UI tests |
| Charts from results | ✅ | Collection → *Chart* view: bar, line, area, pie, scatter, time series; count/sum/avg/min/max; dates by day, week, month or year; split by a field; saved to the Library | Grouping runs on the server (`$group`), over every matching document; works on MongoDB 4.4+ | `internal/charts` tests, `TestChartsOnRealData`, UI tests |
| Change stream viewer | ✅ | *Watch changes* on a connection, database or collection: live list with changed fields, `$match` filter, full document, pause and resume from the token, export to JSON (last 10,000 kept) | Replica set or sharded cluster (explained on a standalone server) | `TestEventFromUpdate`, `TestChangeStreamsAndResume`, UI tests |
| Test data generator | ✅ | Collection → *Generate test data…*: a template with markers (`$name`, `$email`, `$int(18,90)`, `$date("-1y","now")`, `$pick(…)`, `$maybe(p, …)`…) pre-filled from the schema; preview, seed, count; written in batches as a job | Writable connections; asks when the collection isn't empty | `internal/datagen` tests (types, same seed → same data), `TestTestDataGeneratorThroughAPI`, UI tests |
| Shell: mongosh mode | ✅ | Shell → *mongosh*: full JavaScript through the installed mongosh, output streamed, cancel | mongosh installed; not through SSH tunnels or proxies; the password goes in a private temporary file, never on the command line | `TestConnectionURI`, `TestMongoshScriptWithAuth` (password never in the output) |
| Shell: built-in JavaScript | ✅ | Shell → *JavaScript*: variables, loops, functions, cursors (`forEach`, `map`, `toArray`), BSON helpers, `use`/`show`; no mongosh needed | Embedded `goja` engine | `internal/jsengine` tests, `TestJavaScriptEngineOnRealServer` (read-only refusal, endless loop stopped) |
| Users and roles | ✅ | Connection → *Users and roles…*: users per database with their roles as checkboxes, *What can this user do?* (all inherited roles and privileges, plus a check for one action on a database or collection), password change with a generator, drop (typed name); custom roles with a privilege matrix and inherited roles. Warns before you remove your own right to manage users or the last user that has it. The built-in shell has `db.createUser`, `db.getUsers`, `db.changeUserPassword` and the other user and role helpers; passwords are masked before shell code reaches the history | `usersInfo`, `rolesInfo` and the user/role commands; read-only connections can only look | `internal/users` tests, `TestUsersAndRoles` (auth server: sign-in as the new user, role swap, password change, custom role, warnings, shell, history), `TestRedactScript`, UI tests |
| Export aggregation results | ✅ | Aggregation editor → *Export results…*: every result (not just the shown page) to JSON, CSV or Excel; also as a scheduled export | A pipeline ending in `$out`/`$merge` is refused | `TestExportAggregationResults` (export → import, counts add up), UI test |
| Excel import and export | ✅ | *Export…* → Excel: one sheet, typed cells (numbers, true/false, dates in UTC), header row; *Import…* → Excel (.xlsx): choose the sheet, same column types as CSV | `excelize` (BSD-3); up to 1,048,575 rows and 32,767 characters per cell | `TestExcelRoundTrip` (every type, milliseconds and doubles exact), `TestExcelExportImport`, UI test |
| Atlas Search and Vector Search indexes | ✅ | *Collection info…* → Indexes → *Search indexes*: list with build status (refreshes while building), create from a template, edit the definition, drop (typed name); `$search` and `$vectorSearch` templates in the aggregation editor | Atlas, or a local Atlas deployment (`make search-up`); explained on other servers | `TestSearchIndexes` (Atlas local: create, wait until queryable, `$search`, update, drop), `TestSearchIndexesUnsupported` (4.4 and 8.0), UI tests |
| Task notifications | ✅ | Tasks → a task → *Notifications*: never, on failure, or after every run; desktop notification and/or a webhook (JSON POST); *Send a test notification* | The webhook URL is kept in the credential store, never in tasks.json or exports | `TestNotifyWants`, `TestNotifierHearsFinishedRuns`, `TestTaskWebhook`, `TestPost`, `TestDesktopPassesTextAsData`, UI test |
| Compare and sync by a custom key | ✅ | Compare & sync → *Match documents by* (e.g. `email`, or `tenant, orderNo`): documents with different `_id`s are paired by the key; the target keeps its `_id`s | The key must be unique on both sides (a duplicate stops the comparison) | `TestSyncByMatchKey` |
| Sync indexes and validation rules | ✅ | Compare & sync → *Indexes and validation*: create, rebuild or drop indexes and copy the validator, level and action, after a preview; additive changes are preselected | Dropping or rebuilding needs the target typed; a preview that went stale is refused | `TestSyncIndexesAndValidation` (also into a collection that doesn't exist yet), UI test |

## Logical exports vs full backups

| | Export (JSON/CSV, per collection) | Built-in backup (folder) | mongodump archive |
|---|---|---|---|
| Documents with exact BSON types | Canonical Extended JSON: yes. Relaxed JSON and CSV lose some types | Yes (raw BSON) | Yes |
| Indexes | No | Yes | Yes |
| Collection options and validators | No | Yes | Yes |
| Views | No | Definitions | Definitions |
| Users and roles | No | No | Yes, for one database (`--dumpDbUsersAndRoles`) |
| Point in time | No | No; collections are read one after another | With `--oplog` on a replica set |
| Works through SSH tunnels and proxies | Yes | Yes | No: the external tools can't use the Mango3S tunnel |
| Read by the other tools | mongoimport | mongorestore (folder layout) | mongorestore |

Never included in any backup:

- server configuration;
- replica-set and sharding configuration;
- `local` and `config` databases;
- the `admin` database, unless you select it;
- profiler data;
- data on other shards when connected to a single shard.

A backup taken while applications write is not a consistent snapshot across collections. Use an archive backup with `--oplog` on a replica set, or stop writes, when you need one.

## Safety

- **Credentials:**
  - Passwords are passed to mongodump and mongorestore in a temporary config file. The file is readable only by you and is deleted after the run.
  - Passwords never go on the command line, where other processes could read them.
  - Tool output is scrubbed of secrets before it reaches the job log.
- **Destructive operations** need confirmation. The following need the name typed:
  - Drop and restore: the connection name.
  - Drop a target during a copy: the target database.
  - Replace during a rename: `db.coll`.
  - Clear: `db.coll`.
  - Rename database: the database name.
  - Drop index: the index name.
- **Overwrites:** backups never overwrite. A backup folder must be new or empty, and an archive or SQL file must not exist yet.
- **Read-only connections** can back up but can't restore, copy into, rename, clear, change validation or indexes, or change the profiler.
- **Nothing is silently skipped:**
  - Failed collections are listed in the manifest and the job log, and the job fails.
  - Index conflicts during a restore are errors.
  - SQL export keeps fields outside the sample, and values whose type doesn't fit their column, in an `_extra` JSON column. It counts both.
  - An integer is never stored in a column that would round it, and neither is a double.
- **Memory:** documents are streamed in batches (1,000 documents or 8 MB), never loaded as a whole collection.

## Limits

- **Built-in restore:**
  - It doesn't read `.archive` files; use the archive option, which runs mongorestore.
  - It skips raw time-series bucket collections (`system.buckets.*`) from mongodump folders and warns; use mongorestore for those.
  - Time-series collections backed up by Mango3S are restored as measurements.
- **Existing collections:** when a restore or copy target already exists, its current options are kept and the job log says so. Use *Drop collections first* to recreate it with the saved options.
- **Renaming a database:**
  - Users and roles defined on the old database are not moved.
  - Time-series collections block the rename; use *Copy database*, then drop the old database.
- **External tools:** SSH-tunnelled and proxied connections can't use mongodump or mongorestore (the built-in format works). Neither can AWS authentication with a session token.
- **Profiler:** the slow threshold applies server-wide. Profiling must be enabled per shard.
- **Index advisor:**
  - It only sees queries the profiler recorded.
  - `$or`, `$text` and geo queries get an explanation instead of a suggestion.
  - Usage counts behind "unused" restart when the server restarts.
  - Unique and TTL indexes are never reported as redundant.
  - Check suggestions against your application before creating them.
- **SQL export:**
  - Columns come from a sample of the first documents (1,000 by default).
  - Arrays and mixed-type fields are stored as JSON (JSONB in PostgreSQL).
  - Indexes, validators and references between collections are not converted.
- **Connection import from Compass:** passwords that Compass encrypted with its own passphrase can't be read. Enter them after importing.
