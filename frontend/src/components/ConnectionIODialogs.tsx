import { useState } from 'react'
import { backend } from '../lib/backend'
import { toAppError, type AppError } from '../lib/errors'
import type { ConnectionImportItem, ConnectionImportPreview, ImportChoice, Profile } from '../lib/types'
import { useApp } from '../state/AppContext'
import { ErrorBanner, Modal, Spinner } from './common'

const MIN_PASSPHRASE = 10

const FORMAT_LABEL: Record<string, string> = {
  mango3s: 'Mango3S connections file',
  compass: 'MongoDB Compass export',
  uris: 'List of connection strings',
}

/** Export connections to a file. Secrets stay out unless explicitly included. */
export function ConnectionExportDialog({ selected, onClose }: { selected: Profile | null; onClose: () => void }) {
  const { profiles, toast } = useApp()
  const [scope, setScope] = useState<'selected' | 'all'>(selected ? 'selected' : 'all')
  const [withSecrets, setWithSecrets] = useState(false)
  const [pass, setPass] = useState('')
  const [pass2, setPass2] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const ids = scope === 'selected' && selected ? [selected.id] : []
  const count = scope === 'selected' && selected ? 1 : profiles.length
  const passOk = !withSecrets || (pass.length >= MIN_PASSPHRASE && pass === pass2)

  const run = async () => {
    setBusy(true)
    setError(null)
    try {
      const r = await backend.exportConnections(ids, withSecrets, withSecrets ? pass : '')
      if (!r.path) return
      toast({ kind: 'success', title: `Exported ${r.count} connection${r.count === 1 ? '' : 's'}`, message: r.path })
      onClose()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal
      title="Export connections"
      onClose={onClose}
      footer={
        <>
          <span className="spacer" />
          <button onClick={onClose}>Cancel</button>
          <button className="primary" disabled={busy || !passOk || count === 0} onClick={run}>
            {busy ? <Spinner small /> : 'Export…'}
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <div className="check-row">
        <label className="check">
          <input type="radio" name="export-scope" checked={scope === 'selected'} disabled={!selected} onChange={() => setScope('selected')} />
          {selected ? `“${selected.name}”` : 'Selected connection'}
        </label>
        <label className="check">
          <input type="radio" name="export-scope" checked={scope === 'all'} onChange={() => setScope('all')} />
          All {profiles.length} connections (with their folders)
        </label>
      </div>
      <p className="muted small">
        Every setting is exported: servers, authentication user and mechanism, TLS, SSH and proxy settings, folders and options. Certificate and key files are referenced by path;
        their contents are never exported.
      </p>
      <label className="check">
        <input type="checkbox" checked={withSecrets} onChange={(e) => setWithSecrets(e.target.checked)} /> Include passwords and other secrets
      </label>
      {withSecrets && (
        <>
          <p className="warning-text small">
            Anyone with this file and its passphrase can connect to these servers. The secrets are encrypted (AES-256-GCM, scrypt) with the passphrase below; share it separately
            from the file.
          </p>
          <div className="field-row">
            <label className="field">
              <span>Passphrase (at least {MIN_PASSPHRASE} characters)</span>
              <input type="password" value={pass} onChange={(e) => setPass(e.target.value)} autoComplete="new-password" aria-label="Passphrase" />
            </label>
            <label className="field">
              <span>Repeat passphrase</span>
              <input type="password" value={pass2} onChange={(e) => setPass2(e.target.value)} autoComplete="new-password" aria-label="Repeat passphrase" />
            </label>
          </div>
          {pass2 && pass !== pass2 && <p className="error-text small">The passphrases differ.</p>}
        </>
      )}
    </Modal>
  )
}

type Row = ImportChoice & { item: ConnectionImportItem }

const defaultAction = (it: ConnectionImportItem): ImportChoice['action'] => (it.error ? 'skip' : it.conflict === 'id' ? 'skip' : 'add')

/** Import connections from a Mango3S file, a Compass export or a list of URIs. */
export function ConnectionImportDialog({ onClose }: { onClose: () => void }) {
  const { reloadProfiles, toast } = useApp()
  const [preview, setPreview] = useState<ConnectionImportPreview | null>(null)
  const [rows, setRows] = useState<Row[]>([])
  const [pass, setPass] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AppError | null>(null)

  const load = async (path: string, passphrase: string) => {
    setBusy(true)
    setError(null)
    try {
      const p = await backend.previewConnectionImport(path, passphrase)
      // Unlocking the same file keeps the choices already made.
      const keep = preview?.path === path && rows.length === p.items.length
      setPreview(p)
      setRows(
        p.items.map((it, i) => ({
          index: it.index,
          action: keep ? rows[i].action : defaultAction(it),
          name: keep ? rows[i].name : it.conflict === 'name' ? `${it.name} (imported)` : it.name,
          folder: keep ? rows[i].folder : it.folder,
          item: it,
        })),
      )
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  const choose = async () => {
    const path = await backend.chooseConnectionsFile()
    if (path) {
      setPass('')
      await load(path, '')
    }
  }
  const apply = async () => {
    if (!preview) return
    setBusy(true)
    setError(null)
    try {
      const r = await backend.applyConnectionImport(
        preview.path,
        preview.secretsUnlocked ? pass : '',
        rows.map(({ index, action, name, folder }) => ({ index, action, name, folder })),
      )
      await reloadProfiles()
      const parts = [r.added && `${r.added} added`, r.replaced && `${r.replaced} replaced`, r.skipped && `${r.skipped} skipped`].filter(Boolean).join(', ')
      if (r.errors.length) {
        setError({ code: 'INVALID_INPUT', message: `Imported with problems (${parts || 'nothing imported'}).`, detail: r.errors.join('\n') })
        return
      }
      toast({ kind: 'success', title: 'Connections imported', message: parts })
      onClose()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }
  const update = (i: number, patch: Partial<Row>) => setRows(rows.map((r, k) => (k === i ? { ...r, ...patch } : r)))
  const n = rows.filter((r) => r.action !== 'skip').length
  return (
    <Modal
      title="Import connections"
      onClose={onClose}
      wide
      footer={
        <>
          <span className="spacer" />
          <button onClick={onClose}>Cancel</button>
          <button className="primary" disabled={busy || !preview || n === 0} onClick={apply}>
            Import {n || ''}
          </button>
        </>
      }
    >
      <ErrorBanner error={error} onDismiss={() => setError(null)} />
      <div className="file-row">
        <input readOnly value={preview?.path ?? ''} placeholder="Mango3S connections file, MongoDB Compass export, or a text file of connection strings" aria-label="File" />
        <button onClick={choose} disabled={busy}>
          Choose file…
        </button>
      </div>
      {busy && <Spinner small />}
      {preview && (
        <>
          <p className="muted small">
            {FORMAT_LABEL[preview.format] ?? preview.format}: {preview.items.length} connection{preview.items.length === 1 ? '' : 's'}. Passwords are saved in the system credential
            store, never in the settings file.
          </p>
          {preview.warnings.map((w) => (
            <p key={w} className="warning-text small">
              {w}
            </p>
          ))}
          {preview.hasSecrets && (
            <div className="file-row">
              <input
                type="password"
                value={pass}
                onChange={(e) => setPass(e.target.value)}
                placeholder="Passphrase for the passwords in this file"
                aria-label="Passphrase"
                disabled={preview.secretsUnlocked}
              />
              <button onClick={() => void load(preview.path, pass)} disabled={busy || !pass || preview.secretsUnlocked}>
                {preview.secretsUnlocked ? 'Unlocked' : 'Unlock'}
              </button>
            </div>
          )}
          <div className="backup-entries">
            <table className="grid compact">
              <thead>
                <tr>
                  <th>Action</th>
                  <th>Name</th>
                  <th>Folder</th>
                  <th>Server</th>
                  <th>Notes</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r, i) => (
                  <tr key={r.index}>
                    <td>
                      <select value={r.action} aria-label={`Action for ${r.item.name}`} disabled={!!r.item.error} onChange={(e) => update(i, { action: e.target.value as ImportChoice['action'] })}>
                        <option value="add">Add</option>
                        {r.item.conflict === 'id' && <option value="replace">Replace existing</option>}
                        <option value="skip">Skip</option>
                      </select>
                    </td>
                    <td>
                      <input value={r.name} aria-label={`Name for ${r.item.name}`} disabled={r.action !== 'add'} onChange={(e) => update(i, { name: e.target.value })} />
                    </td>
                    <td>
                      <input value={r.folder} aria-label={`Folder for ${r.item.name}`} disabled={r.action === 'skip'} onChange={(e) => update(i, { folder: e.target.value })} />
                    </td>
                    <td className="mono small">
                      {r.item.hosts}
                      {r.item.via && <div className="muted">via {r.item.via}</div>}
                    </td>
                    <td className="small">
                      {r.item.error && <div className="error-text">{r.item.error}</div>}
                      {r.item.conflict === 'id' && <div className="warning-text">Already saved (same connection).</div>}
                      {r.item.conflict === 'name' && <div className="warning-text">A connection with this name exists.</div>}
                      {r.item.secretKinds.length > 0 && <div className="muted">With {r.item.secretKinds.length} saved secret(s)</div>}
                      {r.item.notes.map((x) => (
                        <div key={x} className="muted">
                          {x}
                        </div>
                      ))}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </Modal>
  )
}
