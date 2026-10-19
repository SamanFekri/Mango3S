import { PasswordInput } from './PasswordInput'
import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Spinner } from './common'
import { backend } from '../lib/backend'
import { toAppError } from '../lib/errors'
import { passwordProblems } from '../lib/password'
import type { AccountState, SyncReport } from '../lib/types'
import { useApp } from '../state/AppContext'

type Mode = 'signin' | 'signup' | 'verify' | 'forgot' | 'reset'

/** Settings → Account: an optional Mango3S account that backs connections up,
 * encrypted, so they can be restored on another computer. Everything here
 * talks to the Mango3S server; the rest of the app never needs it. */
export function AccountSettings() {
  const { accountState, setAccountState, privacyPolicy, openPrivacy, toastError } = useApp()
  const [recoveryKey, setRecoveryKey] = useState<{ key: string; why: string } | null>(null)

  // Refresh from the server when the tab opens (sync limits, linked computers).
  useEffect(() => {
    if (!accountState?.signedIn) return
    backend
      .refreshAccount()
      .then(setAccountState)
      .catch((err) => {
        if (toAppError(err).code === 'AUTH_FAILED') backend.getAccountState().then(setAccountState).catch(() => undefined)
      })
    // Only when the tab opens or the signed-in account changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accountState?.signedIn, accountState?.email])

  const reviewPrivacy = async () => {
    if (privacyPolicy) return openPrivacy()
    try {
      const p = await backend.getPrivacyPolicy()
      if (p?.acceptanceRequired) openPrivacy(p)
      else setAccountState({ ...(accountState ?? emptyState), privacyPending: false })
    } catch (err) {
      toastError(err, 'Could not load the privacy policy')
    }
  }

  return (
    <section className="settings-group" id="settings-account">
      <h2>Mango3S account</h2>
      <p className="muted small">
        Optional. An account backs up your connections — including their passwords — encrypted with your account password, so you can
        restore them on another computer. Without one, everything stays on this computer.
      </p>
      {accountState?.privacyPending && (
        <div className="info-banner" role="status">
          The privacy policy changed. Accept it to use your account.{' '}
          <button className="link" onClick={reviewPrivacy}>
            Review it
          </button>
        </div>
      )}
      {recoveryKey ? (
        <RecoveryKeyPanel recoveryKey={recoveryKey.key} why={recoveryKey.why} onDone={() => setRecoveryKey(null)} />
      ) : accountState?.signedIn ? (
        <SignedIn state={accountState} onRecoveryKey={(key) => setRecoveryKey({ key, why: 'This is your new recovery key. The old one no longer works.' })} />
      ) : (
        <SignedOut
          state={accountState ?? emptyState}
          onRecoveryKey={(key, why) => setRecoveryKey({ key, why })}
          onPrivacyRequired={reviewPrivacy}
        />
      )}
    </section>
  )
}

const emptyState: AccountState = {
  signedIn: false, email: '', account: null, disabled: {}, syncing: false, lastSyncAt: '', lastSyncError: '',
  pendingChanges: 0, restoreSuggested: false, privacyPending: false,
}

/** Runs an account action with a busy flag and a friendly error toast. */
function useAction() {
  const { toastError, setAccountState } = useApp()
  const [busy, setBusy] = useState<string | null>(null)
  const run = async <T,>(name: string, fn: () => Promise<T>, failTitle: string, onPrivacy?: () => void): Promise<T | undefined> => {
    setBusy(name)
    try {
      return await fn()
    } catch (err) {
      const e = toAppError(err)
      if (e.detail === 'privacy_required' && onPrivacy) onPrivacy()
      else toastError(err, failTitle)
      // The backend may have learned something (feature turned off, session expired).
      backend.getAccountState().then(setAccountState).catch(() => undefined)
      return undefined
    } finally {
      setBusy(null)
    }
  }
  return { busy, run }
}

function PasswordHints({ password, email }: { password: string; email: string }) {
  if (!password) return null
  const problems = passwordProblems(password, email)
  if (problems.length === 0) return <p className="small password-ok">Strong enough.</p>
  return (
    <ul className="small password-hints" aria-label="Password requirements">
      {problems.map((p) => (
        <li key={p}>{p}</li>
      ))}
    </ul>
  )
}

// ---- signed out ---------------------------------------------------------------

function SignedOut({
  state,
  onRecoveryKey,
  onPrivacyRequired,
}: {
  state: AccountState
  onRecoveryKey: (key: string, why: string) => void
  onPrivacyRequired: () => void
}) {
  const { setAccountState, toast } = useApp()
  const off = state.disabled ?? {}
  const [mode, setMode] = useState<Mode>(off.login && !off.registration ? 'signup' : 'signin')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPw, setConfirmPw] = useState('')
  const [code, setCode] = useState('')
  const [recovery, setRecovery] = useState('')
  const [noKey, setNoKey] = useState(false)
  const [sentTo, setSentTo] = useState('')
  const { busy, run } = useAction()

  if (off.login && off.registration) {
    return <p className="muted">Accounts are turned off on the Mango3S server right now. Your connections stay on this computer.</p>
  }

  const go = (m: Mode) => {
    setMode(m)
    setPassword('')
    setConfirmPw('')
    setCode('')
  }
  const signedIn = (res: { state: AccountState; recoveryKey?: string; dataKept?: boolean | null } | undefined, welcome: string) => {
    if (!res) return
    setAccountState(res.state)
    if (res.recoveryKey) {
      onRecoveryKey(
        res.recoveryKey,
        res.dataKept === false
          ? 'Your synced data was erased, so your account has a new recovery key.'
          : 'This key is the only way to keep your synced connections if you forget your password.',
      )
    }
    toast({ kind: 'success', title: welcome })
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (mode === 'signin') {
      signedIn(await run('signin', () => backend.accountSignIn(email, password), 'Could not sign in', onPrivacyRequired), 'Signed in')
    } else if (mode === 'signup') {
      const sent = await run('signup', () => backend.accountStartSignUp(email, password), 'Could not start sign-up', onPrivacyRequired)
      if (sent) {
        setSentTo(sent.codeSentTo)
        setMode('verify')
      }
    } else if (mode === 'verify') {
      signedIn(await run('verify', () => backend.accountFinishSignUp(email, code, password), 'Could not create the account'), 'Account created')
    } else if (mode === 'forgot') {
      const sent = await run('forgot', () => backend.accountForgotPassword(email), 'Could not send a code')
      if (sent) {
        setSentTo(sent.codeSentTo)
        setMode('reset')
      }
    } else {
      signedIn(
        await run(
          'reset',
          () => backend.accountResetPassword({ email, code, newPassword: password, recoveryKey: noKey ? '' : recovery, discardSyncedData: noKey }),
          'Could not reset the password',
        ),
        'Password changed',
      )
    }
  }

  const weak = passwordProblems(password, email).length > 0
  const mismatch = (mode === 'signup' || mode === 'reset') && confirmPw !== password
  const field = (label: string, input: ReactNode) => (
    <label className="field">
      <span>{label}</span>
      {input}
    </label>
  )
  const emailField = field('Email', <input type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} required />)

  return (
    <form className="account-form" onSubmit={submit}>
      {mode !== 'verify' && mode !== 'reset' && (
        <div className="segmented" role="tablist" aria-label="Account">
          {!off.login && (
            <button type="button" role="tab" aria-selected={mode === 'signin'} className={mode === 'signin' ? 'active' : ''} onClick={() => go('signin')}>
              Sign in
            </button>
          )}
          {!off.registration && (
            <button type="button" role="tab" aria-selected={mode === 'signup'} className={mode === 'signup' ? 'active' : ''} onClick={() => go('signup')}>
              Create account
            </button>
          )}
        </div>
      )}

      {mode === 'signin' && (
        <>
          {emailField}
          {field('Password', <PasswordInput autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />)}
          <div className="form-actions">
            <button className="primary" type="submit" disabled={!!busy || !email || !password}>
              {busy && <Spinner small />} Sign in
            </button>
            {!off.passwordReset && (
              <button type="button" className="link" onClick={() => go('forgot')}>
                Forgot password?
              </button>
            )}
          </div>
        </>
      )}

      {mode === 'signup' && (
        <>
          {emailField}
          {field('Password', <PasswordInput autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} required />)}
          <PasswordHints password={password} email={email} />
          {field('Repeat password', <PasswordInput autoComplete="new-password" value={confirmPw} onChange={(e) => setConfirmPw(e.target.value)} required />)}
          {confirmPw && mismatch && <p className="small error-text">The passwords don't match.</p>}
          <p className="muted small">We'll email you a 6-digit code to confirm the address.</p>
          <div className="form-actions">
            <button className="primary" type="submit" disabled={!!busy || !email || weak || mismatch}>
              {busy && <Spinner small />} Send code
            </button>
          </div>
        </>
      )}

      {mode === 'verify' && (
        <>
          <p>
            Enter the code we sent to <strong>{sentTo || email}</strong>.
          </p>
          {field(
            'Code',
            <input inputMode="numeric" autoComplete="one-time-code" maxLength={6} value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))} autoFocus />,
          )}
          <div className="form-actions">
            <button className="primary" type="submit" disabled={!!busy || code.length !== 6}>
              {busy && <Spinner small />} Create account
            </button>
            <button type="button" onClick={() => go('signup')} disabled={!!busy}>
              Back
            </button>
          </div>
        </>
      )}

      {mode === 'forgot' && (
        <>
          <p className="muted small">We'll email you a code to set a new password.</p>
          {emailField}
          <div className="form-actions">
            <button className="primary" type="submit" disabled={!!busy || !email}>
              {busy && <Spinner small />} Send code
            </button>
            <button type="button" onClick={() => go('signin')} disabled={!!busy}>
              Back
            </button>
          </div>
        </>
      )}

      {mode === 'reset' && (
        <>
          <p>
            Enter the code we sent to <strong>{sentTo || email}</strong> and choose a new password.
          </p>
          {field('Code', <input inputMode="numeric" maxLength={6} value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))} autoFocus />)}
          {field('New password', <PasswordInput autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />)}
          <PasswordHints password={password} email={email} />
          {field('Repeat new password', <PasswordInput autoComplete="new-password" value={confirmPw} onChange={(e) => setConfirmPw(e.target.value)} />)}
          {!noKey && field('Recovery key', <input className="mono" value={recovery} onChange={(e) => setRecovery(e.target.value)} placeholder="XXXX-XXXX-…" />)}
          <label className="check">
            <input type="checkbox" checked={noKey} onChange={(e) => setNoKey(e.target.checked)} />I don't have my recovery key
          </label>
          {noKey && (
            <div className="error-banner" role="alert">
              <div>
                <strong>Your synced connections will be erased.</strong>
                <div className="error-message">
                  Without the recovery key nobody — not even the administrator — can decrypt them. Connections on your computers are not
                  affected; you can sync them again afterwards.
                </div>
              </div>
            </div>
          )}
          <div className="form-actions">
            <button
              className={noKey ? 'danger' : 'primary'}
              type="submit"
              disabled={!!busy || code.length !== 6 || weak || mismatch || (!noKey && !recovery.trim())}
            >
              {busy && <Spinner small />} {noKey ? 'Erase synced data and reset' : 'Reset password'}
            </button>
            <button type="button" onClick={() => go('forgot')} disabled={!!busy}>
              Back
            </button>
          </div>
        </>
      )}
    </form>
  )
}

/** Shown once: the server can never show the recovery key again. */
function RecoveryKeyPanel({ recoveryKey, why, onDone }: { recoveryKey: string; why: string; onDone: () => void }) {
  const { toast } = useApp()
  const [saved, setSaved] = useState(false)
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(recoveryKey)
      toast({ kind: 'success', title: 'Recovery key copied' })
    } catch {
      toast({ kind: 'error', title: 'Could not copy', message: 'Select the key and copy it by hand.' })
    }
  }
  return (
    <div className="recovery-key" role="region" aria-label="Recovery key">
      <h3>Save your recovery key</h3>
      <p className="small">{why} It will not be shown again — store it somewhere safe, like a password manager.</p>
      <code className="recovery-key-value" aria-label="Your recovery key">
        {recoveryKey}
      </code>
      <div className="form-actions">
        <button onClick={copy}>Copy</button>
      </div>
      <label className="check">
        <input type="checkbox" checked={saved} onChange={(e) => setSaved(e.target.checked)} />I saved my recovery key
      </label>
      <div className="form-actions">
        <button className="primary" disabled={!saved} onClick={onDone}>
          Continue
        </button>
      </div>
    </div>
  )
}

// ---- signed in ------------------------------------------------------------------

type PasswordAction = 'restore' | 'rotate' | 'wipe' | 'delete' | 'change'

function SignedIn({ state, onRecoveryKey }: { state: AccountState; onRecoveryKey: (key: string) => void }) {
  const { settings, saveSettings, setAccountState, toast, toastError, confirm } = useApp()
  const { busy, run } = useAction()
  const [action, setAction] = useState<PasswordAction | null>(null)
  const [password, setPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const acct = state.account
  const syncOff = !!state.disabled?.sync

  const refresh = () => backend.getAccountState().then(setAccountState).catch(() => undefined)
  const reportToast = (title: string, r: SyncReport) => {
    const parts = [
      r.uploaded && `${r.uploaded} uploaded`,
      r.deleted && `${r.deleted} deletion${r.deleted === 1 ? '' : 's'} sent`,
      r.added && `${r.added} added`,
      r.updated && `${r.updated} updated`,
      r.removed && `${r.removed} removed`,
      r.skipped && `${r.skipped} newer on the server`,
    ].filter(Boolean)
    toast({ kind: r.errors?.length ? 'error' : 'success', title, message: [parts.join(', ') || 'Everything was already in sync', ...(r.errors ?? [])].join('\n') })
  }

  const syncNow = async () => {
    const r = await run('sync', () => backend.syncNow(), 'Sync failed')
    if (r) reportToast('Connections synced', r)
    refresh()
  }
  const setAuto = async (on: boolean) => {
    if (!settings) return
    try {
      await saveSettings({ ...settings, syncAuto: on })
    } catch (err) {
      toastError(err, 'Could not save the setting')
    }
  }
  const signOut = async () => {
    const ok = await confirm({ title: 'Sign out?', message: 'Your connections stay on this computer. Automatic backup stops until you sign in again.', confirmLabel: 'Sign out' })
    if (!ok) return
    const s = await run('signout', () => backend.accountSignOut(), 'Could not sign out')
    if (s) setAccountState(s)
  }

  const submitPassword = async (e: FormEvent) => {
    e.preventDefault()
    const done = () => {
      setAction(null)
      setPassword('')
      setNewPassword('')
    }
    if (action === 'restore') {
      const r = await run('restore', () => backend.syncRestore(password), 'Restore failed')
      if (r) {
        reportToast('Connections restored', r)
        done()
      }
    } else if (action === 'rotate') {
      const key = await run('rotate', () => backend.accountRotateRecoveryKey(password), 'Could not create a new recovery key')
      if (key) {
        done()
        onRecoveryKey(key)
      }
    } else if (action === 'change') {
      const ok = await run('change', () => backend.accountChangePassword(password, newPassword).then(() => true), 'Could not change the password')
      if (ok) {
        toast({ kind: 'success', title: 'Password changed', message: 'Your other computers were signed out.' })
        done()
      }
    } else if (action === 'wipe') {
      const ok = await confirm({
        title: 'Erase synced data?',
        message: 'Every connection backed up to your account is deleted from the server. Connections on your computers are not affected. Automatic backup is turned off.',
        confirmLabel: 'Erase',
        danger: true,
      })
      if (!ok) return
      const wiped = await run('wipe', () => backend.syncWipe(password).then(() => true), 'Could not erase the synced data')
      if (wiped) {
        toast({ kind: 'success', title: 'Synced data erased' })
        done()
      }
    } else if (action === 'delete') {
      const ok = await confirm({
        title: 'Delete your account?',
        message: 'Your account and everything synced to it are deleted for good. Connections on this computer stay.',
        confirmLabel: 'Delete account',
        danger: true,
        typeToConfirm: 'DELETE',
      })
      if (!ok) return
      const deleted = await run('delete', () => backend.accountDelete(password).then(() => true), 'Could not delete the account')
      if (deleted) {
        toast({ kind: 'success', title: 'Account deleted' })
        done()
      }
    }
    refresh()
  }

  const actionLabel: Record<PasswordAction, string> = {
    restore: 'Restore connections',
    rotate: 'Create new recovery key',
    change: 'Change password',
    wipe: 'Erase synced data',
    delete: 'Delete account',
  }
  const machines = acct?.machines ?? []

  return (
    <div className="account-signed-in">
      <dl className="stats">
        <dt>Signed in as</dt>
        <dd>{state.email || acct?.email}</dd>
        {acct && (
          <>
            <dt>On the server</dt>
            <dd>
              {acct.sync.itemCount} of {acct.sync.maxItems} items
              {acct.sync.lastSyncedAt ? ` · last change ${new Date(acct.sync.lastSyncedAt).toLocaleString()}` : ''}
            </dd>
          </>
        )}
        <dt>This computer</dt>
        <dd>
          {state.pendingChanges > 0 ? `${state.pendingChanges} change${state.pendingChanges === 1 ? '' : 's'} not uploaded yet` : 'Up to date'}
          {state.lastSyncAt ? ` · synced ${new Date(state.lastSyncAt).toLocaleString()}` : ''}
        </dd>
        {machines.length > 0 && (
          <>
            <dt>Computers</dt>
            <dd>{machines.map((m) => m.label || m.machineId.slice(0, 8)).join(', ')}</dd>
          </>
        )}
      </dl>
      {syncOff && <p className="muted small">Sync is turned off on the Mango3S server right now. Your synced data there is untouched.</p>}
      {state.lastSyncError && !syncOff && <p className="small error-text">Last sync failed: {state.lastSyncError}</p>}
      {state.restoreSuggested && (
        <p className="small">Another computer has newer versions of some connections. Use Restore to bring them here.</p>
      )}

      <label className="check">
        <input type="checkbox" checked={!!settings?.syncAuto} onChange={(e) => void setAuto(e.target.checked)} disabled={syncOff} />
        Back up connection changes automatically
      </label>
      <p className="muted small">Uploads changes every 10 minutes while Mango3S is open. Restoring always asks for your password.</p>

      <div className="form-actions">
        <button className="primary" onClick={syncNow} disabled={!!busy || syncOff || state.syncing}>
          {(busy === 'sync' || state.syncing) && <Spinner small />} Sync now
        </button>
        <button onClick={() => setAction('restore')} disabled={!!busy || syncOff}>
          Restore from account…
        </button>
        <button onClick={() => setAction('change')} disabled={!!busy}>
          Change password…
        </button>
        <button onClick={() => setAction('rotate')} disabled={!!busy}>
          New recovery key…
        </button>
        <span className="spacer" />
        <button onClick={signOut} disabled={!!busy}>
          Sign out
        </button>
      </div>

      {action && (
        <form className="account-password-action" onSubmit={submitPassword} aria-label={actionLabel[action]}>
          <h3>{actionLabel[action]}</h3>
          {action === 'restore' && (
            <p className="small muted">
              Downloads your backed-up connections and adds or updates the newer ones here. Connections deleted on another computer are removed
              here too, unless you changed them since.
            </p>
          )}
          <label className="field">
            <span>{action === 'change' ? 'Current password' : 'Account password'}</span>
            <PasswordInput autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} autoFocus />
          </label>
          {action === 'change' && (
            <>
              <label className="field">
                <span>New password</span>
                <PasswordInput autoComplete="new-password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
              </label>
              <PasswordHints password={newPassword} email={state.email} />
            </>
          )}
          <div className="form-actions">
            <button
              type="submit"
              className={action === 'wipe' || action === 'delete' ? 'danger' : 'primary'}
              disabled={!!busy || !password || (action === 'change' && passwordProblems(newPassword, state.email).length > 0)}
            >
              {busy && <Spinner small />} {actionLabel[action]}
            </button>
            <button type="button" onClick={() => setAction(null)} disabled={!!busy}>
              Cancel
            </button>
          </div>
        </form>
      )}

      <details className="account-danger">
        <summary>Erase or delete</summary>
        <div className="form-actions">
          <button className="danger" onClick={() => setAction('wipe')} disabled={!!busy}>
            Erase synced data…
          </button>
          <button className="danger" onClick={() => setAction('delete')} disabled={!!busy}>
            Delete account…
          </button>
        </div>
      </details>
    </div>
  )
}
