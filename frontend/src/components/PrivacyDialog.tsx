import { useState } from 'react'
import { Modal, Spinner } from './common'
import { backend } from '../lib/backend'
import { renderMarkdown } from '../lib/markdown'
import { useApp } from '../state/AppContext'

/** Asks the user to accept a new privacy policy when the Mango3S server
 * requires it. "Later" closes it for now; it comes back on the next launch.
 * Local work is never blocked — only online account features wait. */
export function PrivacyDialog() {
  const { privacyPolicy: policy, privacyDialogOpen, setPrivacyDialogOpen, acceptPrivacy, toast, toastError } = useApp()
  const [showFull, setShowFull] = useState(false)
  const [busy, setBusy] = useState(false)
  if (!policy || !privacyDialogOpen) return null

  const open = (url: string) => backend.openPlatformLink(url).catch((err) => toastError(err, 'Could not open the link'))
  const accept = async () => {
    setBusy(true)
    try {
      await acceptPrivacy(policy.version)
      toast({ kind: 'success', title: 'Thanks — privacy policy accepted' })
    } catch (err) {
      toastError(err, 'Could not save your answer')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Privacy policy updated"
      onClose={() => setPrivacyDialogOpen(false)}
      footer={
        <>
          {policy.url && (
            <button className="ghost" onClick={() => open(policy.url)}>
              Open in browser ↗
            </button>
          )}
          <span className="spacer" />
          <button onClick={() => setPrivacyDialogOpen(false)} disabled={busy}>
            Later
          </button>
          <button className="primary" onClick={accept} disabled={busy} autoFocus>
            {busy && <Spinner small />} Accept
          </button>
        </>
      }
    >
      <div className="privacy-dialog">
        <p>
          {policy.title || 'The privacy policy'} changed
          {policy.publishedAt ? ` on ${new Date(policy.publishedAt).toLocaleDateString()}` : ''}. Please read and accept it to keep using
          online features such as account sync. Everything else in Mango3S works as usual.
        </p>
        {policy.summary && (
          <div className="privacy-summary">
            <strong>What changed</strong>
            <p>{policy.summary}</p>
          </div>
        )}
        {policy.body && (
          <>
            <button className="link" onClick={() => setShowFull(!showFull)} aria-expanded={showFull}>
              {showFull ? 'Hide the full policy' : 'Read the full policy'}
            </button>
            {showFull && (
              <div className="markdown privacy-body" tabIndex={0} aria-label="Privacy policy text">
                {renderMarkdown(policy.body, open)}
              </div>
            )}
          </>
        )}
      </div>
    </Modal>
  )
}
