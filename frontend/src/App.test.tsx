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
const session = { id: 'p1', name: 'Local dev', readOnly: false, serverVersion: '8.0.0', topology: 'standalone', connectedAt: '' }

const backend = vi.hoisted(() => ({
  getAppInfo: vi.fn(),
  getSettings: vi.fn(),
  saveSettings: vi.fn(),
  listProfiles: vi.fn(),
  saveProfile: vi.fn(),
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
  detectAIProviders: vi.fn(),
  listLocalModels: vi.fn(),
  testAI: vi.fn(),
  chooseFile: vi.fn(),
  runShellPage: vi.fn(),
}))

const jobListeners = vi.hoisted(() => [] as ((j: unknown) => void)[])

vi.mock('./lib/backend', () => ({
  backend,
  newOpId: (p: string) => `${p}-1`,
  onMenu: () => () => undefined,
  onJobUpdate: (cb: (j: unknown) => void) => {
    jobListeners.push(cb)
    return () => undefined
  },
}))

import App from './App'
import { setShortcutPlatform } from './state/shortcuts'

beforeEach(() => {
  vi.clearAllMocks()
  backend.getAppInfo.mockResolvedValue({ version: 'test', configDir: '/tmp', logFile: '', secretBackend: 'file' })
  backend.getSettings.mockResolvedValue({
    theme: 'dark', pageSize: 50, queryTimeoutSeconds: 60, confirmDeletes: true, defaultView: 'tree', editorFontSize: 13, logLevel: 'info',
    aiEnabled: true, aiProvider: 'anthropic', aiModel: 'claude-opus-5-5', aiTimeoutSeconds: 180, claudeCodePath: '', claudeCodeModel: '', codexPath: '', codexModel: '',
    localUrl: 'http://localhost:11434', localApi: 'ollama', localModel: '', aiAllowSamples: false, schedulerEnabled: true,
  })
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
    const status = (await within(tree).findAllByRole('row')).find((r) => r.textContent?.startsWith('status'))
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

const key = (init: KeyboardEventInit & { code?: string }, target: EventTarget = document.body) =>
  act(() => {
    target.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }))
  })

async function connectAndOpenOrders() {
  render(<App />)
  const sidebar = screen.getByRole('navigation', { name: 'Database explorer' })
  fireEvent.click(await within(sidebar).findByRole('button', { name: 'Connect' }))
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
    expect(await screen.findByRole('list', { name: 'Recent connections' })).toHaveTextContent('Local dev')
    fireEvent.click(screen.getAllByRole('button', { name: 'Keyboard shortcuts' })[0])
    const dialog = await screen.findByRole('dialog', { name: 'Keyboard shortcuts' })
    expect(within(dialog).getByText('Open the connection manager')).toBeInTheDocument()
    expect(within(dialog).getAllByText('Studio 3T').length).toBeGreaterThan(5)
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

    key({ key: 'p', code: 'KeyP', ctrlKey: true }) // settings (Studio 3T: Preferences)
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
    expect(within(sidebar).getByText('Collections')).toBeInTheDocument() // grouped like Studio 3T
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
    const row = (await within(tree).findAllByRole('row')).find((r) => r.textContent?.startsWith('status'))!
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
      'Server', 'Authentication', 'SSL / TLS', 'SSH', 'Proxy', 'IntelliShell', 'MongoDB Tools', 'Advanced', 'Collection History',
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
