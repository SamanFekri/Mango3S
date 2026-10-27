import { useState } from 'react'
import { INSERT_TEMPLATE } from '../../components/DocumentEditor'
import { backend } from '../../lib/backend'
import { toAppError } from '../../lib/errors'
import type { Doc } from '../../lib/types'
import { useApp } from '../../state/AppContext'
import type { usePagedQuery } from './usePagedQuery'

export interface EditorState {
  mode: 'insert' | 'edit'
  doc?: Doc
  text: string
}

/** Editing documents of a collection tab: the document editor, deletes,
 * bulk updates and re-reading one document. */
export function useDocumentActions(
  { connId, db, coll, readOnly }: { connId: string; db: string; coll: string; readOnly: boolean },
  results: Pick<ReturnType<typeof usePagedQuery>, 'replaceDocument' | 'removeDocument' | 'changed'>,
) {
  const { settings, toast, toastError, confirm } = useApp()
  const [editor, setEditor] = useState<EditorState | null>(null)
  const [bulk, setBulk] = useState<{ kind: 'update' | 'remove'; ids: string[] } | null>(null)

  const insert = (text = INSERT_TEMPLATE) => (readOnly ? false : setEditor({ mode: 'insert', text }))

  const edit = async (doc: Doc) => {
    // Re-read so the editor starts from the current server version.
    try {
      const fresh = await backend.getDocument(connId, db, coll, doc.id)
      setEditor({ mode: 'edit', doc: fresh, text: fresh.text })
    } catch (err) {
      toastError(err, 'Could not open document')
    }
  }

  /** Saves the open editor: inserts, or replaces the edited document. */
  const saveEditor = async (text: string) => {
    if (!editor) return
    if (editor.mode === 'insert') {
      const ids = await backend.insertDocuments(connId, db, coll, text)
      toast({ kind: 'success', title: `Inserted ${ids.length} document${ids.length === 1 ? '' : 's'}` })
    } else {
      await backend.replaceDocument(connId, db, coll, editor.doc!.id, text)
      toast({ kind: 'success', title: 'Document saved' })
    }
    setEditor(null)
    results.changed()
  }

  const remove = async (docs: Doc[], opts?: { keyboard?: boolean }) => {
    if (settings?.confirmDeletes !== false || opts?.keyboard) {
      const ok = await confirm({
        title: docs.length === 1 ? 'Delete document' : `Delete ${docs.length} documents`,
        message: (
          <>
            Permanently delete {docs.length === 1 ? <code>{docs[0].id}</code> : `${docs.length} documents`} from{' '}
            <code>
              {db}.{coll}
            </code>
            ?
          </>
        ),
        confirmLabel: 'Delete',
        danger: true,
      })
      if (!ok) return
    }
    try {
      const n = await backend.deleteDocuments(connId, db, coll, docs.map((d) => d.id))
      toast({ kind: 'success', title: `Deleted ${n} document${n === 1 ? '' : 's'}` })
      if (n < docs.length) toast({ kind: 'info', title: `${docs.length - n} document(s) were already gone` })
      results.changed()
    } catch (err) {
      toastError(err, 'Delete failed')
    }
  }

  const refreshDocument = async (doc: Doc) => {
    try {
      results.replaceDocument(await backend.getDocument(connId, db, coll, doc.id))
    } catch (err) {
      const e = toAppError(err)
      if (e.code === 'NOT_FOUND') {
        results.removeDocument(doc.id)
        toast({ kind: 'info', title: 'Document is gone', message: 'It was deleted since the page loaded.' })
      } else toastError(err, 'Could not refresh the document')
    }
  }

  return { editor, setEditor, bulk, setBulk, insert, edit, saveEditor, remove, refreshDocument }
}
