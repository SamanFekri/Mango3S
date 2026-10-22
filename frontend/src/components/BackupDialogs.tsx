import { useApp } from '../state/AppContext'
import { useEffect, useMemo, useState } from 'react'
import { backend } from '../lib/backend'
import { formatBytes, formatCount } from '../lib/format'
import { toAppError, type AppError } from '../lib/errors'
import type { BackupEngine, BackupInspection, BackupTools, NamespaceSelection, RestoreMode, SessionInfo } from '../lib/types'
import { ErrorBanner, Modal, Segmented, Spinner } from './common'
import { ConfirmInput, JobStatus, useJob } from './p2common'

const list = (s: string) =>
  s
    .split(/[,\n]/)
    .map((x) => x.trim())
    .filter(Boolean)

const stamp = () => {
  const d = new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}`
}

const safeName = (s: string) => s.replace(/[^\w.-]+/g, '_')

const joinPath = (dir: string, name: string) => (dir.endsWith('/') || dir.endsWith('\\') ? dir + name : `${dir}${dir.includes('\\') && !dir.includes('/') ? '\\' : '/'}${name}`)

/** Tells why the MongoDB Database Tools can't be used, with install guidance. */
function ToolsNotice({ tools, need }: { tools: BackupTools | null; need: 'dump' | 'restore' }) {
  if (!tools) return <Spinner small />
  const info = tools[need]
  if (tools.blocked) return <p className="warning-text">{tools.blocked}</p>
  if (info.status !== 'ok')
    return (
      <p className="warning-text">
        {info.name || (need === 'dump' ? 'mongodump' : 'mongorestore')}: {info.message || 'not available.'}
      </p>
    )
  return (
    <p className="muted small">
      Using {info.name} {info.version} ({info.path}). Credentials are passed in a private temporary file, never on the command line.
    </p>
  )
}

function useBackupTools(connId: string) {
  const [tools, setTools] = useState<BackupTools | null>(null)
  useEffect(() => {
    backend
      .backupToolsStatus(connId)
      .then(setTools)
      .catch(() => setTools({ blocked: 'Could not check for the MongoDB Database Tools.' } as BackupTools))
  }, [connId])
  const usable = !!tools && !tools.blocked
  return { tools, canDump: usable && tools?.dump.status === 'ok', canRestore: usable && tools?.restore.status === 'ok' }
}

/**
 * Back up databases or collections. The built-in engine writes the
 * mongodump folder layout (works through SSH tunnels and proxies); the tools
 * engine runs mongodump for .archive files, users and roles, or an oplog.
 */
export function BackupDialog({ conn, db, coll, onClose }: { conn: SessionInfo; db?: string; coll?: string; onClose: () => void }) {
  const { tools, canDump } = useBackupTools(conn.id)
  const [engine, setEngine] = useState<BackupEngine>('builtin')
  const [allDbs, setAllDbs] = useState<string[] | null>(null)
  const [dbs, setDbs] = useState<string[]>(db ? [db] : [])
  const [parent, setParent] = useState('')
  const [folder, setFolder] = useState(`${safeName(conn.name)}-${db ? safeName(db) + '-' : ''}${stamp()}`)
  const [archive, setArchive] = useState('')
  const [gzip, setGzip] = useState(true)
  const [views, setViews] = useState(true)
  const [system, setSystem] = useState(false)
  const [include, setInclude] = useState(coll && db ? `${db}.${coll}` : '')
  const [exclude, setExclude] = useState('')
  const [query, setQuery] = useState('')
  const [users, setUsers] = useState(false)
  const [oplog, setOplog] = useState(false)
  const [jobId, setJobId] = useState<string | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  const job = useJob(jobId)

  useEffect(() => {
    backend
      .listDatabases(conn.id)
      .then((d) => setAllDbs(d.map((x) => x.name).filter((n) => n !== 'local' && n !== 'config')))
      .catch((err) => setError(toAppError(err)))
  }, [conn.id])

  const toolDb = dbs.length === 1 ? dbs[0] : ''
  const start = async () => {
    setError(null)
    try {
      const selection: NamespaceSelection = { databases: dbs, include: list(include), exclude: list(exclude), includeSystem: system, views }
      const j = await backend.startBackup({
        connId: conn.id,
        engine,
        builtin: { dir: parent ? joinPath(parent, folder.trim()) : '', gzip, selection, query: query.trim() },
        tool: {
          archive,
          out: '',
          gzip,
          db: toolDb,
          collection: toolDb && coll ? coll : '',
          excludeCollections: list(exclude),
          query: toolDb && coll ? query.trim() : '',
          usersAndRoles: users,
          oplog,
          viewsAsCollections: false,
        },
      })
      setJobId(j.id)
    } catch (err) {
      setError(toAppError(err))
    }
  }
  const running = job && (job.state === 'running' || job.state === 'queued')
  const ready = engine === 'builtin' ? !!parent && !!folder.trim() : canDump && !!archive && dbs.length <= 1
  return (
    <Modal
      title={`Back up ${coll ? `${db}.${coll}` : db ? db : conn.name}`}
      onClose={onClose}
      wide
      footer={
        <>
          <span className="muted small">{running ? 'The backup keeps running if you close this window.' : ''}</span>
          <span className="spacer" />
          <button onClick={onClose}>{jobId ? 'Close' : 'Cancel'}</button>
          {!jobId && (
            <button className="primary" disabled={!ready} onClick={start}>
              Start backup
            </button>
          )}
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <Segmented
        label="Backup format"
        value={engine}
        onChange={setEngine}
        options={[
          { value: 'builtin', label: 'Backup folder (built-in)', title: 'mongodump-compatible folder; works over SSH and proxies' },
          { value: 'tools', label: 'Archive file (mongodump)', title: 'A single .archive file; can include users, roles and an oplog' },
        ]}
      />
      <p className="muted small">
        A logical backup: documents with exact BSON types, indexes, collection options (validators, collation, capped, time series) and view definitions.
        {engine === 'builtin'
          ? ' Users, roles and server settings are not included; use the archive format for users and roles.'
          : ' Server settings, sharding and replica set configuration are never included.'}
      </p>
      {engine === 'tools' && <ToolsNotice tools={tools} need="dump" />}

      <fieldset className="field-group">
        <legend>Databases</legend>
        {allDbs === null ? (
          <Spinner small />
        ) : (
          <div className="check-grid">
            {allDbs.map((n) => (
              <label key={n} className="check">
                <input type="checkbox" checked={dbs.includes(n)} onChange={(e) => setDbs(e.target.checked ? [...dbs, n] : dbs.filter((x) => x !== n))} />
                {n}
              </label>
            ))}
          </div>
        )}
        <span className="muted small">
          {dbs.length === 0 ? 'Nothing ticked: every database except admin, local and config.' : `${dbs.length} selected.`}
          {engine === 'tools' && dbs.length > 1 && ' mongodump backs up one database or all of them; tick one or none.'}
        </span>
      </fieldset>

      {engine === 'builtin' ? (
        <>
          <div className="field-row">
            <label className="field">
              <span>Save in folder</span>
              <div className="file-row">
                <input value={parent} onChange={(e) => setParent(e.target.value)} placeholder="Choose a folder" />
                <button onClick={async () => setParent((await backend.chooseFile('folder', 'Save the backup in')) || parent)}>Choose…</button>
              </div>
            </label>
            <label className="field">
              <span>Backup name (new folder)</span>
              <input value={folder} onChange={(e) => setFolder(e.target.value)} aria-label="Backup name" />
            </label>
          </div>
          <div className="field-row">
            <label className="field">
              <span>Only namespaces matching (comma-separated, * wildcard)</span>
              <input value={include} onChange={(e) => setInclude(e.target.value)} placeholder="e.g. shop.orders, shop.customers_*" />
            </label>
            <label className="field">
              <span>Skip namespaces matching</span>
              <input value={exclude} onChange={(e) => setExclude(e.target.value)} placeholder="e.g. *.tmp_*" />
            </label>
          </div>
          <label className="field">
            <span>Filter (optional, Extended JSON, applied to every collection)</span>
            <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder='e.g. { "createdAt": { "$gte": { "$date": "2024-01-01T00:00:00Z" } } }' className="mono" />
          </label>
          <div className="check-row">
            <label className="check">
              <input type="checkbox" checked={gzip} onChange={(e) => setGzip(e.target.checked)} /> Compress (gzip)
            </label>
            <label className="check">
              <input type="checkbox" checked={views} onChange={(e) => setViews(e.target.checked)} /> Include view definitions
            </label>
            <label className="check">
              <input type="checkbox" checked={system} onChange={(e) => setSystem(e.target.checked)} /> Include system collections (system.js…)
            </label>
          </div>
        </>
      ) : (
        <>
          <label className="field">
            <span>Archive file</span>
            <div className="file-row">
              <input value={archive} onChange={(e) => setArchive(e.target.value)} placeholder="Choose where to save the .archive file" />
              <button
                onClick={async () =>
                  setArchive((await backend.chooseSaveFile('Save the archive as', `${safeName(conn.name)}-${toolDb ? safeName(toolDb) + '-' : ''}${stamp()}.archive${gzip ? '.gz' : ''}`)) || archive)
                }
              >
                Save as…
              </button>
            </div>
          </label>
          <label className="field">
            <span>Skip collections (comma-separated names)</span>
            <input value={exclude} onChange={(e) => setExclude(e.target.value)} />
          </label>
          <div className="check-row">
            <label className="check">
              <input type="checkbox" checked={gzip} onChange={(e) => setGzip(e.target.checked)} /> Compress (gzip)
            </label>
            <label className="check" title="Needs exactly one database other than admin">
              <input type="checkbox" checked={users} disabled={!toolDb || toolDb === 'admin'} onChange={(e) => setUsers(e.target.checked)} /> Include the database's users
              and roles
            </label>
            <label className="check" title="Point-in-time backup of all databases on a replica set">
              <input type="checkbox" checked={oplog} disabled={!!toolDb} onChange={(e) => setOplog(e.target.checked)} /> Include the oplog (point in time)
            </label>
          </div>
        </>
      )}
      {jobId && <JobStatus jobId={jobId} />}
    </Modal>
  )
}

/** Restore a backup folder (built-in or mongodump) or a mongodump archive. */
export function RestoreDialog({ conn, db, onClose }: { conn: SessionInfo; db?: string; onClose: () => void }) {
  const { tools, canRestore } = useBackupTools(conn.id)
  const { confirm: confirmDialog } = useApp()
  const [engine, setEngine] = useState<BackupEngine>('builtin')
  const [dir, setDir] = useState('')
  const [inspection, setInspection] = useState<BackupInspection | null>(null)
  const [picked, setPicked] = useState<Set<string>>(new Set())
  const [targets, setTargets] = useState<Record<string, string>>({})
  const [mode, setMode] = useState<RestoreMode>('insert')
  const [skipIndexes, setSkipIndexes] = useState(false)
  const [skipOptions, setSkipOptions] = useState(false)
  const [stopOnError, setStopOnError] = useState(false)
  const [bypass, setBypass] = useState(false)
  const [shardColls, setShardColls] = useState(false)
  const [archive, setArchive] = useState('')
  const [gzip, setGzip] = useState(true)
  const [nsInclude, setNsInclude] = useState(db ? `${db}.*` : '')
  const [nsFrom, setNsFrom] = useState('')
  const [nsTo, setNsTo] = useState('')
  const [users, setUsers] = useState(false)
  const [dryRun, setDryRun] = useState(false)
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [jobId, setJobId] = useState<string | null>(null)
  const [error, setError] = useState<AppError | null>(null)

  const inspect = async (path: string) => {
    setDir(path)
    setInspection(null)
    setError(null)
    if (!path) return
    setBusy(true)
    try {
      const ins = await backend.inspectBackup(path)
      setInspection(ins)
      setPicked(new Set(ins.entries.filter((e) => !e.coll.startsWith('system.')).map((e) => `${e.db}.${e.coll}`)))
      const srcDbs = [...new Set(ins.entries.map((e) => e.db))]
      // Restoring from the database menu targets that database.
      setTargets(Object.fromEntries(srcDbs.map((d) => [d, db && srcDbs.length === 1 ? db : d])))
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  // mongorestore has no replace-by-_id mode.
  useEffect(() => {
    if (engine === 'tools' && mode === 'upsert') setMode('insert')
  }, [engine, mode])
  const srcDbs = useMemo(() => [...new Set(inspection?.entries.map((e) => e.db) ?? [])], [inspection])
  const drop = engine === 'builtin' ? mode === 'drop' : !dryRun && mode === 'drop'

  const sharded = (inspection?.entries ?? []).filter((e) => e.shardKey && picked.has(`${e.db}.${e.coll}`))
  const canShard = engine === 'builtin' && conn.topology === 'sharded' && sharded.length > 0

  const start = async () => {
    setError(null)
    if (canShard && shardColls) {
      const ok = await confirmDialog({
        title: 'Shard the restored collections',
        message: (
          <>
            <p>Before loading their data, these collections will be sharded on {conn.name} with the key they had when they were backed up:</p>
            <ul className="small">
              {sharded.map((e) => (
                <li key={`${e.db}.${e.coll}`}>
                  <code>
                    {e.db}.{e.coll}
                  </code>{' '}
                  on <code>{e.shardKey}</code>
                  {e.shardUnique && ' (unique)'}
                </li>
              ))}
            </ul>
            <p className="muted small">Sharding a collection can't be undone without dropping it. The target databases must allow sharding.</p>
          </>
        ),
        confirmLabel: 'Shard and restore',
      })
      if (!ok) return
    }
    try {
      const include = inspection?.entries.filter((e) => picked.has(`${e.db}.${e.coll}`)).map((e) => `${e.db}.${e.coll}`) ?? []
      const renames = srcDbs.filter((d) => targets[d] && targets[d] !== d).map((d) => ({ from: `${d}.*`, to: `${targets[d].trim()}.*` }))
      const j = await backend.startRestoreBackup(
        {
          connId: conn.id,
          engine,
          builtin: {
            dir,
            mode,
            selection: { databases: [], include, exclude: [], includeSystem: include.some((n) => n.split('.').slice(1).join('.').startsWith('system.')), views: true },
            renames,
            skipIndexes,
            skipOptions,
            stopOnError,
            bypassValidate: bypass,
            shardCollections: canShard && shardColls,
          },
          tool: {
            archive,
            dir: '',
            gzip,
            nsInclude: list(nsInclude),
            nsExclude: [],
            nsFrom: nsFrom.trim() ? [nsFrom.trim()] : [],
            nsTo: nsFrom.trim() ? [nsTo.trim()] : [],
            drop,
            noIndexRestore: skipIndexes,
            noOptionsRestore: skipOptions,
            stopOnError,
            bypassValidation: bypass,
            usersAndRoles: users,
            oplogReplay: false,
            dryRun,
          },
        },
        confirm,
      )
      setJobId(j.id)
    } catch (err) {
      setError(toAppError(err))
    }
  }
  const ready =
    !conn.readOnly &&
    (engine === 'builtin' ? !!inspection && picked.size > 0 : canRestore && !!archive && (!nsFrom.trim() || !!nsTo.trim())) &&
    (!drop || confirm === conn.name)
  const all = inspection?.entries ?? []
  return (
    <Modal
      title={`Restore into ${db ? `${conn.name} / ${db}` : conn.name}`}
      onClose={onClose}
      wide
      footer={
        <>
          <span className="spacer" />
          <button onClick={onClose}>{jobId ? 'Close' : 'Cancel'}</button>
          {!jobId && (
            <button className={drop ? 'danger' : 'primary'} disabled={!ready} onClick={start}>
              {drop ? 'Drop and restore' : dryRun && engine === 'tools' ? 'Dry run' : 'Start restore'}
            </button>
          )}
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {conn.readOnly && <p className="warning-text">This connection is read-only. Edit the connection to allow writes before restoring.</p>}
      <Segmented
        label="Backup format"
        value={engine}
        onChange={setEngine}
        options={[
          { value: 'builtin', label: 'Backup folder', title: 'A folder made by Mango3S or mongodump' },
          { value: 'tools', label: 'Archive file (mongorestore)', title: 'A .archive file made by mongodump' },
        ]}
      />
      {engine === 'tools' && <ToolsNotice tools={tools} need="restore" />}

      {engine === 'builtin' ? (
        <>
          <label className="field">
            <span>Backup folder</span>
            <div className="file-row">
              <input value={dir} readOnly placeholder="Choose a backup folder" />
              <button onClick={async () => inspect((await backend.chooseFile('folder', 'Choose a backup folder')) || dir)}>Choose…</button>
            </div>
          </label>
          {busy && <Spinner small />}
          {inspection && (
            <>
              <p className="muted small">
                {inspection.manifest
                  ? `Made by Mango3S on ${new Date(inspection.manifest.created).toLocaleString()}${inspection.manifest.source ? ` from ${inspection.manifest.source}` : ''}${inspection.manifest.serverVersion ? ` (MongoDB ${inspection.manifest.serverVersion})` : ''}. Document counts are checked after the restore.`
                  : 'A mongodump folder. Document counts cannot be checked against a manifest.'}
              </p>
              {inspection.warnings.map((w) => (
                <p key={w} className="warning-text small">
                  {w}
                </p>
              ))}
              <div className="backup-entries">
                <table className="grid compact">
                  <thead>
                    <tr>
                      <th>
                        <input
                          type="checkbox"
                          aria-label="Select all"
                          checked={picked.size === all.length}
                          onChange={(e) => setPicked(new Set(e.target.checked ? all.map((x) => `${x.db}.${x.coll}`) : []))}
                        />
                      </th>
                      <th>Namespace</th>
                      <th>Type</th>
                      <th>Documents</th>
                      <th>Size</th>
                      <th>Indexes</th>
                    </tr>
                  </thead>
                  <tbody>
                    {all.map((e) => {
                      const k = `${e.db}.${e.coll}`
                      return (
                        <tr key={k}>
                          <td>
                            <input
                              type="checkbox"
                              aria-label={k}
                              checked={picked.has(k)}
                              onChange={(ev) => {
                                const n = new Set(picked)
                                if (ev.target.checked) n.add(k)
                                else n.delete(k)
                                setPicked(n)
                              }}
                            />
                          </td>
                          <td className="mono">{k}</td>
                          <td>{e.type}</td>
                          <td>{e.type === 'view' ? '—' : e.expected !== undefined && e.expected >= 0 ? formatCount(e.expected) : '?'}</td>
                          <td>{e.type === 'view' ? '—' : formatBytes(e.bytes)}</td>
                          <td>{e.type === 'view' ? '—' : e.indexes}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
              <fieldset className="field-group">
                <legend>Restore into</legend>
                {srcDbs.map((d) => (
                  <label key={d} className="field inline">
                    <span className="mono">{d} →</span>
                    <input value={targets[d] ?? d} aria-label={`Target database for ${d}`} onChange={(e) => setTargets({ ...targets, [d]: e.target.value })} />
                  </label>
                ))}
              </fieldset>
            </>
          )}
        </>
      ) : (
        <>
          <label className="field">
            <span>Archive file</span>
            <div className="file-row">
              <input value={archive} readOnly placeholder="Choose a .archive file" />
              <button
                onClick={async () => {
                  const f = (await backend.chooseFile('tool', 'Choose a mongodump archive')) || archive
                  setArchive(f)
                  if (f) setGzip(f.endsWith('.gz'))
                }}
              >
                Choose…
              </button>
            </div>
          </label>
          <div className="field-row">
            <label className="field">
              <span>Only namespaces (comma-separated, * wildcard)</span>
              <input value={nsInclude} onChange={(e) => setNsInclude(e.target.value)} placeholder="e.g. shop.*" />
            </label>
            <label className="field">
              <span>Rename from</span>
              <input value={nsFrom} onChange={(e) => setNsFrom(e.target.value)} placeholder="e.g. shop.*" />
            </label>
            <label className="field">
              <span>to</span>
              <input value={nsTo} onChange={(e) => setNsTo(e.target.value)} placeholder="e.g. shop_restored.*" />
            </label>
          </div>
          <div className="check-row">
            <label className="check">
              <input type="checkbox" checked={gzip} onChange={(e) => setGzip(e.target.checked)} /> Archive is gzip-compressed
            </label>
            <label className="check">
              <input type="checkbox" checked={users} onChange={(e) => setUsers(e.target.checked)} /> Restore users and roles
            </label>
            <label className="check">
              <input type="checkbox" checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} /> Dry run (write nothing)
            </label>
          </div>
        </>
      )}

      <fieldset className="field-group">
        <legend>Existing data</legend>
        <Segmented
          label="Restore mode"
          value={mode}
          onChange={setMode}
          options={[
            { value: 'insert', label: 'Keep existing', title: 'Insert documents; ones whose _id already exists are skipped and counted' },
            ...(engine === 'builtin' ? [{ value: 'upsert' as RestoreMode, label: 'Replace by _id', title: 'Replace documents with the same _id, insert the rest' }] : []),
            { value: 'drop', label: 'Drop collections first', title: 'Deletes each target collection before restoring it' },
          ]}
        />
        <p className="muted small">
          {mode === 'insert' && 'Documents are added. Documents whose _id already exists are left unchanged and counted as skipped.'}
          {mode === 'upsert' && 'Documents with the same _id are replaced by the backup copy; others are inserted. Nothing is deleted.'}
          {mode === 'drop' && 'Each target collection is dropped, with its current data and indexes, then recreated from the backup.'}
        </p>
        <div className="check-row">
          <label className="check">
            <input type="checkbox" checked={skipIndexes} onChange={(e) => setSkipIndexes(e.target.checked)} /> Don't restore indexes
          </label>
          <label className="check">
            <input type="checkbox" checked={skipOptions} onChange={(e) => setSkipOptions(e.target.checked)} /> Don't restore collection options (validators, collation…)
          </label>
          <label className="check">
            <input type="checkbox" checked={bypass} onChange={(e) => setBypass(e.target.checked)} /> Bypass document validation
          </label>
          <label className="check">
            <input type="checkbox" checked={stopOnError} onChange={(e) => setStopOnError(e.target.checked)} /> Stop at the first error
          </label>
          {canShard && (
            <label className="check">
              <input type="checkbox" checked={shardColls} onChange={(e) => setShardColls(e.target.checked)} /> Shard collections again with their saved shard
              keys ({sharded.length})
            </label>
          )}
        </div>
      </fieldset>
      {drop && !jobId && <ConfirmInput text={conn.name} value={confirm} onChange={setConfirm} action="Drop the target collections" />}
      {jobId && <JobStatus jobId={jobId} />}
    </Modal>
  )
}
