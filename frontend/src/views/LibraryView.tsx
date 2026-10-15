import { useCallback, useEffect, useState } from 'react'
import { Segmented, Spinner } from '../components/common'
import { backend } from '../lib/backend'
import { formatDuration } from '../lib/format'
import { KIND_LABELS, openAction, timeAgo } from '../lib/library'
import type { HistoryEntry, ItemKind, SavedItem } from '../lib/types'
import { useApp } from '../state/AppContext'

type Section = 'saved' | 'history'

export function LibraryView() {
  const { sessions, dispatchTabs, toast, toastError, confirm, prompt } = useApp()
  const [section, setSection] = useState<Section>('saved')
  const [kind, setKind] = useState<ItemKind | ''>('')
  const [search, setSearch] = useState('')
  const [saved, setSaved] = useState<SavedItem[]>([])
  const [history, setHistory] = useState<HistoryEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [connId, setConnId] = useState('')

  const session = sessions.find((s) => s.id === connId) ?? sessions[0]

  const load = useCallback(async () => {
    setLoading(true)
    try {
      if (section === 'saved') setSaved(await backend.listSaved(kind))
      else setHistory(await backend.listHistory(kind, search, 300))
    } catch (err) {
      toastError(err, 'Could not load the library')
    } finally {
      setLoading(false)
    }
  }, [section, kind, search, toastError])

  useEffect(() => {
    const id = window.setTimeout(load, 150)
    return () => window.clearTimeout(id)
  }, [load])

  const open = (item: SavedItem | HistoryEntry, asSaved?: SavedItem) => {
    if (!session) {
      toast({ kind: 'info', title: 'Connect first', message: 'Open a connection to run this item.' })
      return
    }
    const action = openAction(item, session, asSaved)
    if (typeof action === 'string') toast({ kind: 'error', title: 'Cannot open', message: action })
    else dispatchTabs(action)
  }

  const remove = async (item: SavedItem) => {
    if (!(await confirm({ title: 'Delete saved item', message: `Delete "${item.name}"?`, confirmLabel: 'Delete', danger: true }))) return
    try {
      await backend.deleteItem(item.id)
      load()
    } catch (err) {
      toastError(err)
    }
  }

  const rename = async (item: SavedItem) => {
    const name = await prompt({ title: 'Rename', label: 'Name', initial: item.name, confirmLabel: 'Rename' })
    if (!name) return
    try {
      await backend.saveItem({ ...item, name })
      load()
    } catch (err) {
      toastError(err)
    }
  }

  const saveFromHistory = async (e: HistoryEntry) => {
    const name = await prompt({ title: 'Save to library', label: 'Name', initial: e.summary.slice(0, 60), confirmLabel: 'Save' })
    if (!name) return
    try {
      await backend.saveItem({ kind: e.kind, name, db: e.db, coll: e.coll, query: e.query, stages: e.stages, text: e.text })
      toast({ kind: 'success', title: `Saved "${name}"` })
    } catch (err) {
      toastError(err, 'Save failed')
    }
  }

  const clearHistory = async () => {
    if (!(await confirm({ title: 'Clear history', message: 'Delete the entire query history?', confirmLabel: 'Clear', danger: true }))) return
    await backend.clearHistory()
    load()
  }

  const shownSaved = search
    ? saved.filter((s) => `${s.name} ${s.description} ${s.db}.${s.coll} ${s.text ?? ''}`.toLowerCase().includes(search.toLowerCase()))
    : saved

  return (
    <div className="view library-view">
      <div className="view-header">
        <h1>Library</h1>
        <span className="spacer" />
        {sessions.length > 1 && (
          <label className="inline">
            Open on
            <select value={session?.id ?? ''} onChange={(e) => setConnId(e.target.value)}>
              {sessions.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </label>
        )}
      </div>
      <div className="library-toolbar">
        <Segmented
          label="Library section"
          value={section}
          onChange={setSection}
          options={[
            { value: 'saved', label: 'Saved' },
            { value: 'history', label: 'History' },
          ]}
        />
        <select value={kind} onChange={(e) => setKind(e.target.value as ItemKind | '')} aria-label="Type">
          <option value="">All types</option>
          {(Object.keys(KIND_LABELS) as ItemKind[]).map((k) => (
            <option key={k} value={k}>
              {KIND_LABELS[k]}
            </option>
          ))}
        </select>
        <input placeholder="Search" value={search} onChange={(e) => setSearch(e.target.value)} aria-label="Search library" />
        {loading && <Spinner small />}
        <span className="spacer" />
        {section === 'history' && history.length > 0 && (
          <button className="small ghost danger-text" onClick={clearHistory}>
            Clear history
          </button>
        )}
      </div>

      {section === 'saved' ? (
        <ul className="library-list">
          {shownSaved.length === 0 && !loading && (
            <li className="empty-state">Nothing saved yet. Use "Save" in a query, pipeline, shell or SQL tab.</li>
          )}
          {shownSaved.map((it) => (
            <li key={it.id} className="library-item">
              <div className="library-main" onDoubleClick={() => open(it, it)}>
                <div>
                  <span className={`kind-badge kind-${it.kind}`}>{KIND_LABELS[it.kind]}</span>
                  <strong>{it.name}</strong>
                </div>
                <div className="muted small mono">
                  {[it.db && it.coll ? `${it.db}.${it.coll}` : it.db, it.kind === 'aggregation' && `${it.stages?.length ?? 0} stages`]
                    .filter(Boolean)
                    .join(' · ')}
                  {it.query?.filter && ` · ${it.query.filter}`}
                  {it.text && ` · ${it.text.split('\n')[0].slice(0, 80)}`}
                </div>
              </div>
              <span className="muted small">{timeAgo(it.updatedAt)}</span>
              <button className="small" onClick={() => open(it, it)}>
                Open
              </button>
              <button className="small ghost" onClick={() => rename(it)}>
                Rename
              </button>
              <button className="small ghost danger-text" onClick={() => remove(it)}>
                Delete
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <ul className="library-list">
          {history.length === 0 && !loading && <li className="empty-state">No history yet.</li>}
          {history.map((h) => (
            <li key={h.id} className={h.ok ? 'library-item' : 'library-item failed'}>
              <div className="library-main" onDoubleClick={() => open(h)}>
                <div>
                  <span className={`kind-badge kind-${h.kind}`}>{KIND_LABELS[h.kind]}</span>
                  <code className="history-summary">{h.summary}</code>
                </div>
                <div className="muted small">
                  {[h.connName, h.db && (h.coll ? `${h.db}.${h.coll}` : h.db), formatDuration(h.durationMs), timeAgo(h.at)]
                    .filter(Boolean)
                    .join(' · ')}
                  {!h.ok && <span className="error-text"> · {h.error}</span>}
                </div>
              </div>
              <button className="small" onClick={() => open(h)}>
                Open
              </button>
              <button className="small ghost" onClick={() => saveFromHistory(h)}>
                Save
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
