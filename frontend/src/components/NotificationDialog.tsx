import { Modal } from './common'
import { useApp } from '../state/AppContext'

/** Shows one queued platform notification at a time (oldest first); dismissing
 * acknowledges it so it won't be shown again, then advances to the next. */
export function NotificationDialog() {
  const { notificationQueue, dismissNotification } = useApp()
  const current = notificationQueue[0]
  if (!current) return null

  return (
    <Modal
      title={current.title}
      onClose={() => dismissNotification(current.id)}
      footer={
        <button className="primary" onClick={() => dismissNotification(current.id)}>
          Dismiss{notificationQueue.length > 1 ? ` (${notificationQueue.length - 1} more)` : ''}
        </button>
      }
    >
      <p className="confirm-message">{current.body}</p>
    </Modal>
  )
}
