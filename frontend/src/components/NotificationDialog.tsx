import { Modal } from './common'
import { backend } from '../lib/backend'
import type { NotificationButton } from '../lib/types'
import { useApp } from '../state/AppContext'

/** Shows one queued platform notification at a time (oldest first): its
 * title, body and buttons, plus the dialog's own close (X). Every answer is
 * reported to the platform ("close" for the X), which also marks it read, and
 * the next notification follows. */
export function NotificationDialog() {
  const { notificationQueue, dismissNotification, toastError } = useApp()
  const current = notificationQueue[0]
  if (!current) return null

  const buttons = (current.buttons ?? []).slice(0, 3)
  const more = notificationQueue.length > 1 ? ` (${notificationQueue.length - 1} more)` : ''

  const press = (b: NotificationButton) => {
    if (b.action === 'open_link' && b.url) {
      backend.openPlatformLink(b.url).catch((err) => toastError(err, 'Could not open the link'))
    }
    dismissNotification(current.id, b.id)
  }

  return (
    <Modal
      title={current.title}
      onClose={() => dismissNotification(current.id, 'close')}
      footer={
        buttons.length > 0 ? (
          <>
            {more && <span className="muted small">{more.trim()}</span>}
            <span className="spacer" />
            {buttons.map((b, i) => (
              <button key={b.id} className={b.style === 'primary' ? 'primary' : ''} onClick={() => press(b)} autoFocus={i === 0 && b.style === 'primary'}>
                {b.label}
                {b.action === 'open_link' && <span aria-hidden> ↗</span>}
              </button>
            ))}
          </>
        ) : (
          <button className="primary" onClick={() => dismissNotification(current.id, 'close')} autoFocus>
            Dismiss{more}
          </button>
        )
      }
    >
      <p className="confirm-message notification-body">{current.body}</p>
    </Modal>
  )
}
