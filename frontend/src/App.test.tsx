import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const profile = {
  id: 'p1',
  name: 'Local dev',
  uri: 'mongodb://localhost:27017',
  connectTimeoutSeconds: 10,
  readOnly: false,
  hasPassword: false,
  createdAt: '',
  updatedAt: '',
}
const signedOut = {
  signedIn: false, email: '', account: null, disabled: {}, syncing: false, lastSyncAt: '', lastSyncError: '',
  pendingChanges: 0, restoreSuggested: false, privacyPending: false,
}
const session = { id: 'p1', name: 'Local dev', readOnly: false, serverVersion: '8.0.0', topology: 'standalone', connectedAt: '' }

const backend = vi.hoisted(() => ({
  getAppInfo: vi.fn(),
  getSettings: vi.fn(),
  saveSettings: vi.fn(),
  listProfiles: vi.fn(),
  saveProfile: vi.fn(),
  reorderProfiles: vi.fn(),
  setProfileFolder: vi.fn(),
  deleteProfile: vi.fn(),
  testConnection: vi.fn(),
  connect: vi.fn(),
  disconnect: vi.fn(),
  listConnections: vi.fn(),
  listDatabases: vi.fn(),
  listCollections: vi.fn(),
  collectionStats: vi.fn(),
  sampleFields: vi.fn(),
  runQuery: vi.fn(),
  countQuery: vi.fn(),
  cancel: vi.fn(),
  listJobs: vi.fn(),
  cancelJob: vi.fn(),
  clearFinishedJobs: vi.fn(),
  listSaved: vi.fn(),
  listHistory: vi.fn(),
  getAIStatus: vi.fn(),
  getMCPInfo: vi.fn(),
  listMaskRuleSets: vi.fn(),
  listBackups: vi.fn(),
  recordCollectionOpened: vi.fn(),
  listCollectionHistory: vi.fn(),
  connectionCapabilities: vi.fn(),
  buildConnectionString: vi.fn(),
  setUnsavedWork: vi.fn(),
  getField: vi.fn(),
  updateField: vi.fn(),
  parseCollectionQuery: vi.fn(),
  connectWithLog: vi.fn(),
  testConnectionWithLog: vi.fn(),
  detectAIProviders: vi.fn(),
  listLocalModels: vi.fn(),
  testAI: vi.fn(),
  chooseFile: vi.fn(),
  runShellPage: vi.fn(),
  getUpdateStatus: vi.fn(),
  checkForUpdatesNow: vi.fn(),
  applyPendingUpdate: vi.fn(),
  fetchNotifications: vi.fn(),
  ackNotification: vi.fn(),
  respondNotification: vi.fn(),
  openPlatformLink: vi.fn(),
  getPlatformInfo: vi.fn(),
  getPrivacyPolicy: vi.fn(),
  acceptPrivacyPolicy: vi.fn(),
  getAccountState: vi.fn(),
  refreshAccount: vi.fn(),
  accountStartSignUp: vi.fn(),
  accountFinishSignUp: vi.fn(),
  accountSignIn: vi.fn(),
  accountSignOut: vi.fn(),
  accountChangePassword: vi.fn(),
  accountForgotPassword: vi.fn(),
  accountResetPassword: vi.fn(),
  accountRotateRecoveryKey: vi.fn(),
  accountDelete: vi.fn(),
  syncNow: vi.fn(),
  syncRestore: vi.fn(),
  syncWipe: vi.fn(),
}))

const jobListeners = vi.hoisted(() => [] as ((j: unknown) => void)[])
const stepListeners = vi.hoisted(() => [] as ((e: unknown) => void)[])
const menuListeners = vi.hoisted(() => ({}) as Record<string, (() => void)[]>)
const updateStatusListeners = vi.hoisted(() => [] as ((s: unknown) => void)[])
const notificationListeners = vi.hoisted(() => [] as ((n: unknown) => void)[])
const privacyListeners = vi.hoisted(() => [] as ((p: unknown) => void)[])
const accountListeners = vi.hoisted(() => [] as ((s: unknown) => void)[])
const quitAppMock = vi.hoisted(() => vi.fn())

vi.mock('./lib/backend', () => ({
  backend,
  newOpId: (p: string) => `${p}-1`,
  onMenu: (name: string, cb: () => void) => {
    ;(menuListeners[name] ??= []).push(cb)
    return () => undefined
  },
  onConnectionStep: (cb: (e: unknown) => void) => {
    stepListeners.push(cb)
    return () => undefined
  },
  onJobUpdate: (cb: (j: unknown) => void) => {
    jobListeners.push(cb)
    return () => undefined
  },
  onUpdateStatus: (cb: (s: unknown) => void) => {
    updateStatusListeners.push(cb)
    return () => undefined
  },
  onPrivacyRequired: (cb: (p: unknown) => void) => {
    privacyListeners.push(cb)
    return () => undefined
  },
  onAccountChanged: (cb: (s: unknown) => void) => {
    accountListeners.push(cb)
    return () => undefined
  },
  onProfilesChanged: () => () => undefined,
  onNewNotification: (cb: (n: unknown) => void) => {
    notificationListeners.push(cb)
    return () => undefined
  },
  quitApp: quitAppMock,
}))

import App from './App'

/** The field name (or array index) shown in a tree row's key cell. */
const keyOf = (r: HTMLElement) => r.querySelector('.tree-field, .tree-index')?.textContent ?? ''
import { setShortcutPlatform } from './state/shortcuts'

beforeEach(() => {
  vi.clearAllMocks()
  backend.getAppInfo.mockResolvedValue({
    version: '1.2.3', configDir: '/Users/me/Library/Application Support/Mango3S', logFile: '/Users/me/Library/Application Support/Mango3S/logs/mango3s.log',
    secretBackend: 'keyring', commit: 'abcdef1234567890', buildTime: '2026-10-18T10:00:00Z', modified: false, goVersion: 'go1.26.0',
    platform: 'darwin/arm64', executable: '/Applications/Mango3S.app/Contents/MacOS/mango3s', backupDir: '/Users/me/Library/Application Support/Mango3S/backups',
    components: { Wails: 'v2.16.0', 'MongoDB Go driver': 'v2.9.2' },
  })
  for (const k of Object.keys(menuListeners)) delete menuListeners[k]
  backend.getSettings.mockResolvedValue({
    theme: 'dark', pageSize: 50, queryTimeoutSeconds: 60, confirmDeletes: true, defaultView: 'tree', editorFontSize: 13, logLevel: 'info',
    aiEnabled: true, aiProvider: 'anthropic', aiModel: 'claude-opus-5-5', aiTimeoutSeconds: 180, claudeCodePath: '', claudeCodeModel: '', codexPath: '', codexModel: '',
    localUrl: 'http://localhost:11434', localApi: 'ollama', localModel: '', aiAllowSamples: false, schedulerEnabled: true,
    autoUpdateEnabled: true, syncAuto: true,
  })
  backend.getUpdateStatus.mockResolvedValue({ connected: false, revoked: false, state: 'idle', currentVersion: '1.2.3' })
  backend.fetchNotifications.mockResolvedValue([])
  backend.ackNotification.mockResolvedValue(undefined)
  backend.respondNotification.mockResolvedValue(undefined)
  backend.openPlatformLink.mockResolvedValue(undefined)
  backend.getPlatformInfo.mockResolvedValue(null)
  backend.getPrivacyPolicy.mockResolvedValue(null)
  backend.getAccountState.mockResolvedValue(signedOut)
  backend.refreshAccount.mockResolvedValue(signedOut)
  backend.checkForUpdatesNow.mockResolvedValue({ connected: true, revoked: false, state: 'up_to_date', currentVersion: '1.2.3' })
  backend.applyPendingUpdate.mockResolvedValue(false)
  backend.listProfiles.mockResolvedValue([profile])
  backend.listConnections.mockResolvedValue([])
  backend.connect.mockResolvedValue(session)
  backend.listDatabases.mockResolvedValue([{ name: 'shop', sizeOnDisk: 4096, empty: false }])
  backend.listCollections.mockResolvedValue([{ name: 'orders', type: 'collection', readOnly: false }])
  backend.sampleFields.mockResolvedValue(['_id', 'status'])
  backend.runQuery.mockResolvedValue({
    documents: [{ id: '1', text: '{\n  "_id": 1,\n  "status": "A"\n}', json: '{"_id":1,"status":"A"}' }],
    page: 0, pageSize: 50, hasMore: false, durationMs: 3,
  })
  backend.cancel.mockResolvedValue(true)
  backend.listJobs.mockResolvedValue([])
  backend.cancelJob.mockResolvedValue(true)
  backend.listSaved.mockResolvedValue([
    { id: 's1', kind: 'sql', name: 'Big orders', description: '', db: 'shop', coll: '', text: 'SELECT * FROM orders', createdAt: '', updatedAt: new Date().toISOString() },
  ])
  backend.listHistory.mockResolvedValue([])
  backend.getAIStatus.mockResolvedValue({ enabled: false, ready: false, provider: 'anthropic', providerName: 'Anthropic API', model: 'claude-opus-5-5', hasKey: false, keySource: 'none', allowSamples: false, leavesDevice: true })
  backend.detectAIProviders.mockResolvedValue({
    claudeCode: { name: 'Claude Code', found: true, path: '/Users/me/.local/bin/claude', version: '2.1.220', signedIn: true, account: 'claude.ai (pro)', message: '' },
    codex: { name: 'Codex', found: false, path: '', version: '', signedIn: null, account: '', message: 'Not installed. Install the Codex CLI.' },
    localServers: [{ name: 'Ollama', url: 'http://localhost:11434', api: 'ollama', models: ['qwen2.5-coder:7b', 'llama3.2'] }],
  })
  backend.listLocalModels.mockResolvedValue(['qwen2.5-coder:7b', 'llama3.2'])
  backend.getMCPInfo.mockResolvedValue({
    policy: { enabled: false, profiles: [], databases: [], tools: [], allowWrites: false, maxDocs: 50, maxWriteDocs: 100, maskRuleSetId: '', auditLog: true },
    catalog: [{ name: 'find', description: 'Find documents', write: false }],
    snippet: '{}',
  })
  backend.listMaskRuleSets.mockResolvedValue([])
  backend.listBackups.mockResolvedValue([])
  backend.recordCollectionOpened.mockResolvedValue(undefined)
  backend.listCollectionHistory.mockResolvedValue([])
  backend.setUnsavedWork.mockResolvedValue(undefined)
  backend.buildConnectionString.mockImplementation(async (s: { hosts: { host: string; port: number }[] }) => `mongodb://${s.hosts.map((h) => `${h.host}:${h.port}`).join(',')}/`)
  backend.connectionCapabilities.mockResolvedValue({
    authMechanisms: [
      { id: '', label: 'Default (SCRAM negotiated)', supported: true },
      { id: 'SCRAM-SHA-256', label: 'SCRAM-SHA-256', supported: true },
      { id: 'GSSAPI', label: 'Kerberos (GSSAPI)', supported: false, note: 'Not available in this build.' },
    ],
    sshAuth: [
      { id: 'password', label: 'Password', supported: true },
      { id: 'privateKey', label: 'Private key', supported: true },
    ],
    proxy: [
      { id: 'none', label: 'None', supported: true },
      { id: 'socks5', label: 'SOCKS5', supported: true },
    ],
    advanced: [{ id: 'socketTimeout', label: 'Socket timeout', supported: false, note: 'Removed in Go driver v2.' }],
  })
  setShortcutPlatform('other')
  jobListeners.length = 0
  updateStatusListeners.length = 0
  notificationListeners.length = 0
  privacyListeners.length = 0
  accountListeners.length = 0
  // Connect goes through the logging variant; keep the connect mock as the source of truth.
  backend.connectWithLog.mockImplementation((_op: string, id: string) => backend.connect(id))
  backend.testConnectionWithLog.mockImplementation((_op: string, p: unknown) => backend.testConnection(p))
})

describe('App', () => {
  it('connects, browses and opens a collection', async () => {
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Welcome to Mango3S' })).toBeInTheDocument()

    const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
    fireEvent.click(await within(sidebar).findByRole('button', { name: 'Connect' }))
    await waitFor(() => expect(backend.connect).toHaveBeenCalledWith('p1'))
    expect(await screen.findByText('Connected to Local dev')).toBeInTheDocument()
    // The connection gets its own workspace tab; Welcome stays first.
    const tabNames = () => screen.getAllByRole('tab').map((t) => t.textContent)
    expect(tabNames()[0]).toMatch(/^Welcome/)
    expect(await screen.findByRole('tab', { name: /Local dev/ })).toHaveAttribute('aria-selected', 'true')

    fireEvent.click(await within(sidebar).findByText('shop'))
    fireEvent.click(await within(sidebar).findByText('orders'))

    expect(await screen.findByRole('tab', { name: /orders/ })).toHaveAttribute('aria-selected', 'true')
    await waitFor(() => expect(backend.runQuery).toHaveBeenCalled())
    const req = backend.runQuery.mock.calls[0][1]
    expect(req).toMatchObject({ connId: 'p1', db: 'shop', coll: 'orders', filter: '{}', page: 0, pageSize: 50 })
    // Tree view is the default: field, value and BSON type per row.
    const tree = await screen.findByRole('treegrid')
    const status = (await within(tree).findAllByRole('row')).find((r) => keyOf(r) === 'status')
    expect(status).toHaveTextContent('A')
    expect(backend.recordCollectionOpened).toHaveBeenCalledWith('p1', 'shop', 'orders')
  })

  it('opens the connection manager and settings', async () => {
    render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'New connection' }))
    expect(await screen.findByRole('dialog', { name: 'Connection Manager' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))

    fireEvent.click(within(screen.getByRole('tablist', { name: '' })).getByRole('button', { name: 'Settings' }))
    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument()
    expect(document.documentElement.dataset.theme).toBe('dark')
  })

  it('shows background job progress and lets the user cancel', async () => {
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })
    const job = {
      id: 'j1', kind: 'export', title: 'Export shop.orders → orders.json', state: 'running', done: 250, total: 1000,
      unit: 'documents', message: '', createdAt: '2026-01-01T00:00:00Z', startedAt: '2026-01-01T00:00:00Z', finishedAt: '0001-01-01T00:00:00Z',
    }
    act(() => jobListeners.forEach((l) => l(job)))
    const indicator = await screen.findByRole('button', { name: /1 job running · 25%/ })
    fireEvent.click(indicator)
    expect(await screen.findByText('Export shop.orders → orders.json')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '25')
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(backend.cancelJob).toHaveBeenCalledWith('j1')

    act(() => jobListeners.forEach((l) => l({ ...job, state: 'succeeded', done: 1000, result: '1000 documents written' })))
    expect(await screen.findByText('1000 documents written')).toBeInTheDocument()
  })

  it('opens the library with saved items', async () => {
    render(<App />)
    fireEvent.click(await screen.findByTitle(/Saved queries and history/))
    expect(await screen.findByText('Big orders')).toBeInTheDocument()
    expect(backend.listSaved).toHaveBeenCalled()
  })
})

describe('platform updates', () => {
  it('auto-opens the restart dialog once an update is ready, and installs it', async () => {
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })

    act(() =>
      updateStatusListeners.forEach((l) =>
        l({ connected: true, revoked: false, state: 'ready_to_install', currentVersion: '1.0.0', latestVersion: '1.1.0' }),
      ),
    )
    const dialog = await screen.findByRole('dialog', { name: 'Update ready to install' })
    expect(within(dialog).getByText('1.1.0')).toBeInTheDocument()

    backend.applyPendingUpdate.mockResolvedValue(true)
    fireEvent.click(screen.getByRole('button', { name: /Restart & install/ }))
    await waitFor(() => expect(backend.applyPendingUpdate).toHaveBeenCalled())
    await waitFor(() => expect(quitAppMock).toHaveBeenCalled())
  })

  it('shows the status-bar indicator when revoked, with an explanatory dialog', async () => {
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })
    act(() => updateStatusListeners.forEach((l) => l({ connected: true, revoked: true, state: 'error', currentVersion: '1.0.0' })))

    const indicator = await screen.findByRole('button', { name: 'Updates paused' })
    fireEvent.click(indicator)
    expect(await screen.findByRole('dialog', { name: 'Updates paused' })).toBeInTheDocument()
  })

  it('queues and dismisses platform notifications one at a time, acknowledging each', async () => {
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })

    act(() => notificationListeners.forEach((l) => l({ id: 'n1', title: 'Maintenance', body: 'Server work tonight' })))
    expect(await screen.findByRole('dialog', { name: 'Maintenance' })).toBeInTheDocument()

    act(() => notificationListeners.forEach((l) => l({ id: 'n2', title: 'New release', body: 'Check it out' })))
    // The second one queues behind the first rather than replacing it.
    expect(screen.getByRole('dialog', { name: 'Maintenance' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Dismiss/ }))
    expect(backend.respondNotification).toHaveBeenCalledWith('n1', 'close')
    expect(await screen.findByRole('dialog', { name: 'New release' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Dismiss/ }))
    expect(backend.respondNotification).toHaveBeenCalledWith('n2', 'close')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New release' })).toBeNull())
  })

  it('shows notification buttons, opens links and reports which one was pressed', async () => {
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })
    const buttons = [
      { id: 'b1', label: 'Take the survey', action: 'open_link', url: 'https://example.com/survey', style: 'primary' },
      { id: 'later', label: 'Not now', action: 'cancel', url: null, style: 'secondary' },
    ]
    act(() => notificationListeners.forEach((l) => l({ id: 'n1', title: 'Quick survey', body: 'Two minutes', buttons })))
    act(() => notificationListeners.forEach((l) => l({ id: 'n2', title: 'Terms', body: 'Agree?', buttons: [{ id: 'yes', label: 'I agree', action: 'accept', url: null, style: 'primary' }] })))
    const dlg = await screen.findByRole('dialog', { name: 'Quick survey' })
    expect(within(dlg).getByRole('button', { name: /Take the survey/ })).toHaveClass('primary')
    expect(within(dlg).getByRole('button', { name: 'Close' })).toBeInTheDocument() // the X is always there

    fireEvent.click(within(dlg).getByRole('button', { name: /Take the survey/ }))
    expect(backend.openPlatformLink).toHaveBeenCalledWith('https://example.com/survey')
    expect(backend.respondNotification).toHaveBeenCalledWith('n1', 'b1')

    const next = await screen.findByRole('dialog', { name: 'Terms' })
    fireEvent.click(within(next).getByRole('button', { name: 'I agree' }))
    expect(backend.respondNotification).toHaveBeenCalledWith('n2', 'yes')
    expect(backend.openPlatformLink).toHaveBeenCalledTimes(1)
  })

  it('asks to accept a new privacy policy, and Later leaves the app usable', async () => {
    backend.acceptPrivacyPolicy.mockResolvedValue({ version: 2, title: 'Privacy policy', body: '', summary: null, publishedAt: null, url: '', acceptanceRequired: false, acceptedAt: 'now' })
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })
    const policy = { version: 2, title: 'Privacy policy', body: '## Who we are\nWe make **Mango3S**.\n\n- one\n- two', summary: 'We now name our email provider.', publishedAt: null, url: 'https://mango3s.com/privacy', acceptanceRequired: true, acceptedAt: null }
    act(() => privacyListeners.forEach((l) => l(policy)))
    let dlg = await screen.findByRole('dialog', { name: 'Privacy policy updated' })
    expect(within(dlg).getByText('We now name our email provider.')).toBeInTheDocument()
    fireEvent.click(within(dlg).getByRole('button', { name: 'Read the full policy' }))
    const text = within(dlg).getByLabelText('Privacy policy text')
    expect(within(text).getByRole('heading', { name: 'Who we are' })).toBeInTheDocument()
    expect(within(text).getByText('Mango3S').tagName).toBe('STRONG')
    expect(within(text).getAllByRole('listitem')).toHaveLength(2)

    fireEvent.click(within(dlg).getByRole('button', { name: 'Later' }))
    expect(screen.queryByRole('dialog', { name: 'Privacy policy updated' })).toBeNull()

    act(() => privacyListeners.forEach((l) => l(policy)))
    dlg = await screen.findByRole('dialog', { name: 'Privacy policy updated' })
    fireEvent.click(within(dlg).getByRole('button', { name: /Accept/ }))
    await waitFor(() => expect(backend.acceptPrivacyPolicy).toHaveBeenCalledWith(2))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Privacy policy updated' })).toBeNull())
  })

  it('has no activation step: Settings offers only the auto-update toggle and Check now', async () => {
    render(<App />)
    fireEvent.click(within(screen.getByRole('tablist', { name: '' })).getByRole('button', { name: 'Settings' }))
    await screen.findByRole('heading', { name: 'Settings' })
    fireEvent.click(screen.getByRole('tab', { name: 'Updates' }))

    const section = document.getElementById('settings-platform')!
    expect(within(section).queryByRole('textbox')).toBeNull()
    expect(within(section).queryByLabelText(/activation key/i)).toBeNull()
    expect(within(section).getByRole('checkbox', { name: /Automatically check for and download updates/ })).toBeChecked()

    fireEvent.click(within(section).getByRole('button', { name: /Check for updates now/ }))
    await waitFor(() => expect(backend.checkForUpdatesNow).toHaveBeenCalled())
    expect(await screen.findByText('You are up to date')).toBeInTheDocument()
  })

  it('stays fully usable when the update server is unreachable', async () => {
    backend.getUpdateStatus.mockRejectedValue(new Error('offline'))
    backend.fetchNotifications.mockRejectedValue(new Error('offline'))
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Welcome to Mango3S' })).toBeInTheDocument()
    // Nothing about the outage is pushed at the user.
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.queryByRole('button', { name: /Update/ })).toBeNull()
  })
})

const key = (init: KeyboardEventInit & { code?: string }, target: EventTarget = document.body) =>
  act(() => {
    target.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }))
  })

/** A successful connect shows the connection log briefly; close it. */
async function dismissConnectLog() {
  const dlg = await screen.findByRole('dialog', { name: /Connecting to/ })
  await within(dlg).findByText(/Connected to/)
  fireEvent.click(within(dlg).getAllByRole('button', { name: 'Close' }).at(-1)!)
  await waitFor(() => expect(screen.queryByRole('dialog', { name: /Connecting to/ })).toBeNull())
}

async function connectAndOpenOrders() {
  render(<App />)
  const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
  fireEvent.click(await within(sidebar).findByRole('button', { name: 'Connect' }))
  await dismissConnectLog()
  fireEvent.click(await within(sidebar).findByText('shop'))
  fireEvent.click(await within(sidebar).findByText('orders'))
  await screen.findByRole('treegrid')
}

describe('Welcome tab', () => {
  it('is the first, default tab and offers the main actions', async () => {
    render(<App />)
    const tabs = await screen.findAllByRole('tab')
    expect(tabs[0]).toHaveTextContent('Welcome')
    expect(tabs[0]).toHaveAttribute('aria-selected', 'true')
    for (const name of ['New connection', 'Open saved connections…', 'Keyboard shortcuts']) {
      expect(screen.getAllByRole('button', { name }).length).toBeGreaterThan(0)
    }
    expect(await screen.findByRole('list', { name: 'Connections' })).toHaveTextContent('Local dev')
    fireEvent.click(screen.getAllByRole('button', { name: 'Keyboard shortcuts' })[0])
    const dialog = await screen.findByRole('dialog', { name: 'Keyboard shortcuts' })
    expect(within(dialog).getByText('Open the connection manager')).toBeInTheDocument()
  })

  it('comes back first when reopened after closing', async () => {
    render(<App />)
    const welcome = (await screen.findAllByRole('tab'))[0]
    fireEvent.click(within(welcome).getByRole('button', { name: /Close Welcome/ }))
    await waitFor(() => expect(screen.queryByRole('tab', { name: /Welcome/ })).toBeNull())
    key({ key: 'F1', shiftKey: true })
    expect((await screen.findAllByRole('tab'))[0]).toHaveTextContent('Welcome')
  })
})

describe('keyboard shortcuts', () => {
  it('open the connection manager and the shortcut reference, and switch tabs', async () => {
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })
    key({ key: 'n', code: 'KeyN', ctrlKey: true })
    expect(await screen.findByRole('dialog', { name: 'Connection Manager' })).toBeInTheDocument()
    key({ key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

    key({ key: 'F1' })
    expect(await screen.findByRole('dialog', { name: 'Keyboard shortcuts' })).toBeInTheDocument()
    key({ key: 'Escape' })

    key({ key: 'p', code: 'KeyP', ctrlKey: true }) // settings
    expect(await screen.findByRole('tab', { name: /Settings/ })).toHaveAttribute('aria-selected', 'true')
    key({ key: 'PageUp', ctrlKey: true })
    expect(screen.getByRole('tab', { name: /Welcome/ })).toHaveAttribute('aria-selected', 'true')
    key({ key: 'PageDown', ctrlKey: true })
    expect(screen.getByRole('tab', { name: /Settings/ })).toHaveAttribute('aria-selected', 'true')
    key({ key: 'w', code: 'KeyW', ctrlKey: true })
    await waitFor(() => expect(screen.queryByRole('tab', { name: /Settings/ })).toBeNull())
  })

  it('do not fire while typing plain keys in a field', async () => {
    render(<App />)
    const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
    fireEvent.click(await within(sidebar).findByRole('button', { name: 'Connect' }))
    await dismissConnectLog()
    const filter = await within(sidebar).findByRole('textbox', { name: 'Filter collections' })
    filter.focus()
    key({ key: '1', code: 'Digit1', altKey: true }, filter) // quick-connect, but the user is typing
    key({ key: 'Delete' }, filter)
    expect(backend.connect).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('navigate the explorer with arrows and open a collection with Enter', async () => {
    render(<App />)
    const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
    fireEvent.click(await within(sidebar).findByRole('button', { name: 'Connect' }))
    await within(sidebar).findByText('shop')
    const tree = within(sidebar).getByRole('tree')
    tree.focus()
    key({ key: 'Home' }, tree) // the connection
    key({ key: 'ArrowDown' }, tree) // → shop
    key({ key: 'ArrowRight' }, tree) // expand shop
    await within(sidebar).findByText('orders')
    expect(within(sidebar).getByText('Collections')).toBeInTheDocument() // grouped into folders
    key({ key: 'ArrowDown' }, tree) // Collections folder
    key({ key: 'ArrowDown' }, tree) // orders
    key({ key: 'Enter' }, tree)
    expect(await screen.findByRole('tab', { name: /orders/ })).toHaveAttribute('aria-selected', 'true')
  })
})

describe('unsaved work', () => {
  it('asks before closing a tab with an unsaved field edit', async () => {
    backend.getField.mockResolvedValue({ exists: true, type: 'string', text: 'A', expected: '"A"', editable: true })
    await connectAndOpenOrders()
    const tree = screen.getByRole('treegrid')
    const row = (await within(tree).findAllByRole('row')).find((r) => keyOf(r) === 'status')!
    fireEvent.doubleClick(row)
    const input = await screen.findByRole('textbox', { name: 'Edit status' })
    fireEvent.change(input, { target: { value: 'B' } })
    await waitFor(() => expect(screen.getByRole('tab', { name: /orders/ })).toHaveClass('dirty'))
    expect(backend.setUnsavedWork).toHaveBeenLastCalledWith(true)

    key({ key: 'w', code: 'KeyW', ctrlKey: true })
    const confirmDlg = await screen.findByRole('dialog', { name: /Discard changes/ })
    fireEvent.click(within(confirmDlg).getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('tab', { name: /orders/ })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Edit status' })).toHaveValue('B')

    fireEvent.click(within(screen.getByRole('tab', { name: /orders/ })).getByRole('button', { name: /Close/ }))
    fireEvent.click(within(await screen.findByRole('dialog', { name: /Discard changes/ })).getByRole('button', { name: 'Discard' }))
    await waitFor(() => expect(screen.queryByRole('tab', { name: /orders/ })).toBeNull())
    expect(backend.updateField).not.toHaveBeenCalled()
  })
})

describe('connection manager', () => {
  it('has the nine tabs and keeps values when switching between them', async () => {
    backend.saveProfile.mockImplementation(async (p: { name: string }) => ({ ...profile, ...p, id: 'p2' }))
    render(<App />)
    await screen.findByText('Local dev', { selector: '.welcome-name' })
    fireEvent.click(within(screen.getByRole('tabpanel')).getByRole('button', { name: 'New connection' }))
    const dlg = await screen.findByRole('dialog', { name: 'Connection Manager' })
    const tabs = within(within(dlg).getByRole('tablist', { name: 'Connection settings' })).getAllByRole('tab')
    expect(tabs.map((t) => t.textContent)).toEqual([
      'Server', 'Authentication', 'SSL / TLS', 'SSH', 'Proxy', 'Shell', 'MongoDB Tools', 'Advanced', 'Collection History',
    ])
    fireEvent.change(within(dlg).getByLabelText('Connection name'), { target: { value: 'Prod via bastion' } })
    fireEvent.change(within(dlg).getByLabelText('Host 1'), { target: { value: 'db.internal' } })

    fireEvent.click(within(dlg).getByRole('tab', { name: 'SSH' }))
    fireEvent.click(within(dlg).getByLabelText('Connect through an SSH tunnel'))
    fireEvent.change(within(dlg).getByLabelText('SSH host'), { target: { value: 'bastion.example.com' } })
    fireEvent.change(within(dlg).getByLabelText('SSH username'), { target: { value: 'ops' } })
    fireEvent.change(within(dlg).getByLabelText('SSH password'), { target: { value: 's3cret' } })

    fireEvent.click(within(dlg).getByRole('tab', { name: 'Authentication' }))
    const mech = within(dlg).getByLabelText('Authentication method') as HTMLSelectElement
    const kerberos = Array.from(mech.options).find((o) => o.value === 'GSSAPI')!
    expect(kerberos.disabled).toBe(true) // unsupported options are shown as such
    expect(kerberos.textContent).toContain('not supported')

    fireEvent.click(within(dlg).getByRole('tab', { name: 'Advanced' }))
    fireEvent.change(within(dlg).getByLabelText('Read preference'), { target: { value: 'secondaryPreferred' } })
    expect(within(dlg).getByText(/Socket timeout/)).toBeInTheDocument()

    fireEvent.click(within(dlg).getByRole('tab', { name: 'Server' }))
    expect(within(dlg).getByLabelText('Connection name')).toHaveValue('Prod via bastion')
    expect(within(dlg).getByLabelText('Host 1')).toHaveValue('db.internal')
    fireEvent.click(within(dlg).getByRole('tab', { name: 'SSH enabled' })) // the tab shows that SSH is on
    expect(within(dlg).getByLabelText('SSH host')).toHaveValue('bastion.example.com')

    fireEvent.click(within(dlg).getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(backend.saveProfile).toHaveBeenCalled())
    const saved = backend.saveProfile.mock.calls[0][0]
    expect(saved).toMatchObject({
      name: 'Prod via bastion',
      server: { mode: 'manual', type: 'standalone', hosts: [{ host: 'db.internal', port: 27017 }] },
      ssh: { enabled: true, host: 'bastion.example.com', username: 'ops', port: 22, authMethod: 'password' },
      advanced: { readPreference: 'secondaryPreferred' },
      secrets: { sshPassword: { value: 's3cret', clear: false } },
    })
  })
})

describe('collection query editor', () => {
  it('shows the query as one statement and runs it with paging', async () => {
    await connectAndOpenOrders()
    const editor = screen.getByRole('textbox', { name: 'Query' })
    expect(editor.textContent).toBe('db.getCollection("orders").find({})')
    backend.parseCollectionQuery.mockResolvedValue({
      kind: 'find', db: 'shop', coll: 'orders', filter: '{ status: "A" }', projection: '', sort: '{ _id: -1 }', skip: 0, limit: 0, stages: null,
    })
    backend.runQuery.mockClear()
    fireEvent.click(screen.getByRole('button', { name: /Run/ }))
    await waitFor(() => expect(backend.runQuery).toHaveBeenCalled())
    expect(backend.parseCollectionQuery).toHaveBeenCalledWith('db.getCollection("orders").find({})', 'shop')
    expect(backend.runQuery.mock.calls[0][1]).toMatchObject({ coll: 'orders', filter: '{ status: "A" }', sort: '{ _id: -1 }', page: 0, pageSize: 50 })
  })

  it('opens aggregate() in the aggregation editor and other collections in their own tab', async () => {
    await connectAndOpenOrders()
    backend.parseCollectionQuery.mockResolvedValueOnce({
      kind: 'aggregate', db: 'shop', coll: 'orders', filter: '', projection: '', sort: '', skip: 0, limit: 0,
      stages: [{ operator: '$match', body: '{}', enabled: true }],
    })
    backend.runQuery.mockClear()
    fireEvent.click(screen.getByRole('button', { name: /Run/ }))
    expect(await screen.findByRole('tab', { name: /orders · pipeline/ })).toHaveAttribute('aria-selected', 'true')
    expect(backend.runQuery).not.toHaveBeenCalled()
  })

  it('shows parse errors next to the editor without running anything', async () => {
    await connectAndOpenOrders()
    backend.parseCollectionQuery.mockRejectedValueOnce(new Error(JSON.stringify({ code: 'INVALID_INPUT', message: 'Only find() and aggregate() run in the collection tab.' })))
    backend.runQuery.mockClear()
    fireEvent.click(screen.getByRole('button', { name: /Run/ }))
    expect(await screen.findByText('Only find() and aggregate() run in the collection tab.')).toBeInTheDocument()
    expect(backend.runQuery).not.toHaveBeenCalled()
  })

  it('switches to separate fields with the parsed parts', async () => {
    await connectAndOpenOrders()
    backend.parseCollectionQuery.mockResolvedValue({
      kind: 'find', db: 'shop', coll: 'orders', filter: '{ qty: 5 }', projection: '', sort: '', skip: 0, limit: 7, stages: null,
    })
    fireEvent.click(screen.getByRole('radio', { name: 'Fields' }))
    expect(await screen.findByRole('textbox', { name: 'Filter' })).toHaveTextContent('{ qty: 5 }')
    expect(screen.getByTitle('0 = no limit')).toHaveValue(7)
    fireEvent.click(screen.getByRole('radio', { name: 'Query' }))
    expect(screen.getByRole('textbox', { name: 'Query' }).textContent).toContain('.limit(7)')
  })
})

describe('AI settings', () => {
  it('detects Claude Code, Codex and local servers and saves the chosen provider', async () => {
    backend.saveSettings.mockImplementation(async (s: unknown) => s)
    render(<App />)
    fireEvent.click(await screen.findByTitle(/Settings \(/))
    fireEvent.click(await screen.findByRole('tab', { name: 'AI assistant' }))
    const group = await screen.findByRole('radiogroup', { name: 'AI provider' })
    const card = (name: RegExp) => within(group).getByRole('radio', { name })
    await waitFor(() => expect(card(/Claude Code/)).toHaveTextContent('signed in'))
    expect(card(/Codex/)).toHaveTextContent('not installed')
    expect(card(/Local model/)).toHaveTextContent('Ollama')

    fireEvent.click(card(/Claude Code/))
    expect(screen.getByText(/2\.1\.220/)).toBeInTheDocument()
    expect(screen.getByText(/signed in with claude.ai \(pro\)/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Test/ })).toBeDisabled() // unsaved changes must be saved first

    fireEvent.click(card(/Local model/))
    fireEvent.click(await screen.findByRole('button', { name: /Ollama · 2 models/ }))
    expect(screen.getByRole('combobox', { name: 'Local model' })).toHaveValue('qwen2.5-coder:7b')
    expect(screen.queryByText(/This server is not on this computer/)).toBeNull()
    fireEvent.change(screen.getByRole('textbox', { name: 'Server URL' }), { target: { value: 'http://gpu-box.lan:11434' } })
    expect(screen.getByText(/This server is not on this computer/)).toBeInTheDocument()
    fireEvent.change(screen.getByRole('textbox', { name: 'Server URL' }), { target: { value: 'http://localhost:11434' } })

    fireEvent.click(screen.getByRole('button', { name: /Save settings/ }))
    await waitFor(() => expect(backend.saveSettings).toHaveBeenCalled())
    expect(backend.saveSettings.mock.calls[0][0]).toMatchObject({ aiProvider: 'local', localUrl: 'http://localhost:11434', localApi: 'ollama', localModel: 'qwen2.5-coder:7b' })
  })
})

describe('connection log', () => {
  const step = (opId: string, id: string, label: string, state: string, extra: Record<string, unknown> = {}) =>
    act(() => stepListeners.forEach((l) => l({ opId, step: { id, label, state, durationMs: 12, ...extra } })))

  it('shows each step while connecting and keeps a failed attempt open with the reason', async () => {
    let finish: (v: unknown) => void = () => undefined
    let fail: (e: unknown) => void = () => undefined
    backend.connectWithLog.mockImplementation(() => new Promise((res, rej) => ((finish = res), (fail = rej))))
    render(<App />)
    const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
    fireEvent.click(await within(sidebar).findByRole('button', { name: 'Connect' }))
    const dlg = await screen.findByRole('dialog', { name: 'Connecting to Local dev' })
    const opId = backend.connectWithLog.mock.calls[0][0]
    expect(backend.connectWithLog).toHaveBeenCalledWith(opId, 'p1')
    step(opId, 'settings', 'Check connection settings', 'ok', { detail: 'Hosts: localhost:27017' })
    step(opId, 'reach', 'Reach the server', 'running')
    step(opId, 'auth', 'Connect', 'pending')
    expect(within(dlg).getAllByRole('listitem').map((li) => li.textContent)).toEqual([
      expect.stringContaining('Check connection settings'), expect.stringContaining('Reach the server'), expect.stringContaining('Connect'),
    ])
    step(opId, 'reach', 'Reach the server', 'failed', { error: 'No server could be reached.', detail: '✗ localhost:27017 — connection refused' })
    step(opId, 'auth', 'Connect', 'skipped')
    await act(async () => fail(new Error(JSON.stringify({ code: 'UNREACHABLE', message: 'No server could be reached.' }))))
    expect(within(dlg).getByRole('alert')).toHaveTextContent('No server could be reached.')
    expect(within(dlg).getByText(/Failed at “Reach the server”/)).toBeInTheDocument()
    expect(within(dlg).getByText(/connection refused/)).toBeInTheDocument()
    // Copy log gives the whole log as text.
    const write = vi.fn()
    Object.assign(navigator, { clipboard: { writeText: write } })
    fireEvent.click(within(dlg).getByRole('button', { name: 'Copy log' }))
    expect(write.mock.calls[0][0]).toMatch(/\[OK\] Check connection settings[\s\S]*\[FAILED\] Reach the server[\s\S]*error: No server could be reached\.[\s\S]*\[SKIPPED\] Connect/)
    // A failed attempt stays open.
    await new Promise((r) => setTimeout(r, 1700))
    expect(screen.getByRole('dialog', { name: 'Connecting to Local dev' })).toBeInTheDocument()
    fireEvent.click(within(dlg).getAllByRole('button', { name: 'Close' }).at(-1)!)
    expect(screen.queryByRole('dialog', { name: /Connecting/ })).toBeNull()
    void finish
  })

  it('closes by itself after a successful connect unless kept open', async () => {
    backend.connectWithLog.mockImplementation(async () => session)
    render(<App />)
    const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
    fireEvent.click(await within(sidebar).findByRole('button', { name: 'Connect' }))
    const dlg = await screen.findByRole('dialog', { name: 'Connecting to Local dev' })
    expect(await within(dlg).findByText(/Connected to Local dev/)).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: /Connecting/ })).toBeNull(), { timeout: 3000 })
  })

  it('Test in the connection manager shows the log too', async () => {
    backend.testConnection.mockResolvedValue({ serverVersion: '8.0.0', topology: 'standalone', latencyMs: 3 })
    render(<App />)
    await screen.findByText('Local dev', { selector: '.welcome-name' })
    fireEvent.click(within(screen.getByRole('tabpanel')).getByRole('button', { name: 'New connection' }))
    const mgr = await screen.findByRole('dialog', { name: 'Connection Manager' })
    fireEvent.change(within(mgr).getByLabelText('Connection name'), { target: { value: 'Dev' } })
    fireEvent.click(within(mgr).getByRole('button', { name: /Test connection/ }))
    const log = await screen.findByRole('dialog', { name: 'Testing Dev' })
    expect(await within(log).findByText(/Connection OK · MongoDB 8.0.0/)).toBeInTheDocument()
    // A test result stays until closed.
    await new Promise((r) => setTimeout(r, 1700))
    expect(screen.getByRole('dialog', { name: 'Testing Dev' })).toBeInTheDocument()
  })
})

describe('app menu', () => {
  const menu = (name: string) => act(() => (menuListeners[name] ?? []).forEach((f) => f()))

  it('Settings… (⌘,) opens Settings, and Keymap… opens it at the keymap', async () => {
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })
    menu('settings')
    expect(await screen.findByRole('tab', { name: /^Settings/ })).toHaveAttribute('aria-selected', 'true')
    expect(await screen.findByRole('tab', { name: 'General' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.queryByRole('heading', { name: 'Keymap' })).toBeNull() // one section at a time
    menu('welcome')
    menu('keymap')
    expect(await screen.findByRole('heading', { name: 'Keymap' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Keymap' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: /^Settings/ })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getAllByRole('tab', { name: /^Settings/ })).toHaveLength(1) // reused, not duplicated
  })

  it('About Mango3S shows the version and the license', async () => {
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })
    menu('about')
    const dlg = await screen.findByRole('dialog', { name: 'About Mango3S' })
    expect(dlg.textContent).toMatch(/Version 1\.2\.3 \(build abcdef1/)
    expect(within(dlg).getByText(/Copyright © 2026 saman/)).toBeInTheDocument()
    expect(within(dlg).getByText(/ABSOLUTELY NO\s+WARRANTY/)).toBeInTheDocument()
    expect(within(dlg).queryByText(/Technical details/)).toBeNull()
    fireEvent.click(within(dlg).getByRole('tab', { name: 'License' }))
    expect(within(dlg).getByRole('tab', { name: 'License' })).toHaveAttribute('aria-selected', 'true')
    const text = within(dlg).getByLabelText('License text')
    expect(text.textContent).toContain('GNU GENERAL PUBLIC LICENSE')
    expect(text.textContent).toContain('Version 3, 29 June 2007')
    const third = within(dlg).getByText(/Third-party software/).closest('details')!
    expect(within(third).getByText('go.mongodb.org/mongo-driver/v2')).toBeInTheDocument()
    expect(within(third).getAllByText('Apache-2.0').length).toBeGreaterThan(0)
    fireEvent.click(within(dlg).getAllByRole('button', { name: 'Close' }).at(-1)!)
    expect(screen.queryByRole('dialog', { name: 'About Mango3S' })).toBeNull()
  })

})

describe('settings tabs and account', () => {
  const openSettings = async () => {
    render(<App />)
    fireEvent.click(await screen.findByTitle(/Settings \(/))
    await screen.findByRole('heading', { name: 'Settings' })
  }

  it('shows one section at a time and keeps unsaved edits across tabs', async () => {
    backend.saveSettings.mockImplementation(async (s: unknown) => s)
    await openSettings()
    const list = screen.getByRole('tablist', { name: 'Settings sections' })
    expect(within(list).getAllByRole('tab').map((t) => t.textContent)).toEqual([
      'General', 'AI assistant', 'MCP server', 'Keymap', 'Account & sync', 'Updates', 'Diagnostics',
    ])
    fireEvent.change(screen.getByRole('spinbutton', { name: /Documents per page/ }), { target: { value: '75' } })
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument()

    // Arrow keys move between tabs.
    fireEvent.keyDown(screen.getByRole('tab', { name: 'General' }), { key: 'ArrowDown' })
    expect(screen.getByRole('tab', { name: 'AI assistant' })).toHaveAttribute('aria-selected', 'true')
    fireEvent.click(screen.getByRole('tab', { name: 'Diagnostics' }))
    expect(screen.queryByRole('spinbutton', { name: /Documents per page/ })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /Save settings/ }))
    await waitFor(() => expect(backend.saveSettings).toHaveBeenCalled())
    expect(backend.saveSettings.mock.calls[0][0]).toMatchObject({ pageSize: 75 })

    // Keymap and MCP save on their own: no shared Save bar there.
    fireEvent.click(screen.getByRole('tab', { name: 'Keymap' }))
    expect(screen.queryByRole('button', { name: /Save settings/ })).toBeNull()
  })

  it('signs up with an emailed code and shows the recovery key once', async () => {
    backend.accountStartSignUp.mockResolvedValue({ codeSentTo: 'ada@example.com', expiresInMinutes: 15 })
    const signedIn = { ...signedOut, signedIn: true, email: 'ada@example.com', account: { clientId: 'c', email: 'ada@example.com', createdAt: '', recoveryKeyCreatedAt: null, machines: [], sync: { enabled: true, itemCount: 0, lastSyncedAt: null, maxItems: 500, maxItemKb: 256 } } }
    backend.accountFinishSignUp.mockResolvedValue({ state: signedIn, recoveryKey: '7KQ2-9XWD-AAAA-BBBB' })
    backend.refreshAccount.mockResolvedValue(signedIn)
    await openSettings()
    fireEvent.click(screen.getByRole('tab', { name: 'Account & sync' }))
    fireEvent.click(await screen.findByRole('tab', { name: 'Create account' }))

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'ada@example.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'weakpass' } })
    expect(screen.getByRole('list', { name: 'Password requirements' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Send code/ })).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'Correct-Horse-9' } })
    fireEvent.change(screen.getByLabelText('Repeat password'), { target: { value: 'Correct-Horse-9' } })
    fireEvent.click(screen.getByRole('button', { name: /Send code/ }))
    await waitFor(() => expect(backend.accountStartSignUp).toHaveBeenCalledWith('ada@example.com', 'Correct-Horse-9'))

    fireEvent.change(await screen.findByLabelText('Code'), { target: { value: '12a3456' } })
    expect(screen.getByLabelText('Code')).toHaveValue('123456')
    fireEvent.click(screen.getByRole('button', { name: /Create account/ }))
    await waitFor(() => expect(backend.accountFinishSignUp).toHaveBeenCalledWith('ada@example.com', '123456', 'Correct-Horse-9'))

    expect(await screen.findByLabelText('Your recovery key')).toHaveTextContent('7KQ2-9XWD-AAAA-BBBB')
    const cont = screen.getByRole('button', { name: 'Continue' })
    expect(cont).toBeDisabled()
    fireEvent.click(screen.getByRole('checkbox', { name: /I saved my recovery key/ }))
    fireEvent.click(cont)
    expect(await screen.findByText('Signed in as')).toBeInTheDocument()
    expect(screen.queryByLabelText('Your recovery key')).toBeNull()
  })

  it('hides what the server turned off', async () => {
    backend.getAccountState.mockResolvedValue({ ...signedOut, disabled: { registration: true, passwordReset: true } })
    await openSettings()
    fireEvent.click(screen.getByRole('tab', { name: 'Account & sync' }))
    expect(await screen.findByRole('tab', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: 'Create account' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Forgot password?' })).toBeNull()
  })

  it('syncs and restores when signed in; restoring asks for the password', async () => {
    const signedIn = { ...signedOut, signedIn: true, email: 'ada@example.com', pendingChanges: 2, account: { clientId: 'c', email: 'ada@example.com', createdAt: '', recoveryKeyCreatedAt: null, machines: [{ machineId: 'abcdef123456', label: 'Ada-MBP', lastSeenAt: null }], sync: { enabled: true, itemCount: 3, lastSyncedAt: null, maxItems: 500, maxItemKb: 256 } } }
    backend.getAccountState.mockResolvedValue(signedIn)
    backend.refreshAccount.mockResolvedValue(signedIn)
    backend.syncNow.mockResolvedValue({ uploaded: 2, deleted: 0, added: 0, updated: 0, removed: 0, skipped: 0, errors: [] })
    backend.syncRestore.mockResolvedValue({ uploaded: 0, deleted: 0, added: 1, updated: 0, removed: 0, skipped: 0, errors: [] })
    await openSettings()
    fireEvent.click(screen.getByRole('tab', { name: 'Account & sync' }))
    expect(await screen.findByText('ada@example.com')).toBeInTheDocument()
    expect(screen.getByText(/2 changes not uploaded yet/)).toBeInTheDocument()
    expect(screen.getByText('Ada-MBP')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Sync now/ }))
    await waitFor(() => expect(backend.syncNow).toHaveBeenCalled())
    expect(await screen.findByText('Connections synced')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Restore from account/ }))
    const form = screen.getByRole('form', { name: 'Restore connections' })
    const submit = within(form).getByRole('button', { name: /Restore connections/ })
    expect(submit).toBeDisabled()
    fireEvent.change(within(form).getByLabelText('Account password'), { target: { value: 'Correct-Horse-9' } })
    fireEvent.click(submit)
    await waitFor(() => expect(backend.syncRestore).toHaveBeenCalledWith('Correct-Horse-9'))
    expect(await screen.findByText('Connections restored')).toBeInTheDocument()
  })

  it('About shows the server links and the donation only when the admin turned it on', async () => {
    backend.getPlatformInfo.mockResolvedValue({ name: 'Mango3S', websiteUrl: 'https://mango3s.com', downloadUrl: '', privacyPolicyUrl: 'https://mango3s.com/privacy', contactUrl: 'https://samanfekri.me', donation: null })
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })
    act(() => (menuListeners.about ?? []).forEach((f) => f()))
    let dlg = await screen.findByRole('dialog', { name: 'About Mango3S' })
    fireEvent.click(await within(dlg).findByRole('button', { name: 'Privacy policy' }))
    expect(backend.openPlatformLink).toHaveBeenCalledWith('https://mango3s.com/privacy')
    expect(within(dlg).queryByRole('button', { name: /Donate/ })).toBeNull()
    fireEvent.click(within(dlg).getAllByRole('button', { name: 'Close' }).at(-1)!)

    backend.getPlatformInfo.mockResolvedValue({ name: 'Mango3S', websiteUrl: 'https://mango3s.com', downloadUrl: '', privacyPolicyUrl: '', contactUrl: '', donation: { url: 'https://donate.example', heading: 'Support Mango3S', message: 'Made by one person', buttonLabel: 'Buy me a coffee' } })
    act(() => (menuListeners.about ?? []).forEach((f) => f()))
    dlg = await screen.findByRole('dialog', { name: 'About Mango3S' })
    fireEvent.click(await within(dlg).findByRole('button', { name: /Buy me a coffee/ }))
    expect(backend.openPlatformLink).toHaveBeenCalledWith('https://donate.example')
  })
})

describe('Mango, the mascot', () => {
  it('lives in the explorer, shares a tip when clicked, and can be hidden', async () => {
    backend.saveSettings.mockImplementation(async (s: unknown) => s)
    render(<App />)
    const mango = await screen.findByRole('button', { name: /Mango, the Mango3S mascot/ })
    fireEvent.click(mango)
    expect(await screen.findByText(/Say Mango3S fast: Mango Trees/)).toBeInTheDocument()
    fireEvent.click(mango)
    expect(await screen.findByText(/Run a query with/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Hide Mango' }))
    await waitFor(() => expect(backend.saveSettings).toHaveBeenCalledWith(expect.objectContaining({ showMascot: false })))
    await waitFor(() => expect(screen.queryByRole('button', { name: /Mango, the Mango3S mascot/ })).toBeNull())
    expect(await screen.findByText('Mango is resting')).toBeInTheDocument()
  })

  it('stays hidden when turned off in Settings', async () => {
    backend.getSettings.mockResolvedValue({ ...(await backend.getSettings()), showMascot: false })
    render(<App />)
    await screen.findByRole('heading', { name: 'Welcome to Mango3S' })
    expect(screen.queryByRole('button', { name: /Mango, the Mango3S mascot/ })).toBeNull()
  })
})

describe('reordering connections', () => {
  const p2 = { ...profile, id: 'p2', name: 'Staging' }
  const p3 = { ...profile, id: 'p3', name: 'Prod', folder: 'Clients' }

  const dragOnto = (from: HTMLElement, to: HTMLElement, before: boolean) => {
    const data = { setData: vi.fn(), getData: vi.fn(), effectAllowed: '', dropEffect: '' }
    to.getBoundingClientRect = () => ({ top: 100, height: 20, bottom: 120, left: 0, right: 200, width: 200, x: 0, y: 100, toJSON: () => ({}) })
    fireEvent.dragStart(from, { dataTransfer: data })
    fireEvent.dragOver(to, { dataTransfer: data, clientY: before ? 102 : 118 })
    fireEvent.drop(to, { dataTransfer: data, clientY: before ? 102 : 118 })
  }

  it('drag in the explorer saves the new order', async () => {
    backend.listProfiles.mockResolvedValue([profile, p2, p3])
    backend.reorderProfiles.mockImplementation(async (ids: string[]) => ids.map((id) => [profile, p2, p3].find((p) => p.id === id)))
    render(<App />)
    const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
    const row = async (name: string) => (await within(sidebar).findByText(name)).closest('.tree-row') as HTMLElement
    // Drop Local dev onto the lower half of Prod: it moves after it.
    dragOnto(await row('Local dev'), await row('Prod'), false)
    await waitFor(() => expect(backend.reorderProfiles).toHaveBeenCalledWith(['p2', 'p3', 'p1']))
    await waitFor(() => {
      const names = [...sidebar.querySelectorAll('.saved .tree-label')].map((e) => e.textContent?.replace('Clients / ', ''))
      expect(names).toEqual(['Staging', 'Prod', 'Local dev'])
    })
  })

  it('Alt+1–9 follow the connection order, not last use', async () => {
    const used = { ...p3, lastConnectedAt: new Date().toISOString() }
    backend.listProfiles.mockResolvedValue([profile, p2, used])
    backend.reorderProfiles.mockImplementation(async (ids: string[]) => ids.map((id) => [profile, p2, used].find((p) => p.id === id)))
    render(<App />)
    const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
    const row = async (name: string) => (await within(sidebar).findByText(name)).closest('.tree-row') as HTMLElement
    expect(await row('Staging')).toHaveAttribute('title', expect.stringContaining('(Alt+2)'))
    key({ key: '1', code: 'Digit1', altKey: true })
    await waitFor(() => expect(backend.connect).toHaveBeenCalledWith('p1'))
    // Move Local dev to the end: Staging becomes connection 1.
    dragOnto(await row('Local dev'), await row('Prod'), false)
    await waitFor(() => expect(backend.reorderProfiles).toHaveBeenCalledWith(['p2', 'p3', 'p1']))
    await dismissConnectLog()
    await waitFor(async () => expect(await row('Staging')).toHaveAttribute('title', expect.stringContaining('(Alt+1)')))
    key({ key: '1', code: 'Digit1', altKey: true })
    await waitFor(() => expect(backend.connect).toHaveBeenCalledWith('p2'))
  })

  it('Alt+↓ moves the focused connection down', async () => {
    backend.listProfiles.mockResolvedValue([profile, p2])
    backend.reorderProfiles.mockImplementation(async (ids: string[]) => ids.map((id) => [profile, p2].find((p) => p.id === id)))
    render(<App />)
    const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
    fireEvent.click(await within(sidebar).findAllByRole('button', { name: 'Connect' }).then((b) => b[0]))
    await dismissConnectLog()
    const tree = within(sidebar).getByRole('tree')
    tree.focus()
    act(() => {
      tree.dispatchEvent(new KeyboardEvent('keydown', { key: 'Home', bubbles: true }))
    })
    act(() => {
      tree.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', altKey: true, bubbles: true }))
    })
    await waitFor(() => expect(backend.reorderProfiles).toHaveBeenCalledWith(['p2', 'p1']))
  })

  it('dropping on a folder in the connection manager moves the connection into it', async () => {
    backend.listProfiles.mockResolvedValue([profile, p3])
    backend.setProfileFolder.mockResolvedValue({ ...profile, folder: 'Clients' })
    render(<App />)
    await screen.findByText('Local dev', { selector: '.welcome-name' })
    fireEvent.click(within(screen.getByRole('tabpanel')).getByRole('button', { name: 'Open saved connections…' }))
    const dlg = await screen.findByRole('dialog', { name: 'Connection Manager' })
    const item = within(dlg).getByRole('button', { name: /Local dev/ })
    const folder = within(dlg).getByRole('button', { name: /Clients/ })
    const data = { setData: vi.fn(), getData: vi.fn(), effectAllowed: '', dropEffect: '' }
    fireEvent.dragStart(item, { dataTransfer: data })
    fireEvent.dragOver(folder, { dataTransfer: data })
    fireEvent.drop(folder, { dataTransfer: data })
    await waitFor(() => expect(backend.setProfileFolder).toHaveBeenCalledWith('p1', 'Clients'))
  })
})
