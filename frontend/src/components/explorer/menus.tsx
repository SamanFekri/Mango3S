import { backend } from '../../lib/backend'
import type { CollectionInfo, SessionInfo } from '../../lib/types'
import type { useApp } from '../../state/AppContext'

export interface MenuItem {
  label: string
  action: () => void
  danger?: boolean
  disabled?: boolean
  /** Shortcut id whose binding is shown next to the item (same command). */
  shortcut?: string
}

/** The dialog the explorer has open; one at a time. */
export type ExplorerDialog =
  | { kind: 'create'; conn: SessionInfo; db?: string }
  | { kind: 'import'; conn: SessionInfo; db: string; coll?: string }
  | { kind: 'export'; conn: SessionInfo; db: string; coll: string }
  | { kind: 'backup' | 'restore'; conn: SessionInfo; db?: string; coll?: string }
  | {
      kind: 'info' | 'copy' | 'rename' | 'profiler' | 'sql' | 'advisor' | 'datagen'
      conn: SessionInfo
      db: string
      coll?: string
      tab?: 'overview' | 'indexes' | 'validation'
    }

type App = ReturnType<typeof useApp>

/** What the explorer's menus act on. */
export interface MenuContext {
  app: Pick<App, 'dispatchTabs' | 'openConnectionManager' | 'disconnect' | 'confirm' | 'toast' | 'toastError'>
  loadDbs: (connId: string) => void
  loadColls: (connId: string, db: string) => void
  openDialog: (d: ExplorerDialog) => void
}

/** The connection explorer's context menus and its destructive actions (drop,
 * clear), which ask for confirmation first. */
export function explorerActions({ app, loadDbs, loadColls, openDialog }: MenuContext) {
  const { dispatchTabs, openConnectionManager, disconnect, confirm, toast, toastError } = app

  const openCollection = (s: SessionInfo, db: string, coll: string) =>
    dispatchTabs({ type: 'open', tab: { kind: 'collection', connId: s.id, connName: s.name, db, coll }, reuse: true })

  const dropDb = async (s: SessionInfo, db: string) => {
    const ok = await confirm({
      title: 'Drop database',
      message: (
        <>
          Permanently drop database <code>{db}</code> on <strong>{s.name}</strong> and all of its collections? This cannot be undone.
        </>
      ),
      confirmLabel: 'Drop database',
      danger: true,
      typeToConfirm: db,
    })
    if (!ok) return
    try {
      await backend.dropDatabase(s.id, db)
      toast({ kind: 'success', title: `Dropped database ${db}` })
      loadDbs(s.id)
    } catch (err) {
      toastError(err, 'Drop failed')
    }
  }

  const clearColl = async (s: SessionInfo, db: string, coll: string) => {
    const ns = `${db}.${coll}`
    const ok = await confirm({
      title: 'Clear collection',
      message: (
        <>
          Delete every document in <code>{ns}</code> on <strong>{s.name}</strong>? The collection, its indexes and validation rules are kept. This cannot be undone; back the
          collection up first if you may need the data.
        </>
      ),
      confirmLabel: 'Delete all documents',
      danger: true,
      typeToConfirm: ns,
    })
    if (!ok) return
    try {
      const n = await backend.clearCollection(s.id, db, coll, ns)
      toast({ kind: 'success', title: `Deleted ${n} document${n === 1 ? '' : 's'} from ${ns}` })
    } catch (err) {
      toastError(err, 'Clear failed')
    }
  }

  const dropColl = async (s: SessionInfo, db: string, coll: string) => {
    const ok = await confirm({
      title: 'Drop collection',
      message: (
        <>
          Permanently drop <code>
            {db}.{coll}
          </code>{' '}
          on <strong>{s.name}</strong>, including all documents and indexes?
        </>
      ),
      confirmLabel: 'Drop collection',
      danger: true,
      typeToConfirm: coll,
    })
    if (!ok) return
    try {
      await backend.dropCollection(s.id, db, coll)
      toast({ kind: 'success', title: `Dropped ${db}.${coll}` })
      loadColls(s.id, db)
    } catch (err) {
      toastError(err, 'Drop failed')
    }
  }

  const connMenu = (s: SessionInfo): MenuItem[] => [
    { label: 'Open workspace', action: () => dispatchTabs({ type: 'open', tab: { kind: 'connection', connId: s.id, connName: s.name } }) },
    { label: 'Refresh', action: () => loadDbs(s.id), shortcut: 'view.refresh' },
    { label: 'Open shell', action: () => dispatchTabs({ type: 'open', tab: { kind: 'shell', connId: s.id, connName: s.name } }), shortcut: 'tab.shell' },
    { label: 'Server monitor', action: () => dispatchTabs({ type: 'open', tab: { kind: 'monitor', connId: s.id, connName: s.name } }) },
    { label: 'Watch changes', action: () => dispatchTabs({ type: 'open', tab: { kind: 'changes', connId: s.id, connName: s.name }, reuse: true }) },
    { label: 'Users and roles…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'users', connId: s.id, connName: s.name } }) },
    { label: 'SQL query', action: () => dispatchTabs({ type: 'open', tab: { kind: 'sql', connId: s.id, connName: s.name, db: 'test' } }), shortcut: 'tab.sql' },
    { label: 'Create database…', action: () => openDialog({ kind: 'create', conn: s }), disabled: s.readOnly },
    { label: 'Migrate from SQL…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'migrate', connId: s.id, connName: s.name, db: '' } }), disabled: s.readOnly },
    { label: 'Compare / sync…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'compare', connId: s.id, connName: s.name } }) },
    { label: 'Back up…', action: () => openDialog({ kind: 'backup', conn: s }) },
    { label: 'Restore…', action: () => openDialog({ kind: 'restore', conn: s }), disabled: s.readOnly },
    { label: 'Edit connection…', action: () => openConnectionManager(true, s.id) },
    {
      label: 'Copy connection string',
      action: async () => {
        try {
          await navigator.clipboard.writeText(await backend.connectionURI(s.id, false))
          toast({ kind: 'success', title: 'Connection string copied', message: 'The password is not included.' })
        } catch (err) {
          toastError(err, 'Copy failed')
        }
      },
    },
    { label: 'Disconnect', action: () => disconnect(s.id), shortcut: 'conn.disconnect' },
  ]

  const dbMenu = (s: SessionInfo, db: string): MenuItem[] => [
    { label: 'Refresh', action: () => loadColls(s.id, db), shortcut: 'view.refresh' },
    { label: 'Create collection or view…', action: () => openDialog({ kind: 'create', conn: s, db }), disabled: s.readOnly },
    { label: 'Database statistics…', action: () => openDialog({ kind: 'info', conn: s, db }) },
    { label: 'Query profiler…', action: () => openDialog({ kind: 'profiler', conn: s, db }) },
    { label: 'Index advisor…', action: () => openDialog({ kind: 'advisor', conn: s, db }) },
    { label: 'Watch changes', action: () => dispatchTabs({ type: 'open', tab: { kind: 'changes', connId: s.id, connName: s.name, db }, reuse: true }) },
    { label: 'GridFS buckets…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'gridfs', connId: s.id, connName: s.name, db }, reuse: true }) },
    { label: 'Copy database…', action: () => openDialog({ kind: 'copy', conn: s, db }) },
    { label: 'Rename database…', action: () => openDialog({ kind: 'rename', conn: s, db }), disabled: s.readOnly },
    { label: 'Open shell here', action: () => dispatchTabs({ type: 'open', tab: { kind: 'shell', connId: s.id, connName: s.name, db } }), shortcut: 'tab.shell' },
    { label: 'SQL query', action: () => dispatchTabs({ type: 'open', tab: { kind: 'sql', connId: s.id, connName: s.name, db } }), shortcut: 'tab.sql' },
    { label: 'Import into new collection…', action: () => openDialog({ kind: 'import', conn: s, db }), disabled: s.readOnly },
    { label: 'Back up database…', action: () => openDialog({ kind: 'backup', conn: s, db }) },
    { label: 'Restore into database…', action: () => openDialog({ kind: 'restore', conn: s, db }), disabled: s.readOnly },
    { label: 'Compare database…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'compare', connId: s.id, connName: s.name, db } }) },
    { label: 'Migrate from SQL into…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'migrate', connId: s.id, connName: s.name, db } }), disabled: s.readOnly },
    { label: 'Drop database…', action: () => dropDb(s, db), danger: true, disabled: s.readOnly, shortcut: 'explorer.drop' },
  ]

  const collMenu = (s: SessionInfo, db: string, coll: CollectionInfo): MenuItem[] => {
    const t = { connId: s.id, connName: s.name, db, coll: coll.name }
    const c = { conn: s, db, coll: coll.name }
    const view = coll.type === 'view'
    return [
      { label: 'Open', action: () => openCollection(s, db, coll.name), shortcut: 'explorer.open' },
      { label: 'Open in new tab', action: () => dispatchTabs({ type: 'open', tab: { kind: 'collection', ...t } }) },
      { label: 'Aggregation editor', action: () => dispatchTabs({ type: 'open', tab: { kind: 'aggregation', ...t } }) },
      { label: 'Schema', action: () => dispatchTabs({ type: 'open', tab: { kind: 'schema', ...t }, reuse: true }) },
      { label: 'Watch changes', action: () => dispatchTabs({ type: 'open', tab: { kind: 'changes', ...t }, reuse: true }) },
      { label: 'Export…', action: () => openDialog({ kind: 'export', ...c }) },
      { label: 'Export to SQL…', action: () => openDialog({ kind: 'sql', ...c }) },
      { label: 'Back up collection…', action: () => openDialog({ kind: 'backup', ...c }) },
      { label: 'Compare / sync…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'compare', ...t } }) },
      { label: 'Reschema…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'reschema', ...t } }), disabled: view },
      { label: 'Mask data…', action: () => dispatchTabs({ type: 'open', tab: { kind: 'mask', ...t } }), disabled: view },
      { label: 'Import…', action: () => openDialog({ kind: 'import', ...c }), disabled: s.readOnly || view },
      { label: 'Generate test data…', action: () => openDialog({ kind: 'datagen', ...c }), disabled: s.readOnly || view },
      { label: 'Collection info…', action: () => openDialog({ kind: 'info', ...c }) },
      { label: 'Indexes…', action: () => openDialog({ kind: 'info', ...c, tab: 'indexes' }), disabled: view },
      { label: 'Index advisor…', action: () => openDialog({ kind: 'advisor', ...c }), disabled: view },
      { label: 'Validation rules…', action: () => openDialog({ kind: 'info', ...c, tab: 'validation' }), disabled: view },
      { label: 'Copy collection…', action: () => openDialog({ kind: 'copy', ...c }), disabled: view },
      { label: 'Rename…', action: () => openDialog({ kind: 'rename', ...c }), disabled: s.readOnly || view },
      { label: 'Clear (delete all documents)…', action: () => clearColl(s, db, coll.name), danger: true, disabled: s.readOnly || view },
      { label: 'Drop collection…', action: () => dropColl(s, db, coll.name), danger: true, disabled: s.readOnly, shortcut: 'explorer.drop' },
    ]
  }

  return { openCollection, dropDb, dropColl, connMenu, dbMenu, collMenu }
}
