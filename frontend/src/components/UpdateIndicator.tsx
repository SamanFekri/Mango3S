import { useApp } from '../state/AppContext'

/** Status-bar button shown only when there's something worth the user's
 * attention: an update ready/available, or updates paused by the admin. An
 * unreachable update server shows nothing — it isn't something the user needs
 * to act on, and the app doesn't depend on it. */
export function UpdateIndicator() {
  const { updateStatus, setUpdateDialogOpen } = useApp()
  if (!updateStatus) return null

  if (updateStatus.revoked) {
    return (
      <button className="statusbar-btn" onClick={() => setUpdateDialogOpen(true)}>
        Updates paused
      </button>
    )
  }
  if (updateStatus.state === 'ready_to_install') {
    return (
      <button className="statusbar-btn" onClick={() => setUpdateDialogOpen(true)}>
        Update ready · v{updateStatus.latestVersion}
      </button>
    )
  }
  if (updateStatus.state === 'update_available' || updateStatus.state === 'downloading') {
    return (
      <button className="statusbar-btn" onClick={() => setUpdateDialogOpen(true)}>
        {updateStatus.state === 'downloading' ? 'Downloading update…' : `Update available · v${updateStatus.latestVersion}`}
      </button>
    )
  }
  return null
}
