import { AppBuddy } from './AppBuddy'
import { useCallback, useEffect, useRef, useState } from 'react'
import type { KeyboardEvent as ReactKeyboardEvent, MouseEvent as ReactMouseEvent } from 'react'
import { backend } from '../lib/backend'
import { formatBytes } from '../lib/format'
import type { CollectionInfo, DatabaseInfo, SessionInfo } from '../lib/types'
import { useApp } from '../state/AppContext'
import { SHORTCUT_BY_ID, formatBinding, keysFor, shortcutHint } from '../lib/shortcuts'
import { moveId } from '../lib/tabs'
import { getShortcutPlatform, useShortcut } from '../state/shortcuts'
import { Spinner } from './common'
import { CollectionIcon, DatabaseIcon, FolderIcon, ServerIcon } from './icons'
import { ExplorerDialogs } from './explorer/ExplorerDialogs'
import { explorerActions, type ExplorerDialog, type MenuItem } from './explorer/menus'

interface MenuState {
  x: number
  y: number
  items: MenuItem[]
}

type Loadable<T> = { loading: boolean; error?: string; data?: T }

type GroupKind = 'collections' | 'views' | 'system' | 'gridfs'
/** Folders under each database. */
const GROUPS: { kind: GroupKind; label: string; match: (c: CollectionInfo) => boolean }[] = [
  { kind: 'collections', label: 'Collections', match: (c) => !c.system && c.type !== 'view' },
  { kind: 'views', label: 'Views', match: (c) => !c.system && c.type === 'view' },
  { kind: 'system', label: 'System', match: (c) => c.system },
]

/** GridFS buckets: "<name>.files" collections with a matching "<name>.chunks". */
export function gridfsBuckets(colls: CollectionInfo[]): string[] {
  const names = new Set(colls.map((c) => c.name))
  return colls
    .map((c) => c.name)
    .filter((n) => n.endsWith('.files') && n.length > 6 && names.has(`${n.slice(0, -6)}.chunks`))
    .map((n) => n.slice(0, -6))
    .sort()
}

export function Sidebar() {
  const app = useApp()
  const { sessions, profiles, reorderProfiles, connect, connecting, disconnect, dispatchTabs, openConnectionManager } = app
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [dbs, setDbs] = useState<Record<string, Loadable<DatabaseInfo[]>>>({})
  const [colls, setColls] = useState<Record<string, Loadable<CollectionInfo[]>>>({})
  const [menu, setMenu] = useState<MenuState | null>(null)
  const [filter, setFilter] = useState('')
  const [dialog, setDialog] = useState<ExplorerDialog | null>(null)
  const [closedGroups, setClosedGroups] = useState<Set<string>>(new Set())
  const [focused, setFocused] = useState<string | null>(null)
  const navRef = useRef<HTMLElement | null>(null)
  const treeRef = useRef<HTMLDivElement | null>(null)
  const filterRef = useRef<HTMLInputElement | null>(null)

  const loadDbs = useCallback(async (connId: string) => {
    setDbs((d) => ({ ...d, [connId]: { ...d[connId], loading: true, error: undefined } }))
    try {
      const data = await backend.listDatabases(connId)
      setDbs((d) => ({ ...d, [connId]: { loading: false, data } }))
    } catch (err) {
      const e = err as { message?: string }
      setDbs((d) => ({ ...d, [connId]: { loading: false, error: e.message ?? String(err) } }))
    }
  }, [])

  const loadColls = useCallback(async (connId: string, db: string) => {
    const k = `${connId}/${db}`
    setColls((c) => ({ ...c, [k]: { ...c[k], loading: true, error: undefined } }))
    try {
      const data = await backend.listCollections(connId, db)
      setColls((c) => ({ ...c, [k]: { loading: false, data } }))
    } catch (err) {
      const e = err as { message?: string }
      setColls((c) => ({ ...c, [k]: { loading: false, error: e.message ?? String(err) } }))
    }
  }, [])

  // Auto-expand newly opened connections; forget cached trees of closed ones
  // so a reconnect reloads fresh data.
  useEffect(() => {
    const live = new Set(sessions.map((s) => s.id))
    const stale = Object.keys(dbs).filter((id) => !live.has(id))
    if (stale.length > 0) {
      setDbs((d) => Object.fromEntries(Object.entries(d).filter(([id]) => live.has(id))))
      setColls((c) => Object.fromEntries(Object.entries(c).filter(([k]) => live.has(k.split('/')[0]))))
      return
    }
    for (const s of sessions) {
      if (!dbs[s.id]) {
        setExpanded((e) => new Set(e).add(s.id))
        loadDbs(s.id)
      }
    }
  }, [sessions, dbs, loadDbs])

  useEffect(() => {
    if (!menu) return
    const close = () => setMenu(null)
    window.addEventListener('click', close)
    window.addEventListener('blur', close)
    return () => {
      window.removeEventListener('click', close)
      window.removeEventListener('blur', close)
    }
  }, [menu])

  const toggle = (key: string, onOpen?: () => void) => {
    setExpanded((e) => {
      const n = new Set(e)
      if (n.has(key)) n.delete(key)
      else {
        n.add(key)
        onOpen?.()
      }
      return n
    })
  }

  const openMenu = (e: ReactMouseEvent, items: MenuItem[]) => {
    e.preventDefault()
    e.stopPropagation()
    setMenu({ x: e.clientX, y: e.clientY, items })
  }

  const { openCollection, dropDb, dropColl, connMenu, dbMenu, collMenu } = explorerActions({ app, loadDbs, loadColls, openDialog: setDialog })

  // Connections follow the user's order (drag to reorder, or Alt+↑/↓).
  const profileIds = profiles.map((p) => p.id)
  const orderOf = (id: string) => {
    const i = profileIds.indexOf(id)
    return i < 0 ? Number.MAX_SAFE_INTEGER : i
  }
  const orderedSessions = [...sessions].sort((a, b) => orderOf(a.id) - orderOf(b.id))
  // Alt+1–9 connects to the Nth connection in this order.
  const quickKey = (id: string) => {
    const platform = getShortcutPlatform()
    const k = keysFor(SHORTCUT_BY_ID['conn.quick'], platform)[orderOf(id)]
    return k ? ` (${formatBinding(k, platform)})` : ''
  }
  const disconnected = profiles.filter((p) => !sessions.some((s) => s.id === p.id))
  const [drag, setDrag] = useState<{ id: string; over?: string; before?: boolean } | null>(null)
  const dragProps = (id: string) => ({
    draggable: true,
    onDragStart: (e: React.DragEvent) => {
      e.dataTransfer.effectAllowed = 'move'
      e.dataTransfer.setData('text/mango3s-connection', id)
      setDrag({ id })
    },
    onDragOver: (e: React.DragEvent) => {
      if (!drag || drag.id === id) return
      e.preventDefault()
      e.dataTransfer.dropEffect = 'move'
      const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
      const before = e.clientY < r.top + r.height / 2
      if (drag.over !== id || drag.before !== before) setDrag({ ...drag, over: id, before })
    },
    onDrop: (e: React.DragEvent) => {
      e.preventDefault()
      if (drag && drag.id !== id) void reorderProfiles(moveId(profileIds, drag.id, id, drag.over === id ? !!drag.before : true))
      setDrag(null)
    },
    onDragEnd: () => setDrag(null),
  })
  const dropClass = (id: string) => (drag?.over === id ? (drag.before ? ' drop-before' : ' drop-after') : '') + (drag?.id === id ? ' dragging' : '')
  /** Alt+↑/↓: move a connection one place in the user's order. */
  const nudge = (id: string, delta: number) => {
    const i = profileIds.indexOf(id)
    const j = i + delta
    if (i < 0 || j < 0 || j >= profileIds.length) return
    void reorderProfiles(moveId(profileIds, id, profileIds[j], delta < 0))
  }
  const f = filter.trim().toLowerCase()

  // ---- the visible tree as rows (also drives keyboard navigation) -------------
  type Row =
    | { key: string; level: number; kind: 'conn'; s: SessionInfo }
    | { key: string; level: number; kind: 'db'; s: SessionInfo; db: DatabaseInfo }
    | { key: string; level: number; kind: 'group'; s: SessionInfo; db: string; group: GroupKind; count: number }
    | { key: string; level: number; kind: 'coll'; s: SessionInfo; db: string; coll: CollectionInfo }
    | { key: string; level: number; kind: 'bucket'; s: SessionInfo; db: string; bucket: string }
    | { key: string; level: number; kind: 'note'; text: string; error?: boolean; loading?: boolean }
  const rows: Row[] = []
  for (const s of orderedSessions) {
    const ck = s.id
    rows.push({ key: `c:${ck}`, level: 0, kind: 'conn', s })
    if (!expanded.has(ck)) continue
    const d = dbs[s.id]
    if (d?.error) rows.push({ key: `e:${ck}`, level: 1, kind: 'note', text: d.error, error: true })
    for (const db of d?.data ?? []) {
      const dk = `${s.id}/${db.name}`
      const c = colls[dk]
      const visible = f ? c?.data?.filter((x) => x.name.toLowerCase().includes(f)) : c?.data
      if (f && c?.data && !visible?.length && !db.name.toLowerCase().includes(f)) continue
      rows.push({ key: `d:${dk}`, level: 1, kind: 'db', s, db })
      if (!expanded.has(dk) && !(f && visible?.length)) continue
      if (c?.error) rows.push({ key: `e:${dk}`, level: 2, kind: 'note', text: c.error, error: true })
      if (c?.loading && !c.data) rows.push({ key: `l:${dk}`, level: 2, kind: 'note', text: 'Loading…', loading: true })
      if (c?.data && c.data.length === 0) rows.push({ key: `n:${dk}`, level: 2, kind: 'note', text: 'No collections' })
      for (const g of GROUPS) {
        const list = (visible ?? []).filter(g.match)
        if (!list.length) continue
        const gk = `${dk}/${g.kind}`
        rows.push({ key: `g:${gk}`, level: 2, kind: 'group', s, db: db.name, group: g.kind, count: list.length })
        if (!isGroupOpen(gk, g.kind) && !f) continue
        for (const coll of list) rows.push({ key: `k:${dk}/${coll.name}`, level: 3, kind: 'coll', s, db: db.name, coll })
      }
      const buckets = gridfsBuckets(c?.data ?? []).filter((b) => !f || b.toLowerCase().includes(f) || db.name.toLowerCase().includes(f))
      if (buckets.length) {
        const gk = `${dk}/gridfs`
        rows.push({ key: `g:${gk}`, level: 2, kind: 'group', s, db: db.name, group: 'gridfs', count: buckets.length })
        if (isGroupOpen(gk, 'gridfs') || f) for (const b of buckets) rows.push({ key: `b:${dk}/${b}`, level: 3, kind: 'bucket', s, db: db.name, bucket: b })
      }
    }
  }

  function isGroupOpen(gk: string, kind: GroupKind) {
    // Collections start open; Views and System start closed.
    return kind === 'collections' ? !closedGroups.has(gk) : expanded.has(gk)
  }
  const toggleGroup = (gk: string, kind: GroupKind, open?: boolean) => {
    const isOpen = isGroupOpen(gk, kind)
    const want = open ?? !isOpen
    if (want === isOpen) return
    if (kind === 'collections') {
      setClosedGroups((c) => {
        const n = new Set(c)
        if (want) n.delete(gk)
        else n.add(gk)
        return n
      })
    } else {
      setExpanded((e) => {
        const n = new Set(e)
        if (want) n.add(gk)
        else n.delete(gk)
        return n
      })
    }
  }
  const setOpenKey = (key: string, open: boolean, onOpen?: () => void) =>
    setExpanded((e) => {
      if (e.has(key) === open) return e
      const n = new Set(e)
      if (open) {
        n.add(key)
        onOpen?.()
      } else n.delete(key)
      return n
    })

  const focusedRow = rows.find((r) => r.key === focused)
  const activate = (r: Row) => {
    setFocused(r.key)
    switch (r.kind) {
      case 'conn':
        toggle(r.s.id, () => !dbs[r.s.id]?.data && loadDbs(r.s.id))
        break
      case 'db':
        toggle(`${r.s.id}/${r.db.name}`, () => !colls[`${r.s.id}/${r.db.name}`]?.data && loadColls(r.s.id, r.db.name))
        break
      case 'group':
        toggleGroup(`${r.s.id}/${r.db}/${r.group}`, r.group)
        break
      case 'coll':
        openCollection(r.s, r.db, r.coll.name)
        break
      case 'bucket':
        dispatchTabs({ type: 'open', tab: { kind: 'gridfs', connId: r.s.id, connName: r.s.name, db: r.db, coll: r.bucket }, reuse: true })
        break
    }
  }
  const expandRow = (r: Row, open: boolean) => {
    if (r.kind === 'conn') setOpenKey(r.s.id, open, () => !dbs[r.s.id]?.data && loadDbs(r.s.id))
    if (r.kind === 'db') setOpenKey(`${r.s.id}/${r.db.name}`, open, () => !colls[`${r.s.id}/${r.db.name}`]?.data && loadColls(r.s.id, r.db.name))
    if (r.kind === 'group') toggleGroup(`${r.s.id}/${r.db}/${r.group}`, r.group, open)
  }
  const isOpenRow = (r: Row) =>
    r.kind === 'conn' ? expanded.has(r.s.id) : r.kind === 'db' ? expanded.has(`${r.s.id}/${r.db.name}`) : r.kind === 'group' ? isGroupOpen(`${r.s.id}/${r.db}/${r.group}`, r.group) : false
  const nav = rows.filter((r) => r.kind !== 'note')

  const onTreeKey = (e: ReactKeyboardEvent) => {
    if (e.target !== treeRef.current) return // typing in the filter etc.
    const i = nav.findIndex((r) => r.key === focused)
    const cur = nav[i]
    if (e.altKey && (e.key === 'ArrowUp' || e.key === 'ArrowDown') && cur?.kind === 'conn') {
      e.preventDefault()
      nudge(cur.s.id, e.key === 'ArrowUp' ? -1 : 1)
      return
    }
    const go = (r: Row | undefined) => {
      if (!r) return
      setFocused(r.key)
      treeRef.current?.querySelector(`[data-key="${CSS.escape(r.key)}"]`)?.scrollIntoView?.({ block: 'nearest' })
    }
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        go(i < 0 ? nav[0] : nav[i + 1])
        break
      case 'ArrowUp':
        e.preventDefault()
        go(i < 0 ? nav[nav.length - 1] : nav[i - 1])
        break
      case 'Home':
        e.preventDefault()
        go(nav[0])
        break
      case 'End':
        e.preventDefault()
        go(nav[nav.length - 1])
        break
      case 'ArrowRight':
        if (!cur) return
        e.preventDefault()
        if (cur.kind !== 'coll' && !isOpenRow(cur)) expandRow(cur, true)
        else if (nav[i + 1]?.level > cur.level) go(nav[i + 1])
        break
      case 'ArrowLeft':
        if (!cur) return
        e.preventDefault()
        if (cur.kind !== 'coll' && isOpenRow(cur)) expandRow(cur, false)
        else go([...nav.slice(0, i)].reverse().find((r) => r.level < cur.level))
        break
      case 'Enter':
        if (!cur) return
        e.preventDefault()
        activate(cur)
        break
      case 'Delete': // Del (macOS: Fn+Delete)
        if (!cur || e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) return
        e.preventDefault()
        if (cur.kind === 'coll' && !cur.s.readOnly) void dropColl(cur.s, cur.db, cur.coll.name)
        else if (cur.kind === 'db' && !cur.s.readOnly) void dropDb(cur.s, cur.db.name)
        break
    }
  }

  useShortcut(
    'view.refresh',
    () => {
      const r = focusedRow
      if (!r || r.kind === 'note') return false
      if (r.kind === 'conn') loadDbs(r.s.id)
      else loadColls(r.s.id, r.kind === 'db' ? r.db.name : r.db)
    },
    { within: navRef },
  )
  useShortcut(
    'explorer.refreshAll',
    () => {
      for (const s of sessions) loadDbs(s.id)
      for (const k of Object.keys(colls)) {
        const [id, ...rest] = k.split('/')
        loadColls(id, rest.join('/'))
      }
    },
    { within: navRef },
  )
  // With focus in the tree these act on the selected item, like its menu.
  const focusedConn = focusedRow && focusedRow.kind !== 'note' ? focusedRow.s : undefined
  const focusedDb = focusedRow?.kind === 'db' ? focusedRow.db.name : focusedRow?.kind === 'coll' || focusedRow?.kind === 'group' ? focusedRow.db : undefined
  useShortcut('conn.disconnect', () => (focusedConn ? void disconnect(focusedConn.id) : false), { within: navRef })
  useShortcut(
    'tab.shell',
    () => (focusedConn ? dispatchTabs({ type: 'open', tab: { kind: 'shell', connId: focusedConn.id, connName: focusedConn.name, db: focusedDb } }) : false),
    { within: navRef },
  )
  useShortcut(
    'tab.sql',
    () => (focusedConn ? dispatchTabs({ type: 'open', tab: { kind: 'sql', connId: focusedConn.id, connName: focusedConn.name, db: focusedDb ?? 'test' } }) : false),
    { within: navRef },
  )
  useShortcut('explorer.filter', () => (sessions.length ? filterRef.current?.focus() : false))
  useShortcut('focus.explorer', () => {
    if (!focused && nav[0]) setFocused(nav[0].key)
    treeRef.current?.focus()
  })

  const rowProps = (r: Row) => ({
    'data-key': r.key,
    id: `tree-${r.key}`,
    role: 'treeitem',
    'aria-level': r.level + 1,
    'aria-selected': focused === r.key,
    'aria-expanded': r.kind === 'coll' || r.kind === 'note' ? undefined : isOpenRow(r),
  })

  return (
    <nav className="sidebar" aria-label="Database explorer" ref={navRef}>
      <div className="sidebar-header">
        <span className="sidebar-title">Explorer</span>
        <button className="small" onClick={() => openConnectionManager(true)} title={`Connection Manager (${shortcutHint('conn.manager')})`}>
          Connect…
        </button>
      </div>
      {sessions.length > 0 && (
        <div className="sidebar-filter">
          <input ref={filterRef} placeholder="Filter collections" value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Filter collections" />
        </div>
      )}

      <div
        className="tree"
        role="tree"
        tabIndex={0}
        ref={treeRef}
        aria-activedescendant={focused ? `tree-${focused}` : undefined}
        aria-label="Connections, databases and collections"
        onKeyDown={onTreeKey}
        onFocus={() => !focused && nav[0] && setFocused(nav[0].key)}
      >
        {rows.map((r) => {
          const cls = `tree-row level-${r.level}${focused === r.key ? ' focused' : ''}`
          if (r.kind === 'note') {
            return (
              <div key={r.key} className={`${r.error ? 'tree-error' : 'tree-empty'} level-${r.level}`}>
                {r.loading && <Spinner small />} {r.text}
              </div>
            )
          }
          if (r.kind === 'conn') {
            const s = r.s
            const open = expanded.has(s.id)
            return (
              <div
                key={r.key}
                {...rowProps(r)}
                {...dragProps(s.id)}
                className={cls + dropClass(s.id)}
                onClick={() => activate(r)}
                onDoubleClick={() => dispatchTabs({ type: 'open', tab: { kind: 'connection', connId: s.id, connName: s.name } })}
                onContextMenu={(e) => openMenu(e, connMenu(s))}
                title={`MongoDB ${s.serverVersion} · ${s.topology}${quickKey(s.id)} — double-click for the workspace`}
              >
                <span className={open ? 'caret open' : 'caret'} />
                <ServerIcon live />
                <span className="tree-label strong">{s.name}</span>
                {s.readOnly && <span className="badge">RO</span>}
                {dbs[s.id]?.loading && <Spinner small />}
                <button className="row-action" title="More actions" onClick={(e) => openMenu(e, connMenu(s))}>
                  ⋯
                </button>
              </div>
            )
          }
          if (r.kind === 'db') {
            const k = `${r.s.id}/${r.db.name}`
            return (
              <div key={r.key} {...rowProps(r)} className={cls} onClick={() => activate(r)} onContextMenu={(e) => openMenu(e, dbMenu(r.s, r.db.name))}>
                <span className={expanded.has(k) ? 'caret open' : 'caret'} />
                <DatabaseIcon />
                <span className="tree-label">{r.db.name}</span>
                <span className="tree-meta">{formatBytes(r.db.sizeOnDisk)}</span>
                {colls[k]?.loading && colls[k]?.data && <Spinner small />}
                <button className="row-action" title="More actions" onClick={(e) => openMenu(e, dbMenu(r.s, r.db.name))}>
                  ⋯
                </button>
              </div>
            )
          }
          if (r.kind === 'group') {
            const open = isGroupOpen(`${r.s.id}/${r.db}/${r.group}`, r.group) || !!f
            const label = r.group === 'gridfs' ? 'GridFS buckets' : GROUPS.find((x) => x.kind === r.group)!.label
            return (
              <div key={r.key} {...rowProps(r)} className={`${cls} tree-group`} onClick={() => activate(r)}>
                <span className={open ? 'caret open' : 'caret'} />
                <FolderIcon open={open} />
                <span className="tree-label">{label}</span>
                <span className="tree-meta">{r.count}</span>
              </div>
            )
          }
          if (r.kind === 'bucket') {
            return (
              <div key={r.key} {...rowProps(r)} className={cls} onClick={() => activate(r)} title={`${r.db} · GridFS bucket ${r.bucket}`}>
                <FolderIcon open={false} />
                <span className="tree-label">{r.bucket}</span>
                <span className="tree-meta">GridFS</span>
              </div>
            )
          }
          const c = r.coll
          return (
            <div
              key={r.key}
              {...rowProps(r)}
              className={cls}
              onClick={() => activate(r)}
              onContextMenu={(e) => openMenu(e, collMenu(r.s, r.db, c))}
              title={`${r.db}.${c.name}${c.type !== 'collection' ? ` (${c.type})` : ''}`}
            >
              <CollectionIcon kind={c.system ? 'system' : c.type === 'view' ? 'view' : c.type === 'timeseries' ? 'timeseries' : 'collection'} />
              <span className="tree-label">{c.name}</span>
              {c.type === 'timeseries' && <span className="tree-meta">time series</span>}
              <button className="row-action" title="More actions" onClick={(e) => openMenu(e, collMenu(r.s, r.db, c))}>
                ⋯
              </button>
            </div>
          )
        })}

        {disconnected.length > 0 && (
          <div className="saved-list">
            <div className="section-label">Saved connections</div>
            {disconnected.map((p) => (
              <div
                key={p.id}
                className={'tree-row level-0 saved' + dropClass(p.id)}
                onDoubleClick={() => connect(p.id)}
                title={`${p.folder ? `${p.folder} / ${p.name}` : p.name}${quickKey(p.id)} — drag to reorder`}
                {...dragProps(p.id)}
              >
                <ServerIcon />
                <span className="tree-label">
                  {p.folder && <span className="muted">{p.folder} / </span>}
                  {p.name}
                </span>
                {connecting.has(p.id) ? (
                  <Spinner small />
                ) : (
                  <button className="small ghost" onClick={() => connect(p.id)}>
                    Connect
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
        {profiles.length === 0 && (
          <div className="sidebar-empty">
            <p>No connections yet.</p>
            <button className="primary" onClick={() => openConnectionManager(true, 'new')}>
              New connection
            </button>
          </div>
        )}
      </div>

      <AppBuddy />

      {menu && (
        <ul className="context-menu" style={{ left: menu.x, top: menu.y }} role="menu">
          {menu.items.map((it) => (
            <li key={it.label}>
              <button
                role="menuitem"
                className={it.danger ? 'danger-text' : ''}
                disabled={it.disabled}
                onClick={() => {
                  setMenu(null)
                  it.action()
                }}
              >
                <span>{it.label}</span>
                {it.shortcut && shortcutHint(it.shortcut) && <kbd className="menu-kbd">{shortcutHint(it.shortcut)}</kbd>}
              </button>
            </li>
          ))}
        </ul>
      )}

      <ExplorerDialogs
        dialog={dialog}
        onClose={() => setDialog(null)}
        loadDbs={loadDbs}
        loadColls={loadColls}
        expand={(key) => setExpanded((e) => new Set(e).add(key))}
      />
    </nav>
  )
}
