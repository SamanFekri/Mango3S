# Mango3S UI reference: Welcome tab, connections, editing and shortcuts

## Welcome tab and workspace tabs

- **Welcome** opens first at startup and is always the first tab. It can be closed; reopen it
  with Shift+F1, Help → Welcome or View → Welcome.
- **Actions:** new connection, open saved connections (the Connection Manager), keyboard
  shortcuts and settings.
- **Recent connections:** profiles sorted by when you last connected. Each row has Connect / Open
  and Edit, and its quick-connect key (Alt+1…9, ⌥1…9 on macOS).
- **Workspace tabs:** connecting opens a workspace tab for that connection. The tab shows:
  - server version, topology, hosts, user and network path (TLS, proxy, SSH);
  - recently opened collections;
  - databases with their collections, views and system collections.

  Collection, shell and SQL tabs open alongside it.
- **Unsaved work:** a tab reports what would be lost:
  - an unsaved field edit, or an open document editor;
  - a pipeline, script or SQL not saved to the library.

  Closing that tab asks first: the × button, middle click, ⌘W / Ctrl+W, disconnecting, or File →
  Close Tab. So does quitting the app (native dialog). Dirty tabs show a dot.
  ⇧⌘W / Ctrl+Shift+F4 closes without asking.

## Connection Manager

The connection list on the left groups profiles by **folder** (set on the Server tab, e.g.
`Clients/ACME`) and has search, New, Duplicate and Delete:

- **Duplicate** copies the settings, the stored secrets and the collection history.
- **Delete** removes the profile, every stored secret and its history.

All nine tabs edit one form, so switching tabs never loses input. Error messages name the tab to
fix ("SSH: …"), and the dialog switches to that tab.

| Tab | What it configures | Notes |
|---|---|---|
| Server | Name, folder, read-only; **Manual** (Standalone, Replica Set with seed list and name, Sharded Cluster with several mongos, DNS seed list / SRV) or a **connection string** | Manual mode builds the URI, shown live. "Fill in the fields from this string" imports a URI and lists options that have no field |
| Authentication | Default (SCRAM), SCRAM-SHA-256/1, X.509, LDAP (PLAIN), AWS IAM (+ session token), OIDC workload identity (mechanism properties) | Kerberos is listed and disabled: this build has no GSSAPI support |
| SSL / TLS | Enable, CA file, client certificate + key (PEM), key password, allow invalid host names, allow invalid certificates | Files are checked before connecting: missing, no certificates, no key, encrypted without a password, expired or not yet valid. Server-side failures (unknown CA, host mismatch, not TLS) get specific messages. "Allow invalid host names" still verifies the chain |
| SSH | Host, port, user; password (also keyboard-interactive password prompts), private key (+ passphrase) or SSH agent | Host keys are checked against `~/.ssh/known_hosts`. An unknown key shows its SHA-256 fingerprint and a "Trust this key" button that pins it; a changed key is refused. Every MongoDB connection, including discovered replica-set members, goes through the tunnel, which reconnects if it drops |
| Proxy | None, SOCKS5 (host names resolved by the proxy) or HTTP CONNECT, with optional username / password | With SSH enabled, the proxy carries the SSH connection |
| IntelliShell | Default database; initialization commands; run them automatically when a shell tab opens | Commands run in the built-in shell interpreter (a mongosh subset, not JavaScript). Without auto-run they prefill new shell tabs |
| MongoDB Tools | Tools folder and per-tool paths for mongosh, mongodump, mongorestore, mongoexport, mongoimport, bsondump, mongostat, mongotop, mongofiles | "Detect" shows each tool's path, where it was found and its version. It reports missing tools (with a download link), configured paths that don't exist, tools that won't run, legacy (< 100.x) Database Tools, and mongosh 2.x against servers older than 4.2. Mango3S's own import, export and shell don't use these tools |
| Advanced | Connect, server selection and operation (`timeoutMS`) timeouts, heartbeat, local threshold, app name, read preference (+ tag set, max staleness), read concern, write concern (w, journal), pool sizes, max connecting, idle time, retry reads/writes, compression (zstd, snappy, zlib) | Settings here override the same options in the connection string. Socket timeout and `wtimeout` are listed as unsupported (removed in Go driver v2) |
| Collection History | Collections recently opened with this connection (most recent first, with a count) | Click to reopen (connects if needed); Clear history. Kept per connection in `collection_history.json` |

**Credentials:**

- Passwords, SSH passwords and key passphrases, proxy passwords, TLS key passwords and AWS
  session tokens go to the OS credential store, or the encrypted file store where none exists.
- `connections.json` holds only flags saying which secrets exist.
- An empty secret field keeps the stored value; "Remove the saved …" deletes it.
- Tests check that no secret reaches `connections.json` or the logs.

## Collection query editor and paging

- **Query editor:** each collection tab shows its query as one statement, as in Studio 3T, e.g.
  `db.getCollection("agent_definitions").find({})`.
  - You can edit the whole statement: the filter, a projection argument, and `.sort()`,
    `.skip()` and `.limit()`.
  - `db.name.find(…)` and `db.getSiblingDB("x").getCollection("y")` also work.
  - The statement is parsed on the server, never executed as a shell script.
- **Where it runs:**
  - A `find()` on the tab's collection runs as the tab's query.
  - A `find()` on another collection opens that collection's tab.
  - An `aggregate([…])` opens in the aggregation editor.
  - Anything else (writes, `count()`, scripts) is rejected with a pointer to IntelliShell.
- **Fields:** the **Query / Fields** switch shows the same query as separate filter, projection,
  sort, skip and limit fields. Switching carries the query across, and the choice is remembered.
- **Results are always paged:**
  - **Collection tabs, aggregation, SQL:** one page at a time (the page size is in Settings and
    on the results toolbar), within the query's own skip and limit.
  - **IntelliShell:** `find()` and read-only `aggregate()` results are paged too. ‹ › re-run that
    one statement for the previous or next page.
  - `$out` / `$merge` pipelines run once and aren't paged.

## Result views and field editing

- **Views:** Tree (the default until you choose another default in Settings), Table and JSON.
  Switch with ⌥⌘1/2/3 (Ctrl+Alt+1/2/3).
- **Exact types:** documents arrive as canonical Extended JSON, so the Tree's Type column and the
  Table show exact BSON types (int32 vs double, 64-bit integers without rounding).
- **Embedded fields:** the Table's "Embedded fields" option shows each field of an embedded
  document as its own column.

Editing works the same way in the Tree and the Table:

- A **single click selects** a cell. It never starts editing.
- A **double click edits only that field.** The current value is read fresh from the server:
  - strings are edited as raw text, numbers as digits, dates as ISO 8601 and ObjectIds as hex;
  - documents and arrays are edited in shell syntax.

  The field's type is kept. Change a type with **Edit document (JSON)**: the `{ }` button on a
  table row, the button on a tree document, or ⌘J / Ctrl+J.
- **Enter** saves, **Esc** cancels, **Tab / Shift+Tab** save and move to the next or previous
  editable cell. _id cells are skipped. Enter on a selected cell does *not* start editing.
- **Invalid input** is rejected with a type-specific message next to the field, and the text is
  kept. Examples:
  - "Out of range for a 32-bit integer"
  - "An ObjectId is 24 hexadecimal characters"
  - "Strings need quotes here"
- **Saving** sends `$set` (or `$unset` when removing a field with Delete) on that path only:
  - nested paths and array elements (`items.2.qty`) are supported;
  - the rest of the document is never rewritten.

  The update applies only if the field still has the value you started from and nothing else in
  the document changed meanwhile. Otherwise you get a conflict and nothing is overwritten.
- **Display:** the grid shows the document the server returns after the update. A failed update
  leaves the old value in place.
- **Unsaved edits:** clicking another cell with unsaved changes keeps the editor open and says
  so. Paging, switching views and closing the tab ask first.
- **Read-only connections, views and documents without `_id`** can't be edited. A double click
  explains why.
- **Undo / redo:** ⌘Z / ⇧⌘Z (Ctrl+Z / Ctrl+Y) undo and redo field edits in the results panel,
  using the same conflict check.

## Keyboard shortcuts

All shortcuts live in one registry (`frontend/src/lib/shortcuts.ts`):

- The reference dialog (Help → Keyboard Shortcuts, F1, ⇧⌘/ / Ctrl+Shift+/, the Welcome tab, the
  `?` button) is generated from it.
- The dispatcher matches key events against it.
- A test fails if a listed shortcut has no handler.

**How bindings were chosen:**

- **Studio 3T** means the binding matches Studio 3T's documented default
  ([Studio 3T hotkeys](https://studio3t.com/knowledge-base/articles/hotkeys/)).
- **Mango3S default** marks bindings where Studio 3T documents none, or where its binding would
  conflict.

**Rules:**

- Plain keys (Delete, Enter, letters, digits with Alt) never fire while you are typing in a field
  or editor.
- Shortcuts that work while typing use modifier combinations that don't type text.
- Results and explorer shortcuts need focus in that panel.
- AltGr combinations (Ctrl+Alt on Windows layouts, which type characters such as @) are ignored.
- Standard text editing (copy, paste, undo, find, comment) is left to the editor and the native
  Edit menu.
- The native menu (File, Edit, View, Help) runs the same actions.

**Connections**

| Action | macOS | Windows / Linux | Where | Mapping |
|---|---|---|---|---|
| Open the connection manager | `⌘N` | `Ctrl+N` | anywhere | Studio 3T |
| New connection | `⌥⌘N` | `Ctrl+Alt+N` | anywhere | Mango3S default |
| Connect to recent connection 1–9 — In the order shown under Recent connections on the Welcome tab. | `⌥1…9` | `Alt+1…9` | anywhere | Studio 3T |
| Disconnect the current connection | `⌃⌥D` | `Ctrl+Alt+D` | anywhere | Studio 3T |

**Tabs**

| Action | macOS | Windows / Linux | Where | Mapping |
|---|---|---|---|---|
| Close tab | `⌘W` | `Ctrl+W` / `Ctrl+F4` | anywhere | Studio 3T |
| Close tab without asking (discards unsaved changes) | `⇧⌘W` | `Ctrl+Shift+F4` | anywhere | Studio 3T |
| Next tab | `⌃PgDn` / `⌃Tab` | `Ctrl+PgDn` / `Ctrl+Tab` | anywhere | Studio 3T |
| Previous tab | `⌃PgUp` / `⌃⇧Tab` | `Ctrl+PgUp` / `Ctrl+Shift+Tab` | anywhere | Studio 3T |
| Go to tab 1–9 | `⌘1…9` | `Ctrl+1…9` | anywhere | Mango3S default |
| Open the Welcome tab | `⇧F1` | `Shift+F1` | anywhere | Mango3S default |
| Open IntelliShell for this connection | `⌘L` | `Ctrl+L` | anywhere | Studio 3T |
| Open SQL query | `⇧⌘L` | `Ctrl+Shift+L` | anywhere | Studio 3T |
| Open the aggregation editor for this collection | `F4` | `F4` | active tab | Studio 3T |
| Open settings | `⌘P` / `⌘,` | `Ctrl+P` | anywhere | Studio 3T |
| Open query history | `⇧⌘H` | `Ctrl+Shift+H` | anywhere | Mango3S default |

**Help**

| Action | macOS | Windows / Linux | Where | Mapping |
|---|---|---|---|---|
| Show keyboard shortcuts | `F1` / `⇧⌘/` | `F1` / `Ctrl+Shift+/` | anywhere | Mango3S default |

**Queries**

| Action | macOS | Windows / Linux | Where | Mapping |
|---|---|---|---|---|
| Run the query, pipeline, script or SQL — F5 is Studio 3T’s “Execute”; Mod+Enter also works inside every editor. | `⌘↩` / `F5` | `Ctrl+Enter` / `F5` | active tab | Studio 3T |
| Run the selected text (shell) | `F9` / `⇧⌘↩` | `F9` / `Ctrl+Shift+Enter` | active tab | Studio 3T |
| Run the statement at the cursor (shell) — A statement is the block of lines around the cursor, up to blank lines. | `F6` | `F6` | active tab | Studio 3T |
| Format the query or pipeline — Collection (filter, projection, sort) and aggregation tabs. | `⌥⌘L` | `Ctrl+Alt+L` / `Alt+L` | active tab | Studio 3T |
| Save to the library | `⌘S` | `Ctrl+S` | active tab | Studio 3T |
| Save as a new library item | `⇧⌘S` | `Ctrl+Shift+S` | active tab | Studio 3T |
| Cancel the running operation | `⌘.` | `Ctrl+.` | active tab | Mango3S default |
| Refresh results (or the selected explorer item) | `⌘R` | `Ctrl+R` | active tab | Studio 3T |
| Show or hide the visual query builder | `⌘B` | `Ctrl+B` | active tab | Studio 3T |
| Query code | `⌘G` | `Ctrl+G` | active tab | Studio 3T |
| Explain | `F8` | `F8` | active tab | Studio 3T |
| Add a pipeline stage | `⇧⌘N` | `Ctrl+Shift+N` | active tab | Studio 3T |

**Results**

| Action | macOS | Windows / Linux | Where | Mapping |
|---|---|---|---|---|
| Tree view | `⌥⌘1` | `Ctrl+Alt+1` | active tab | Studio 3T |
| Table view | `⌥⌘2` | `Ctrl+Alt+2` | active tab | Studio 3T |
| JSON view | `⌥⌘3` | `Ctrl+Alt+3` | active tab | Studio 3T |
| Select the next / previous cell | `↑` / `↓` / `←` / `→` | `↑` / `↓` / `←` / `→` | results | Mango3S default |
| Edit the field (double-click) — Double-click only — a single click selects. In the table and tree views. | Double-click | Double-click | results | Studio 3T |
| Save the field being edited | `↩` | `Enter` | results | Mango3S default |
| Cancel editing the field | `Esc` | `Esc` | results | Mango3S default |
| Next / previous editable cell (saves an edit first) | `Tab` / `⇧Tab` | `Tab` / `Shift+Tab` | results | Mango3S default |
| Copy value as text | `⌘C` | `Ctrl+C` | results | Studio 3T |
| Copy value (exact shell syntax) | `⌥⇧C` | `Alt+Shift+C` | results | Studio 3T |
| Copy document | `⇧⌘C` | `Ctrl+Shift+C` | results | Studio 3T |
| Select all documents on the page | `⌘A` | `Ctrl+A` | results | Mango3S default |
| Paste documents (opens Insert with the clipboard) | `⌘V` | `Ctrl+V` | results | Mango3S default |
| Undo the last field edit | `⌘Z` | `Ctrl+Z` | results | Mango3S default |
| Redo the field edit | `⇧⌘Z` | `Ctrl+Y` / `Ctrl+Shift+Z` | results | Mango3S default |

**Documents**

| Action | macOS | Windows / Linux | Where | Mapping |
|---|---|---|---|---|
| Insert document — Not while typing in an editor (Mod+D selects the next match there). | `⌘D` | `Ctrl+D` | active tab | Studio 3T |
| Edit the whole document as JSON | `⌘J` | `Ctrl+J` | results | Studio 3T |
| Delete the selected documents (asks first) | `⇧Del` / `⇧⌫` | `Shift+Del` | results | Studio 3T |
| Remove the selected field (asks first) | `Del` / `⌫` | `Del` | results | Studio 3T |

**Explorer**

| Action | macOS | Windows / Linux | Where | Mapping |
|---|---|---|---|---|
| Move between items | `↑` / `↓` / `Home` / `End` | `↑` / `↓` / `Home` / `End` | explorer | Mango3S default |
| Expand / collapse | `→` / `←` | `→` / `←` | explorer | Studio 3T |
| Open the collection / connect | `↩` | `Enter` | explorer | Studio 3T |
| Drop the selected collection or database (asks first) | `Del` / `⌫` | `Del` | explorer | Studio 3T |
| Refresh all connections | `⇧⌘R` | `Ctrl+Shift+R` | anywhere | Studio 3T |
| Filter collections | `⇧⌘F` | `Ctrl+Shift+F` | anywhere | Mango3S default |

**Focus**

| Action | macOS | Windows / Linux | Where | Mapping |
|---|---|---|---|---|
| Focus the explorer | `⌃⇧1` | `Ctrl+Shift+1` | anywhere | Mango3S default |
| Focus the query editor | `⌃⇧2` | `Ctrl+Shift+2` | anywhere | Mango3S default |
| Focus the results | `⌃⇧3` | `Ctrl+Shift+3` | anywhere | Mango3S default |

**Text editing**

| Action | macOS | Windows / Linux | Where | Mapping |
|---|---|---|---|---|
| Undo / redo typing | `⌘Z` / `⇧⌘Z` | `Ctrl+Z` / `Ctrl+Y` | editors | Studio 3T |
| Cut / copy / paste | `⌘X` / `⌘C` / `⌘V` | `Ctrl+X` / `Ctrl+C` / `Ctrl+V` | editors | Studio 3T |
| Select all | `⌘A` | `Ctrl+A` | editors | Mango3S default |
| Find in the editor — Script, pipeline and SQL editors. | `⌘F` | `Ctrl+F` | editors | Studio 3T |
| Toggle line comment | `⌘/` | `Ctrl+/` | editors | Studio 3T |
| Autocomplete | `⌃Space` | `Ctrl+Space` | editors | Studio 3T |
| Previous / next script from history (shell) | `⌥↑` / `⌥↓` | `Alt+↑` / `Alt+↓` | editors | Mango3S default |

## Unsupported and limitations

- **Authentication:**
  - Kerberos (GSSAPI) needs a build with the `gssapi` tag and system libraries.
  - OIDC supports only machine workflows (Azure, GCP, Kubernetes); there is no browser sign-in.
- **SSH:**
  - Keyboard-interactive one-time codes (2FA) are not supported.
  - The Windows OpenSSH agent pipe is not supported; use a key file there.
  - No jump hosts (ProxyJump chains) and no local port-forwarding UI.
- **DNS with SSH or a proxy:** with `mongodb+srv://`, SRV and TXT lookups happen on your machine.
  Only the resulting hosts are reached through the tunnel or proxy.
- **Proxy:** SOCKS4 is not supported, and HTTP proxies support only Basic authentication.
- **Go driver v2 options:** socket timeout and write-concern `wtimeout` were removed in Go driver
  v2; use the operation timeout. OCSP and CRL options are not exposed.
- **MongoDB Tools:** tools are detected and checked, but Mango3S doesn't run them.
- **IntelliShell:** initialization commands use the built-in interpreter (no JavaScript).
- **Field editing:**
  - Field names containing `.` or starting with `$` must be edited through Edit document (JSON).
  - A type changes only through the JSON editor.
  - Any concurrent change to the same document between reading and writing causes a conflict
    (deliberately strict).
  - Undo history is per results panel and lasts for the session.
- **Shortcuts:**
  - The keymap can't be customized.
  - Studio 3T's ⌘E / Ctrl+E ("edit field") is deliberately not bound, because editing is
    double-click only.
  - Its ⌘↩ "show embedded fields" is replaced by the Embedded fields checkbox, because ⌘↩ runs
    the query.
  - Native menu items don't show key hints, so they never double-fire with the web view's handlers.
  - Windows and Linux bindings are defined and unit-tested, but this release was exercised on
    macOS only.
