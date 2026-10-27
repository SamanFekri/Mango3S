import { CodeBlock } from '../lib/syntax'
import { useCallback, useEffect, useState } from 'react'
import { backend, newOpId } from '../lib/backend'
import { formatBytes, formatCount } from '../lib/format'
import { toAppError, type AppError } from '../lib/errors'
import type { CollectionDetails, CollectionSharding, CreateCollectionOptions, DatabaseDetails, ShardInfo, IndexDetail, IndexOptions, RestoreMode, SessionInfo, Validation, ValidationReport } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Modal, Segmented, Spinner } from './common'
import { ConfirmInput, JobStatus } from './p2common'
import { SearchIndexes } from './SearchIndexes'

const emptyCreate = (): CreateCollectionOptions => ({
  name: '',
  capped: false,
  sizeBytes: 0,
  maxDocs: 0,
  collation: '',
  validator: '',
  validationLevel: '',
  validationAction: '',
  timeField: '',
  metaField: '',
  granularity: '',
  expireAfterSeconds: 0,
  clustered: false,
  storageEngine: '',
})

/** True when only a name is set, so a plain create is enough. */
const plain = (o: CreateCollectionOptions) => {
  const e = emptyCreate()
  return (Object.keys(e) as (keyof CreateCollectionOptions)[]).every((k) => k === 'name' || o[k] === e[k])
}

function DocField({ label, value, onChange, placeholder, rows = 3 }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string; rows?: number }) {
  return (
    <label className="field">
      <span>{label}</span>
      <textarea className="mono" rows={rows} value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} spellCheck={false} aria-label={label} />
    </label>
  )
}

/** Create a database, collection (with options) or view. */
export function CreateCollectionDialog({
  conn,
  db: fixedDb,
  onClose,
  onCreated,
}: {
  conn: SessionInfo
  db?: string
  onClose: () => void
  onCreated: (db: string, name: string) => void
}) {
  const [db, setDb] = useState(fixedDb ?? '')
  const [kind, setKind] = useState<'collection' | 'view'>('collection')
  const [o, setO] = useState<CreateCollectionOptions>(emptyCreate())
  const [more, setMore] = useState(false)
  const [viewOn, setViewOn] = useState('')
  const [pipeline, setPipeline] = useState('[\n  { $match: {} }\n]')
  const [sources, setSources] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const set = (p: Partial<CreateCollectionOptions>) => setO({ ...o, ...p })

  useEffect(() => {
    if (kind !== 'view' || !db.trim()) return
    backend
      .listCollections(conn.id, db.trim())
      .then((l) => setSources(l.filter((c) => !c.system).map((c) => c.name)))
      .catch(() => setSources([]))
  }, [kind, db, conn.id])

  const submit = async () => {
    setBusy(true)
    setError(null)
    const d = db.trim()
    const name = o.name.trim()
    try {
      if (kind === 'view') await backend.createView(conn.id, d, name, viewOn, pipeline, o.collation)
      else if (plain(o)) await backend.createCollection(conn.id, d, name)
      else await backend.createCollectionWithOptions(conn.id, d, { ...o, name })
      onCreated(d, name)
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  const ready = !!db.trim() && !!o.name.trim() && (kind === 'collection' || !!viewOn)
  return (
    <Modal
      title={fixedDb ? `Create in ${fixedDb}` : 'Create database'}
      onClose={onClose}
      wide={more || kind === 'view'}
      footer={
        <>
          <button onClick={onClose}>Cancel</button>
          <button className="primary" disabled={busy || !ready} onClick={submit}>
            {busy && <Spinner small />} Create
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {!fixedDb && (
        <>
          <p className="muted small">MongoDB creates a database together with its first collection.</p>
          <label className="field">
            <span>Database name</span>
            <input value={db} onChange={(e) => setDb(e.target.value)} autoFocus />
          </label>
        </>
      )}
      {fixedDb && (
        <Segmented
          label="What to create"
          value={kind}
          onChange={setKind}
          options={[
            { value: 'collection', label: 'Collection' },
            { value: 'view', label: 'View' },
          ]}
        />
      )}
      <label className="field">
        <span>{kind === 'view' ? 'View name' : 'Collection name'}</span>
        <input value={o.name} onChange={(e) => set({ name: e.target.value })} autoFocus={!!fixedDb} onKeyDown={(e) => e.key === 'Enter' && ready && !more && submit()} />
      </label>
      {kind === 'view' ? (
        <>
          <label className="field">
            <span>Reads from</span>
            <select value={viewOn} onChange={(e) => setViewOn(e.target.value)} aria-label="Reads from">
              <option value="">Choose a collection or view</option>
              {sources.map((s) => (
                <option key={s}>{s}</option>
              ))}
            </select>
          </label>
          <DocField label="Pipeline" value={pipeline} onChange={setPipeline} rows={6} />
          <DocField label="Collation (optional)" value={o.collation} onChange={(v) => set({ collation: v })} placeholder='{ locale: "en", strength: 2 }' rows={1} />
        </>
      ) : (
        <>
          <button className="link" onClick={() => setMore(!more)} aria-expanded={more}>
            {more ? 'Hide options' : 'Options: capped, time series, collation, validation…'}
          </button>
          {more && (
            <>
              <fieldset className="field-group">
                <legend>Type</legend>
                <div className="check-row">
                  <label className="check">
                    <input type="checkbox" checked={o.capped} onChange={(e) => set({ capped: e.target.checked })} /> Capped (fixed size, insertion order)
                  </label>
                  <label className="check" title="MongoDB 5.3 or later">
                    <input type="checkbox" checked={o.clustered} onChange={(e) => set({ clustered: e.target.checked })} /> Clustered on _id
                  </label>
                </div>
                {o.capped && (
                  <div className="field-row">
                    <label className="field">
                      <span>Maximum size (bytes)</span>
                      <input type="number" min={1} value={o.sizeBytes || ''} onChange={(e) => set({ sizeBytes: Number(e.target.value) })} aria-label="Maximum size" />
                    </label>
                    <label className="field">
                      <span>Maximum documents (optional)</span>
                      <input type="number" min={0} value={o.maxDocs || ''} onChange={(e) => set({ maxDocs: Number(e.target.value) })} aria-label="Maximum documents" />
                    </label>
                  </div>
                )}
                <div className="field-row">
                  <label className="field">
                    <span>Time series: time field (MongoDB 5.0+)</span>
                    <input value={o.timeField} onChange={(e) => set({ timeField: e.target.value })} placeholder="e.g. timestamp" aria-label="Time field" />
                  </label>
                  <label className="field">
                    <span>Meta field</span>
                    <input value={o.metaField} onChange={(e) => set({ metaField: e.target.value })} disabled={!o.timeField} aria-label="Meta field" />
                  </label>
                  <label className="field">
                    <span>Granularity</span>
                    <select value={o.granularity} onChange={(e) => set({ granularity: e.target.value })} disabled={!o.timeField} aria-label="Granularity">
                      <option value="">default</option>
                      <option>seconds</option>
                      <option>minutes</option>
                      <option>hours</option>
                    </select>
                  </label>
                  <label className="field">
                    <span>Expire after (seconds)</span>
                    <input
                      type="number"
                      min={0}
                      value={o.expireAfterSeconds || ''}
                      onChange={(e) => set({ expireAfterSeconds: Number(e.target.value) })}
                      disabled={!o.timeField && !o.clustered}
                      aria-label="Expire after"
                    />
                  </label>
                </div>
              </fieldset>
              <DocField label="Collation" value={o.collation} onChange={(v) => set({ collation: v })} placeholder='{ locale: "en", strength: 2 }' rows={1} />
              <DocField label="Validator" value={o.validator} onChange={(v) => set({ validator: v })} placeholder='{ $jsonSchema: { required: ["name"] } }' rows={4} />
              <div className="field-row">
                <label className="field">
                  <span>Validation level</span>
                  <select value={o.validationLevel} onChange={(e) => set({ validationLevel: e.target.value })} aria-label="Validation level">
                    <option value="">default (strict)</option>
                    <option>strict</option>
                    <option>moderate</option>
                    <option>off</option>
                  </select>
                </label>
                <label className="field">
                  <span>Validation action</span>
                  <select value={o.validationAction} onChange={(e) => set({ validationAction: e.target.value })} aria-label="Validation action">
                    <option value="">default (error)</option>
                    <option>error</option>
                    <option>warn</option>
                  </select>
                </label>
              </div>
              <DocField
                label="Storage engine options"
                value={o.storageEngine}
                onChange={(v) => set({ storageEngine: v })}
                placeholder='{ wiredTiger: { configString: "block_compressor=zstd" } }'
                rows={1}
              />
            </>
          )}
        </>
      )}
    </Modal>
  )
}

// ---- collection information -------------------------------------------------------

type InfoTab = 'overview' | 'indexes' | 'validation'

/** Collection metadata, statistics, indexes and validation rules. */
export function CollectionInfoDialog({ conn, db, coll, tab: initial = 'overview', onClose }: { conn: SessionInfo; db: string; coll: string; tab?: InfoTab; onClose: () => void }) {
  const [tab, setTab] = useState<InfoTab>(initial)
  const [details, setDetails] = useState<CollectionDetails | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  useEffect(() => {
    backend
      .collectionDetails(conn.id, db, coll)
      .then(setDetails)
      .catch((err) => setError(toAppError(err)))
  }, [conn.id, db, coll])
  const view = details?.type === 'view'
  return (
    <Modal title={`${db}.${coll}`} onClose={onClose} wide footer={<button onClick={onClose}>Close</button>}>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <Segmented
        label="Section"
        value={tab}
        onChange={setTab}
        options={[
          { value: 'overview', label: 'Overview' },
          ...(view
            ? []
            : [
                { value: 'indexes' as InfoTab, label: 'Indexes' },
                { value: 'validation' as InfoTab, label: 'Validation' },
              ]),
        ]}
      />
      {tab === 'overview' && (details ? <Overview d={details} /> : !error && <Spinner />)}
      {tab === 'indexes' && (
        <>
          <IndexManager conn={conn} db={db} coll={coll} />
          <SearchIndexes conn={conn} db={db} coll={coll} />
        </>
      )}
      {tab === 'validation' && <ValidationEditor conn={conn} db={db} coll={coll} />}
    </Modal>
  )
}

function Overview({ d }: { d: CollectionDetails }) {
  const rows: [string, string][] =
    d.type === 'view'
      ? [['Type', 'view']]
      : [
          ['Type', d.type + (d.capped ? ' (capped)' : '') + (d.sharded ? ' (sharded)' : '')],
          ['Documents', formatCount(d.count)],
          ['Data size', formatBytes(d.size)],
          ['Average document', formatBytes(d.avgObjSize)],
          ['Storage size', formatBytes(d.storageSize)],
          ['Index size', formatBytes(d.totalIndexSize)],
          ...Object.entries(d.stats).map(([k, v]) => [k, v] as [string, string]),
        ]
  if (d.uuid) rows.push(['UUID', d.uuid])
  return (
    <>
      <table className="grid compact">
        <tbody>
          {rows.map(([k, v]) => (
            <tr key={k}>
              <th>{k}</th>
              <td className="mono">{v}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {d.sharding && <CollectionShardingView sh={d.sharding} />}
      <h4>Options</h4>
      <pre className="job-log">{d.options || '{}'}</pre>
      <h4>Recreate with</h4>
      <CodeBlock code={d.shell} lang="mongosh" className="job-log" />
    </>
  )
}

export const emptyIndex = (): IndexOptions => ({
  keys: '{ field: 1 }',
  name: '',
  unique: false,
  sparse: false,
  hidden: false,
  expireAfterSeconds: null,
  partialFilterExpression: '',
  collation: '',
  wildcardProjection: '',
  extra: '',
})

export type IndexDirection = '1' | '-1' | '"text"' | '"hashed"' | '"2dsphere"'

/** Field names that are plain identifiers are written bare; others ("a.b") quoted. */
const keyName = (f: string) => (/^[A-Za-z_$][\w$]*$/.test(f) ? f : JSON.stringify(f))

/** Fields named in an index keys document (best effort, for highlighting). */
export function indexKeyFields(keys: string): string[] {
  const out: string[] = []
  for (const m of keys.matchAll(/(?:^|[{,])\s*(?:"((?:[^"\\]|\\.)*)"|'([^']*)'|([A-Za-z_$][\w$.]*))\s*:/g)) out.push(m[1] ?? m[2] ?? m[3])
  return out
}

/** Adds `field: direction` to an index keys document, replacing the template placeholder. */
export function addIndexKey(keys: string, field: string, dir: IndexDirection): string {
  const t = keys.trim()
  const entry = `${keyName(field)}: ${dir}`
  if (t === '' || /^\{\s*\}$/.test(t) || t === emptyIndex().keys) return `{ ${entry} }`
  if (indexKeyFields(t).includes(field)) return keys
  const end = t.lastIndexOf('}')
  if (end < 0) return `{ ${entry} }`
  return `${t.slice(0, end).trimEnd()}, ${entry} ${t.slice(end)}`
}

/** Sampled field names to click into the index keys. */
function IndexFieldPicker({ conn, db, coll, keys, indexed, onAdd }: { conn: SessionInfo; db: string; coll: string; keys: string; indexed: Set<string>; onAdd: (f: string, d: IndexDirection) => void }) {
  const [fields, setFields] = useState<string[] | null>(null)
  const [dir, setDir] = useState<IndexDirection>('1')
  const [filter, setFilter] = useState('')
  useEffect(() => {
    backend
      .sampleFields(conn.id, db, coll)
      .then((f) => setFields(f.filter((x) => x !== '_id')))
      .catch(() => setFields([]))
  }, [conn.id, db, coll])
  const used = new Set(indexKeyFields(keys))
  const shown = (fields ?? []).filter((f) => f.toLowerCase().includes(filter.trim().toLowerCase()))
  return (
    <div className="field">
      <span>Suggested fields (from a sample of documents): click to add</span>
      <div className="field-row">
        <input value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="Filter fields" aria-label="Filter fields" />
        <select value={dir} onChange={(e) => setDir(e.target.value as IndexDirection)} aria-label="Key type">
          <option value="1">ascending (1)</option>
          <option value="-1">descending (-1)</option>
          <option value='"text"'>text</option>
          <option value='"hashed"'>hashed</option>
          <option value='"2dsphere"'>2dsphere</option>
        </select>
      </div>
      {fields === null ? (
        <Spinner small />
      ) : shown.length === 0 ? (
        <span className="muted small">{fields.length ? 'No field matches.' : 'No fields found in the sample (empty collection?). Type the keys instead.'}</span>
      ) : (
        <div className="chip-list" role="group" aria-label="Suggested index fields">
          {shown.map((f) => (
            <button
              key={f}
              type="button"
              className={'chip' + (used.has(f) ? ' active' : '')}
              disabled={used.has(f)}
              onClick={() => onAdd(f, dir)}
              title={indexed.has(f) ? `${f} already leads an index` : `Add ${f}`}
            >
              {f}
              {indexed.has(f) && <span className="muted small"> · indexed</span>}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

function IndexManager({ conn, db, coll }: { conn: SessionInfo; db: string; coll: string }) {
  const { confirm, toast, prompt } = useApp()
  const [list, setList] = useState<IndexDetail[] | null>(null)
  const [form, setForm] = useState<IndexOptions | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const load = useCallback(() => {
    backend
      .listIndexDetails(conn.id, db, coll)
      .then(setList)
      .catch((err) => setError(toAppError(err)))
  }, [conn.id, db, coll])
  useEffect(load, [load])
  const act = async (fn: () => Promise<unknown>, done: string) => {
    setBusy(true)
    setError(null)
    try {
      await fn()
      toast({ kind: 'success', title: done })
      load()
      return true
    } catch (err) {
      setError(toAppError(err))
      return false
    } finally {
      setBusy(false)
    }
  }
  const drop = async (ix: IndexDetail) => {
    const ok = await confirm({
      title: 'Drop index',
      message: (
        <>
          Drop index <code>{ix.name}</code> on {db}.{coll}? Queries that use it will slow down{ix.unique ? ', and uniqueness will no longer be enforced' : ''}. Consider hiding it first
          to see the effect.
        </>
      ),
      confirmLabel: 'Drop index',
      danger: true,
      typeToConfirm: ix.name,
    })
    if (ok) await act(() => backend.dropIndex(conn.id, db, coll, ix.name), `Dropped index ${ix.name}`)
  }
  const editTTL = async (ix: IndexDetail) => {
    const v = await prompt({ title: `TTL of ${ix.name}`, label: 'Expire documents after (seconds)', initial: String(ix.ttl ?? ''), confirmLabel: 'Save' })
    if (v === null) return
    const n = Number(v)
    if (!Number.isInteger(n) || n < 0) {
      setError({ code: 'INVALID_INPUT', message: 'Enter a whole number of seconds.' })
      return
    }
    await act(() => backend.modifyIndex(conn.id, db, coll, ix.name, { ttl: n }), `TTL of ${ix.name} set to ${n} s`)
  }
  const set = (p: Partial<IndexOptions>) => form && setForm({ ...form, ...p })
  return (
    <>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {!list ? (
        <Spinner />
      ) : (
        <table className="grid compact">
          <thead>
            <tr>
              <th>Name</th>
              <th>Keys</th>
              <th>Properties</th>
              <th>Size</th>
              <th>Uses</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {list.map((ix) => (
              <tr key={ix.name}>
                <td className="mono" title={ix.spec}>
                  {ix.name}
                </td>
                <td className="mono">{ix.keys}</td>
                <td className="small">
                  {[
                    ix.unique && 'unique',
                    ix.sparse && 'sparse',
                    ix.hidden && 'hidden',
                    ix.ttl !== undefined && ix.ttl !== null && `TTL ${ix.ttl}s`,
                    ix.partial && `partial ${ix.partial}`,
                    ix.collation && `collation ${ix.collation}`,
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </td>
                <td>{formatBytes(ix.size)}</td>
                <td title={ix.accessSince ? `since ${ix.accessSince}` : undefined}>{formatCount(ix.accesses)}</td>
                <td className="row-buttons">
                  {ix.name !== '_id_' && !conn.readOnly && (
                    <>
                      <button
                        className="small ghost"
                        disabled={busy}
                        onClick={() => act(() => backend.modifyIndex(conn.id, db, coll, ix.name, { hidden: !ix.hidden }), `${ix.hidden ? 'Unhid' : 'Hid'} ${ix.name}`)}
                        title="Hidden indexes are maintained but not used by queries (MongoDB 4.4+)"
                      >
                        {ix.hidden ? 'Unhide' : 'Hide'}
                      </button>
                      {ix.ttl !== undefined && ix.ttl !== null && (
                        <button className="small ghost" disabled={busy} onClick={() => editTTL(ix)}>
                          TTL…
                        </button>
                      )}
                      <button className="small ghost danger-text" disabled={busy} onClick={() => drop(ix)} aria-label={`Drop ${ix.name}`}>
                        Drop
                      </button>
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {!conn.readOnly &&
        (form ? (
          <fieldset className="field-group">
            <legend>New index</legend>
            <IndexFieldPicker
              conn={conn}
              db={db}
              coll={coll}
              keys={form.keys}
              indexed={new Set((list ?? []).map((ix) => indexKeyFields(ix.keys)[0]).filter(Boolean))}
              onAdd={(f, d) => set({ keys: addIndexKey(form.keys, f, d) })}
            />
            <div className="field-row">
              <DocField label="Keys" value={form.keys} onChange={(v) => set({ keys: v })} rows={2} />
              <label className="field">
                <span>Name (optional)</span>
                <input value={form.name} onChange={(e) => set({ name: e.target.value })} aria-label="Index name" />
              </label>
            </div>
            <div className="check-row">
              <label className="check">
                <input type="checkbox" checked={form.unique} onChange={(e) => set({ unique: e.target.checked })} /> Unique
              </label>
              <label className="check">
                <input type="checkbox" checked={form.sparse} onChange={(e) => set({ sparse: e.target.checked })} /> Sparse
              </label>
              <label className="check">
                <input type="checkbox" checked={form.hidden} onChange={(e) => set({ hidden: e.target.checked })} /> Hidden
              </label>
              <label className="check">
                <input type="checkbox" checked={form.expireAfterSeconds !== null} onChange={(e) => set({ expireAfterSeconds: e.target.checked ? 3600 : null })} /> TTL
              </label>
              {form.expireAfterSeconds !== null && (
                <input
                  type="number"
                  min={0}
                  value={form.expireAfterSeconds}
                  onChange={(e) => set({ expireAfterSeconds: Number(e.target.value) })}
                  aria-label="Expire after seconds"
                />
              )}
            </div>
            <div className="field-row">
              <DocField label="Partial filter" value={form.partialFilterExpression} onChange={(v) => set({ partialFilterExpression: v })} placeholder="{ status: { $exists: true } }" rows={1} />
              <DocField label="Collation" value={form.collation} onChange={(v) => set({ collation: v })} placeholder='{ locale: "en", strength: 2 }' rows={1} />
            </div>
            <DocField label="Other options" value={form.extra} onChange={(v) => set({ extra: v })} placeholder='e.g. { weights: { title: 10 }, default_language: "english" }' rows={1} />
            <div className="row-buttons">
              <button onClick={() => setForm(null)}>Cancel</button>
              <button
                className="primary"
                disabled={busy}
                onClick={async () => {
                  if (await act(() => backend.createIndex(newOpId('index'), conn.id, db, coll, form), 'Index created')) setForm(null)
                }}
              >
                {busy ? <Spinner small /> : 'Create index'}
              </button>
            </div>
            <p className="muted small">Building an index on a large collection takes time and uses server resources.</p>
          </fieldset>
        ) : (
          <button onClick={() => setForm(emptyIndex())}>New index…</button>
        ))}
    </>
  )
}

function ValidationEditor({ conn, db, coll }: { conn: SessionInfo; db: string; coll: string }) {
  const { toast, confirm } = useApp()
  const [v, setV] = useState<Validation | null>(null)
  const [report, setReport] = useState<ValidationReport | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  useEffect(() => {
    backend
      .getValidation(conn.id, db, coll)
      .then(setV)
      .catch((err) => setError(toAppError(err)))
  }, [conn.id, db, coll])
  if (!v) return error ? <ErrorBanner error={error} /> : <Spinner />
  const check = async () => {
    setBusy(true)
    setError(null)
    try {
      setReport(await backend.checkValidator(conn.id, db, coll, v.validator))
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  const save = async () => {
    if (!v.validator.trim()) {
      const ok = await confirm({ title: 'Remove validation', message: `Remove all validation rules from ${db}.${coll}?`, confirmLabel: 'Remove', danger: true })
      if (!ok) return
    }
    setBusy(true)
    setError(null)
    try {
      await backend.setValidation(conn.id, db, coll, v)
      toast({ kind: 'success', title: 'Validation rules saved' })
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <DocField
        label="Validator (empty = no validation)"
        value={v.validator}
        onChange={(x) => {
          setV({ ...v, validator: x })
          setReport(null)
        }}
        placeholder='{ $jsonSchema: { bsonType: "object", required: ["name"], properties: { name: { bsonType: "string" } } } }'
        rows={10}
      />
      <div className="field-row">
        <label className="field">
          <span>Level</span>
          <select value={v.level} onChange={(e) => setV({ ...v, level: e.target.value as Validation['level'] })} aria-label="Validation level">
            <option value="strict">strict: all inserts and updates</option>
            <option value="moderate">moderate: skip documents that are already invalid</option>
            <option value="off">off</option>
          </select>
        </label>
        <label className="field">
          <span>Action</span>
          <select value={v.action} onChange={(e) => setV({ ...v, action: e.target.value as Validation['action'] })} aria-label="Validation action">
            <option value="error">error: reject invalid writes</option>
            <option value="warn">warn: allow and log</option>
            <option value="errorAndLog">errorAndLog (MongoDB 8.1+)</option>
          </select>
        </label>
      </div>
      {report && (
        <p className={report.failing > 0 ? 'warning-text small' : 'muted small'}>
          {report.failing > 0
            ? `${formatCount(report.failing)} of ${formatCount(report.checked)} existing documents don't match (e.g. _id ${report.sample.join(', ')}).`
            : `All ${formatCount(report.checked)} existing documents match.`}
        </p>
      )}
      {!conn.readOnly && (
        <div className="row-buttons">
          <button onClick={check} disabled={busy || !v.validator.trim()}>
            Check existing documents
          </button>
          <button className="primary" onClick={save} disabled={busy}>
            Save rules
          </button>
        </div>
      )}
    </>
  )
}

// ---- rename / copy ----------------------------------------------------------------

/** Rename a collection, optionally into another database on the same server. */
export function RenameCollectionDialog({ conn, db, coll, onClose, onDone }: { conn: SessionInfo; db: string; coll: string; onClose: () => void; onDone: () => void }) {
  const [toDb, setToDb] = useState(db)
  const [to, setTo] = useState(coll)
  const [dropTarget, setDropTarget] = useState(false)
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<AppError | null>(null)
  const [busy, setBusy] = useState(false)
  const target = `${toDb.trim()}.${to.trim()}`
  const run = async () => {
    setBusy(true)
    setError(null)
    try {
      await backend.renameCollection(conn.id, db, coll, toDb.trim(), to.trim(), dropTarget, confirm)
      onDone()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal
      title={`Rename ${db}.${coll}`}
      onClose={onClose}
      footer={
        <>
          <button onClick={onClose}>Cancel</button>
          <button className={dropTarget ? 'danger' : 'primary'} disabled={busy || !to.trim() || target === `${db}.${coll}` || (dropTarget && confirm !== target)} onClick={run}>
            Rename
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <div className="field-row">
        <label className="field">
          <span>Database</span>
          <input value={toDb} onChange={(e) => setToDb(e.target.value)} aria-label="Target database" />
        </label>
        <label className="field">
          <span>New name</span>
          <input value={to} onChange={(e) => setTo(e.target.value)} aria-label="New name" autoFocus />
        </label>
      </div>
      <p className="muted small">Indexes, options and validation move with the collection. Views and time series collections can't be renamed by the server.</p>
      <label className="check">
        <input type="checkbox" checked={dropTarget} onChange={(e) => setDropTarget(e.target.checked)} /> Replace an existing collection with that name
      </label>
      {dropTarget && <ConfirmInput text={target} value={confirm} onChange={setConfirm} action={`Delete ${target}`} />}
    </Modal>
  )
}

/** Rename a database by moving every collection and view (a background job). */
export function RenameDatabaseDialog({ conn, db, onClose }: { conn: SessionInfo; db: string; onClose: () => void }) {
  const [to, setTo] = useState('')
  const [confirm, setConfirm] = useState('')
  const [jobId, setJobId] = useState<string | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  const run = async () => {
    setError(null)
    try {
      setJobId((await backend.startRenameDatabase(conn.id, db, to.trim(), confirm)).id)
    } catch (err) {
      setError(toAppError(err))
    }
  }
  return (
    <Modal
      title={`Rename database ${db}`}
      onClose={onClose}
      footer={
        <>
          <button onClick={onClose}>{jobId ? 'Close' : 'Cancel'}</button>
          {!jobId && (
            <button className="primary" disabled={!to.trim() || to.trim() === db || confirm !== db} onClick={run}>
              Rename
            </button>
          )}
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <label className="field">
        <span>New name</span>
        <input value={to} onChange={(e) => setTo(e.target.value)} aria-label="New database name" autoFocus />
      </label>
      <p className="muted small">
        MongoDB has no database rename: each collection is moved with its indexes and options, views are recreated, then the old database is removed. Users and roles defined on
        the old database are not moved. Applications using the old name stop finding the data.
      </p>
      <ConfirmInput text={db} value={confirm} onChange={setConfirm} action="Move every collection" />
      {jobId && <JobStatus jobId={jobId} />}
    </Modal>
  )
}

/** Copy a collection or a whole database, to the same or another open connection. */
export function CopyDialog({ conn, db, coll, onClose }: { conn: SessionInfo; db: string; coll?: string; onClose: () => void }) {
  const { sessions } = useApp()
  const [dst, setDst] = useState(conn.id)
  const [dstDb, setDstDb] = useState(coll ? db : `${db}_copy`)
  const [dstColl, setDstColl] = useState(coll ? `${coll}_copy` : '')
  const [mode, setMode] = useState<RestoreMode>('insert')
  const [filter, setFilter] = useState('')
  const [skipIndexes, setSkipIndexes] = useState(false)
  const [skipOptions, setSkipOptions] = useState(false)
  const [views, setViews] = useState(true)
  const [shardColls, setShardColls] = useState(false)
  const [confirm, setConfirm] = useState('')
  const [jobId, setJobId] = useState<string | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  const target = sessions.find((s) => s.id === dst)
  const canShard = conn.topology === 'sharded' && target?.topology === 'sharded'
  const run = async () => {
    setError(null)
    try {
      const j = await backend.startCopy(
        {
          srcConn: conn.id,
          dstConn: dst,
          options: { srcDb: db, collections: coll ? [coll] : [], dstDb: dstDb.trim(), dstColl: coll ? dstColl.trim() : '', mode, filter: filter.trim(), skipIndexes, skipOptions, views, shardCollections: canShard && shardColls },
        },
        confirm,
      )
      setJobId(j.id)
    } catch (err) {
      setError(toAppError(err))
    }
  }
  const same = dst === conn.id && dstDb.trim() === db && (!coll || dstColl.trim() === coll)
  return (
    <Modal
      title={`Copy ${coll ? `${db}.${coll}` : `database ${db}`}`}
      onClose={onClose}
      wide
      footer={
        <>
          <button onClick={onClose}>{jobId ? 'Close' : 'Cancel'}</button>
          {!jobId && (
            <button
              className={mode === 'drop' ? 'danger' : 'primary'}
              disabled={same || !dstDb.trim() || (!!coll && !dstColl.trim()) || target?.readOnly || (mode === 'drop' && confirm !== dstDb.trim())}
              onClick={run}
            >
              Copy
            </button>
          )}
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <div className="field-row">
        <label className="field">
          <span>To connection</span>
          <select value={dst} onChange={(e) => setDst(e.target.value)} aria-label="Target connection">
            {sessions.map((s) => (
              <option key={s.id} value={s.id} disabled={s.readOnly}>
                {s.name}
                {s.readOnly ? ' (read-only)' : ''}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>Database</span>
          <input value={dstDb} onChange={(e) => setDstDb(e.target.value)} aria-label="Target database" />
        </label>
        {coll && (
          <label className="field">
            <span>Collection</span>
            <input value={dstColl} onChange={(e) => setDstColl(e.target.value)} aria-label="Target collection" />
          </label>
        )}
      </div>
      {same && <p className="warning-text small">Choose another database or name: a collection can't be copied onto itself.</p>}
      <label className="field">
        <span>Only documents matching (optional, Extended JSON)</span>
        <input className="mono" value={filter} onChange={(e) => setFilter(e.target.value)} placeholder='{ "status": "active" }' aria-label="Filter" />
      </label>
      <Segmented
        label="Existing data"
        value={mode}
        onChange={setMode}
        options={[
          { value: 'insert', label: 'Keep existing', title: 'Documents whose _id exists in the target are skipped' },
          { value: 'upsert', label: 'Replace by _id' },
          { value: 'drop', label: 'Drop target first' },
        ]}
      />
      <div className="check-row">
        <label className="check">
          <input type="checkbox" checked={!skipIndexes} onChange={(e) => setSkipIndexes(!e.target.checked)} /> Copy indexes
        </label>
        <label className="check">
          <input type="checkbox" checked={!skipOptions} onChange={(e) => setSkipOptions(!e.target.checked)} /> Copy options and validation
        </label>
        {canShard && (
          <label className="check">
            <input type="checkbox" checked={shardColls} onChange={(e) => setShardColls(e.target.checked)} /> Shard copied collections with their source shard key
          </label>
        )}
        {!coll && (
          <label className="check">
            <input type="checkbox" checked={views} onChange={(e) => setViews(e.target.checked)} /> Copy views
          </label>
        )}
      </div>
      {mode === 'drop' && !jobId && <ConfirmInput text={dstDb.trim()} value={confirm} onChange={setConfirm} action="Drop the target collections" />}
      {jobId && <JobStatus jobId={jobId} />}
    </Modal>
  )
}

/** Database statistics (dbStats). */
export function DatabaseInfoDialog({ conn, db, onClose }: { conn: SessionInfo; db: string; onClose: () => void }) {
  const [d, setD] = useState<DatabaseDetails | null>(null)
  const [error, setError] = useState<AppError | null>(null)
  useEffect(() => {
    backend
      .databaseDetails(conn.id, db)
      .then(setD)
      .catch((err) => setError(toAppError(err)))
  }, [conn.id, db])
  const rows: [string, string][] = d
    ? [
        ['Collections', formatCount(d.collections)],
        ['Views', formatCount(d.views)],
        ['Documents', formatCount(d.objects)],
        ['Data size', formatBytes(d.dataSize)],
        ['Average document', formatBytes(d.avgObjSize)],
        ['Storage size', formatBytes(d.storageSize)],
        ['Indexes', formatCount(d.indexes)],
        ['Index size', formatBytes(d.indexSize)],
        ...(d.fsTotalSize ? ([['Disk used', `${formatBytes(d.fsUsedSize)} of ${formatBytes(d.fsTotalSize)}`]] as [string, string][]) : []),
      ]
    : []
  return (
    <Modal title={`Database ${db}`} onClose={onClose} footer={<button onClick={onClose}>Close</button>}>
      <ErrorBanner error={error} />
      {!d && !error && <Spinner />}
      {d && (
        <table className="grid compact">
          <tbody>
            {rows.map(([k, v]) => (
              <tr key={k}>
                <th>{k}</th>
                <td className="mono">{v}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {d?.sharding && (
        <>
          <h4>Sharding</h4>
          <table className="grid compact">
            <tbody>
              <tr>
                <th>Primary shard</th>
                <td className="mono">{d.sharding.primary || '—'}</td>
              </tr>
              <tr>
                <th>Sharded collections</th>
                <td className="mono">{d.sharding.shardedCollections.join(', ') || 'none'}</td>
              </tr>
              <tr>
                <th>Balancer</th>
                <td>
                  {d.sharding.balancer.problem
                    ? d.sharding.balancer.problem
                    : `${d.sharding.balancer.mode === 'off' ? 'off' : 'on'}${d.sharding.balancer.inBalancerRound ? ', balancing now' : ''} · ${formatCount(d.sharding.balancer.numBalancerRounds)} rounds`}
                </td>
              </tr>
            </tbody>
          </table>
          <ShardList connId={conn.id} shards={d.sharding.shards} />
        </>
      )}
    </Modal>
  )
}

function CollectionShardingView({ sh }: { sh: CollectionSharding }) {
  return (
    <>
      <h4>Sharding</h4>
      <table className="grid compact">
        <tbody>
          <tr>
            <th>Shard key</th>
            <td className="mono">
              {sh.key}
              {sh.unique && ' (unique)'}
            </td>
          </tr>
          <tr>
            <th>Chunks</th>
            <td>
              {formatCount(sh.chunks)}
              {sh.jumbo > 0 && <span className="warning-text"> · {formatCount(sh.jumbo)} jumbo (too big to move)</span>}
            </td>
          </tr>
          <tr>
            <th>Balancing</th>
            <td>{sh.balancing ? 'on' : 'off for this collection'}</td>
          </tr>
        </tbody>
      </table>
      {sh.byShard.length > 0 && (
        <table className="grid compact" aria-label="Chunks per shard">
          <thead>
            <tr>
              <th>Shard</th>
              <th>Chunks</th>
              <th>Jumbo</th>
            </tr>
          </thead>
          <tbody>
            {sh.byShard.map((r) => (
              <tr key={r.shard}>
                <td className="mono">{r.shard}</td>
                <td>{formatCount(r.chunks)}</td>
                <td className={r.jumbo ? 'warning-text' : ''}>{formatCount(r.jumbo)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}

/** The cluster's shards, each with a button that connects straight to it. */
export function ShardList({ connId, shards }: { connId: string; shards: ShardInfo[] }) {
  const { connectToShard } = useApp()
  const [busy, setBusy] = useState<string | null>(null)
  return (
    <table className="grid compact" aria-label="Shards">
      <thead>
        <tr>
          <th>Shard</th>
          <th>Hosts</th>
          <th />
        </tr>
      </thead>
      <tbody>
        {shards.map((sh) => (
          <tr key={sh.id}>
            <td className="mono">
              {sh.id}
              {sh.draining && <span className="warning-text small"> draining</span>}
            </td>
            <td className="mono small">{sh.host}</td>
            <td>
              <button
                className="small"
                disabled={busy !== null}
                onClick={async () => {
                  setBusy(sh.id)
                  await connectToShard(connId, sh.id)
                  setBusy(null)
                }}
                aria-label={`Connect to shard ${sh.id}`}
              >
                {busy === sh.id ? <Spinner small /> : 'Connect'}
              </button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
