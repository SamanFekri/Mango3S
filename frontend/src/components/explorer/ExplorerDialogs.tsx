import { useApp } from '../../state/AppContext'
import { BackupDialog, RestoreDialog } from '../BackupDialogs'
import { ExportWizard } from '../ExportWizard'
import { ImportWizard } from '../ImportWizard'
import { CollectionInfoDialog, CopyDialog, CreateCollectionDialog, DatabaseInfoDialog, RenameCollectionDialog, RenameDatabaseDialog } from '../ManageDialogs'
import { TestDataDialog } from '../TestDataDialog'
import { IndexAdvisorDialog, ProfilerDialog, SqlExportDialog } from '../WorkflowDialogs'
import type { ExplorerDialog } from './menus'

/** The dialog opened from the connection explorer's menus. */
export function ExplorerDialogs({
  dialog,
  onClose,
  loadDbs,
  loadColls,
  expand,
}: {
  dialog: ExplorerDialog | null
  onClose: () => void
  loadDbs: (connId: string) => Promise<void> | void
  loadColls: (connId: string, db: string) => void
  /** Opens a tree node, e.g. "<connId>/<db>" after creating a collection. */
  expand: (key: string) => void
}) {
  const { toast } = useApp()
  if (!dialog) return null
  const d = dialog
  switch (d.kind) {
    case 'backup':
      return <BackupDialog conn={d.conn} db={d.db} coll={d.coll} onClose={onClose} />
    case 'restore':
      return <RestoreDialog conn={d.conn} db={d.db} onClose={onClose} />
    case 'import':
      return (
        <ImportWizard
          connId={d.conn.id}
          db={d.db}
          coll={d.coll}
          onClose={() => {
            onClose()
            loadColls(d.conn.id, d.db)
          }}
        />
      )
    case 'export':
      return (
        <ExportWizard
          query={{ connId: d.conn.id, db: d.db, coll: d.coll, filter: '{}', projection: '', sort: '', skip: 0, limit: 0, page: 0, pageSize: 50 }}
          onClose={onClose}
        />
      )
    case 'create':
      return (
        <CreateCollectionDialog
          conn={d.conn}
          db={d.db}
          onClose={onClose}
          onCreated={async (db, coll) => {
            toast({ kind: 'success', title: `Created ${db}.${coll}` })
            onClose()
            await loadDbs(d.conn.id)
            expand(`${d.conn.id}/${db}`)
            loadColls(d.conn.id, db)
          }}
        />
      )
    case 'info':
      return d.coll ? (
        <CollectionInfoDialog conn={d.conn} db={d.db} coll={d.coll} tab={d.tab} onClose={onClose} />
      ) : (
        <DatabaseInfoDialog conn={d.conn} db={d.db} onClose={onClose} />
      )
    case 'advisor':
      return <IndexAdvisorDialog conn={d.conn} db={d.db} coll={d.coll} onClose={onClose} />
    case 'profiler':
      return <ProfilerDialog conn={d.conn} db={d.db} onClose={onClose} />
    case 'sql':
      return d.coll ? <SqlExportDialog conn={d.conn} db={d.db} coll={d.coll} onClose={onClose} /> : null
    case 'datagen':
      return d.coll ? <TestDataDialog conn={d.conn} db={d.db} coll={d.coll} onClose={onClose} /> : null
    case 'copy':
      return <CopyDialog conn={d.conn} db={d.db} coll={d.coll} onClose={onClose} />
    case 'rename':
      return d.coll ? (
        <RenameCollectionDialog
          conn={d.conn}
          db={d.db}
          coll={d.coll}
          onClose={onClose}
          onDone={() => {
            toast({ kind: 'success', title: `Renamed ${d.db}.${d.coll}` })
            loadDbs(d.conn.id)
            loadColls(d.conn.id, d.db)
            onClose()
          }}
        />
      ) : (
        <RenameDatabaseDialog conn={d.conn} db={d.db} onClose={() => (onClose(), loadDbs(d.conn.id))} />
      )
  }
}
