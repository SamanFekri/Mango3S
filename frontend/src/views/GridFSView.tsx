import { useCallback, useEffect, useRef, useState } from 'react'
import { ErrorBanner, Spinner } from '../components/common'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import { formatBytes, formatCount } from '../lib/format'
import type { Tab } from '../lib/tabs'
import type { GridFSBucket, GridFSFile, GridFSPage, GridFSPreview } from '../lib/types'
import { useApp } from '../state/AppContext'

const PAGE_SIZE = 50

/** Browses a database's GridFS buckets: files page by page, upload and
 * download as background jobs, preview, rename and delete. */
export function GridFSView({ tab }: { tab: Tab }) {
  const connId = tab.connId!
  const db = tab.db!
  const { sessions, confirm, prompt, toast, jobs } = useApp()
  const readOnly = sessions.find((s) => s.id === connId)?.readOnly ?? true
  const [buckets, setBuckets] = useState<GridFSBucket[] | null>(null)
  const [bucket, setBucket] = useState(tab.coll ?? '')
  const [name, setName] = useState('')
  const [sort, setSort] = useState<'uploadDate' | 'filename' | 'length'>('uploadDate')
  const [page, setPage] = useState(0)
  const [data, setData] = useState<GridFSPage | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [selected, setSelected] = useState<GridFSFile | null>(null)
  const [preview, setPreview] = useState<GridFSPreview | null>(null)
  const [previewing, setPreviewing] = useState(false)
  const myJobs = useRef<Set<string>>(new Set())

  useEffect(() => {
    backend
      .listGridFSBuckets(connId, db)
      .then((b) => {
        setBuckets(b)
        if (!bucket && b.length) setBucket(b[0]!.name)
      })
      .catch((err) => setError(toAppError(err)))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connId, db])

  const load = useCallback(async () => {
    if (!bucket) return
    setLoading(true)
    setError(null)
    try {
      setData(await backend.listGridFSFiles({ connId, db, bucket, name, sort, page, pageSize: PAGE_SIZE }))
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setLoading(false)
    }
  }, [connId, db, bucket, name, sort, page])
  useEffect(() => {
    void load()
  }, [load])

  // Reload when one of our uploads finishes.
  const finished = jobs.filter((j) => myJobs.current.has(j.id) && (j.state === 'succeeded' || j.state === 'failed' || j.state === 'canceled'))
  const finishedKey = finished.map((j) => j.id).join(',')
  useEffect(() => {
    if (!finishedKey) return
    for (const j of finished) myJobs.current.delete(j.id)
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [finishedKey])

  const select = async (f: GridFSFile) => {
    setSelected(f)
    setPreview(null)
    setPreviewing(true)
    try {
      setPreview(await backend.previewGridFSFile(connId, db, bucket, f.id))
    } catch (err) {
      setPreview({ kind: 'none', contentType: '', truncated: false, reason: toAppError(err).message })
    } finally {
      setPreviewing(false)
    }
  }

  const upload = async () => {
    try {
      const path = await backend.chooseGridFSUploadFile()
      if (!path) return
      const j = await backend.startGridFSUpload(connId, db, bucket || 'fs', path)
      myJobs.current.add(j.id)
      toast({ kind: 'info', title: 'Upload started', message: 'Progress is in the Jobs panel.' })
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const download = async (f: GridFSFile) => {
    try {
      const j = await backend.startGridFSDownload(connId, db, bucket, f.id, f.filename)
      if (j.id) toast({ kind: 'info', title: `Downloading ${f.filename}`, message: 'Progress is in the Jobs panel.' })
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const rename = async (f: GridFSFile) => {
    const next = await prompt({ title: 'Rename file', label: 'New name', initial: f.filename, confirmLabel: 'Rename' })
    if (!next || next === f.filename) return
    try {
      await backend.renameGridFSFile(connId, db, bucket, f.id, next)
      await load()
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const remove = async (f: GridFSFile) => {
    const ok = await confirm({
      title: 'Delete file',
      message: `Delete ${f.filename} (${formatBytes(f.length)}) from ${db}.${bucket}? Its chunks are deleted too; this can't be undone.`,
      confirmLabel: 'Delete file',
      danger: true,
      typeToConfirm: f.filename,
    })
    if (!ok) return
    try {
      await backend.deleteGridFSFile(connId, db, bucket, f.id)
      if (selected?.id === f.id) {
        setSelected(null)
        setPreview(null)
      }
      await load()
    } catch (err) {
      setError(toAppError(err))
    }
  }

  const pages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1
  return (
    <div className="view gridfs-view">
      <div className="view-header">
        <div className="breadcrumbs">
          <span>{tab.connName}</span>
          <span className="sep">/</span>
          <span>{db}</span>
          <span className="muted">· GridFS</span>
        </div>
        <label className="inline">
          Bucket
          <select value={bucket} onChange={(e) => (setBucket(e.target.value), setPage(0), setSelected(null))} aria-label="Bucket">
            {(buckets ?? []).map((b) => (
              <option key={b.name} value={b.name}>
                {b.name} ({formatCount(b.files)} files, {formatBytes(b.chunkBytes)})
              </option>
            ))}
            {buckets?.length === 0 && <option value="">No buckets</option>}
          </select>
        </label>
        <input placeholder="File name contains" value={name} onChange={(e) => (setName(e.target.value), setPage(0))} aria-label="File name filter" />
        <select value={sort} onChange={(e) => setSort(e.target.value as typeof sort)} aria-label="Sort">
          <option value="uploadDate">Newest first</option>
          <option value="filename">Name</option>
          <option value="length">Largest first</option>
        </select>
        <span className="spacer" />
        {!readOnly && (
          <button className="primary" onClick={upload} title={bucket ? undefined : 'Creates the "fs" bucket'}>
            Upload…
          </button>
        )}
        <button onClick={() => void load()}>{loading ? <Spinner small /> : 'Refresh'}</button>
      </div>
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      {buckets?.length === 0 && <div className="empty-state">This database has no GridFS buckets yet.{!readOnly && ' Upload a file to create the "fs" bucket.'}</div>}
      <div className="gridfs-body">
        <div className="table-wrap">
          {data && (
            <table className="grid compact">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Size</th>
                  <th>Type</th>
                  <th>Uploaded</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {data.files.map((f) => (
                  <tr key={f.id} className={selected?.id === f.id ? 'selected' : ''} onClick={() => void select(f)}>
                    <td title={f.id}>{f.filename}</td>
                    <td>{formatBytes(f.length)}</td>
                    <td className="small muted">{f.contentType || '—'}</td>
                    <td className="small">{new Date(f.uploadDate).toLocaleString()}</td>
                    <td className="row-buttons" onClick={(e) => e.stopPropagation()}>
                      <button className="small ghost" onClick={() => void download(f)} aria-label={`Download ${f.filename}`}>
                        Download…
                      </button>
                      {!readOnly && (
                        <>
                          <button className="small ghost" onClick={() => void rename(f)} aria-label={`Rename ${f.filename}`}>
                            Rename
                          </button>
                          <button className="small ghost danger-text" onClick={() => void remove(f)} aria-label={`Delete ${f.filename}`}>
                            Delete
                          </button>
                        </>
                      )}
                    </td>
                  </tr>
                ))}
                {data.files.length === 0 && (
                  <tr>
                    <td colSpan={5} className="muted">
                      No files{name && ' match'}.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          )}
          {data && data.total > PAGE_SIZE && (
            <div className="pager">
              <button className="small" disabled={page === 0} onClick={() => setPage(page - 1)}>
                ‹ Previous
              </button>
              <span className="small">
                Page {page + 1} of {pages} · {formatCount(data.total)} files
              </span>
              <button className="small" disabled={page + 1 >= pages} onClick={() => setPage(page + 1)}>
                Next ›
              </button>
            </div>
          )}
        </div>
        {selected && (
          <aside className="gridfs-preview" aria-label="Preview">
            <h3>{selected.filename}</h3>
            <dl className="stats small">
              <dt>ID</dt>
              <dd className="mono">{selected.id}</dd>
              <dt>Size</dt>
              <dd>
                {formatBytes(selected.length)} in {formatBytes(selected.chunkSize)} chunks
              </dd>
              {selected.metadata && (
                <>
                  <dt>Metadata</dt>
                  <dd className="mono">{selected.metadata}</dd>
                </>
              )}
            </dl>
            {previewing && <Spinner />}
            {preview?.kind === 'image' && <img className="gridfs-image" src={preview.dataUrl} alt={selected.filename} />}
            {preview?.kind === 'text' && (
              <>
                <pre className="doc-text small gridfs-text">{preview.text}</pre>
                {preview.truncated && <p className="muted small">Showing the first 256 KB.</p>}
              </>
            )}
            {preview?.kind === 'none' && <p className="muted small">{preview.reason}</p>}
          </aside>
        )}
      </div>
    </div>
  )
}
