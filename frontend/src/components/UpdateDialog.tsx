import { useState } from 'react'
import { Modal, Spinner } from './common'
import { backend, quitApp } from '../lib/backend'
import { useApp } from '../state/AppContext'

/** Shown automatically once a downloaded update is verified and ready — asks
 * before installing (per the chosen "confirm before install" behavior), and
 * also doubles as a manual "update available" / "revoked" status view when
 * opened from the status-bar indicator. */
export function UpdateDialog() {
  const { updateStatus, updateDialogOpen, setUpdateDialogOpen, toastError, toast } = useApp()
  const [installing, setInstalling] = useState(false)
  if (!updateDialogOpen || !updateStatus) return null

  const close = () => setUpdateDialogOpen(false)

  const install = async () => {
    setInstalling(true)
    try {
      const restartRequired = await backend.applyPendingUpdate()
      if (restartRequired) {
        quitApp() // the replaced binary runs fresh on the next launch
      } else {
        toast({ kind: 'info', title: 'Installer opened', message: 'Finish the installer, then quit and reopen Mango3S.' })
        close()
      }
    } catch (err) {
      toastError(err, 'Could not install the update')
    } finally {
      setInstalling(false)
    }
  }

  if (updateStatus.revoked) {
    return (
      <Modal title="Updates paused" onClose={close} footer={<button onClick={close}>Close</button>}>
        <p>
          The Mango3S administrator has paused updates and announcements for this installation.
          Everything else in Mango3S works as usual.
        </p>
      </Modal>
    )
  }

  switch (updateStatus.state) {
    case 'ready_to_install':
      return (
        <Modal
          title="Update ready to install"
          onClose={close}
          footer={
            <>
              <button onClick={close} disabled={installing}>
                Later
              </button>
              <button className="primary" onClick={install} disabled={installing}>
                {installing && <Spinner small />} Restart &amp; install
              </button>
            </>
          }
        >
          <p>
            Version <strong>{updateStatus.latestVersion}</strong> has been downloaded and verified.
          </p>
          {updateStatus.releaseNotes && <p className="muted small">{updateStatus.releaseNotes}</p>}
        </Modal>
      )
    case 'update_available':
    case 'downloading':
      return (
        <Modal title="Update available" onClose={close} footer={<button onClick={close}>Close</button>}>
          <p>
            Version <strong>{updateStatus.latestVersion}</strong> is available
            {updateStatus.state === 'downloading' ? ' and is downloading now' : ''} — see the Jobs panel
            in the status bar for progress.
          </p>
        </Modal>
      )
    case 'error':
      return (
        <Modal title="Update check failed" onClose={close} footer={<button onClick={close}>Close</button>}>
          <p className="error-text">{updateStatus.error || 'Something went wrong checking for updates.'}</p>
        </Modal>
      )
    default:
      return null
  }
}
