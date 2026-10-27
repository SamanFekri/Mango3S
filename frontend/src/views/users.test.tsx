import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Tab } from '../lib/tabs'

const backend = vi.hoisted(() => ({
  listDatabases: vi.fn(),
  listUsers: vi.fn(),
  listRoles: vi.fn(),
  userEffective: vi.fn(),
  userCan: vi.fn(),
  createUser: vi.fn(),
  setUserPassword: vi.fn(),
  setUserRoles: vi.fn(),
  dropUser: vi.fn(),
  checkUserChange: vi.fn(),
  saveRole: vi.fn(),
  dropRole: vi.fn(),
}))
const app = vi.hoisted(() => ({
  confirm: vi.fn(),
  toast: vi.fn(),
  sessions: [{ id: 'c1', name: 'Prod', readOnly: false, serverVersion: '8.0.1', topology: 'standalone', connectedAt: '' }],
}))
vi.mock('../lib/backend', () => ({ backend }))
vi.mock('../state/AppContext', () => ({ useApp: () => app }))

import { UsersView } from './users/UsersView'
import { parseRole, groupPrivileges } from './users/roles'

const tab = { id: 't1', kind: 'users', connId: 'c1', connName: 'Prod' } as Tab
const admin = { user: 'admin', db: 'admin', roles: [{ role: 'root', db: 'admin' }], self: true }
const alice = { user: 'alice', db: 'shop', roles: [{ role: 'read', db: 'shop' }], self: false }

beforeEach(() => {
  vi.clearAllMocks()
  app.sessions = [{ ...app.sessions[0]!, readOnly: false }]
  backend.listDatabases.mockResolvedValue([{ name: 'shop' }, { name: 'logs' }])
  backend.listUsers.mockResolvedValue([admin, alice])
  backend.listRoles.mockResolvedValue([])
  backend.checkUserChange.mockResolvedValue([])
  backend.setUserRoles.mockResolvedValue(undefined)
  backend.saveRole.mockResolvedValue(undefined)
})

describe('Users and roles', () => {
  it('lists users and changes roles with checkboxes', async () => {
    render(<UsersView tab={tab} />)
    fireEvent.click(await screen.findByText('alice'))
    const panel = await screen.findByRole('complementary', { name: 'User alice' })
    fireEvent.click(within(panel).getByRole('checkbox', { name: 'readWrite' }))
    fireEvent.click(within(panel).getByRole('checkbox', { name: 'read' }))
    fireEvent.click(within(panel).getByRole('button', { name: 'Save roles' }))
    await waitFor(() => expect(backend.setUserRoles).toHaveBeenCalledWith('c1', 'shop', 'alice', [{ role: 'readWrite', db: 'shop' }]))
    expect(backend.checkUserChange).toHaveBeenCalledWith({ connId: 'c1', db: 'shop', user: 'alice', drop: false, roles: [{ role: 'readWrite', db: 'shop' }] })
    expect(app.confirm).not.toHaveBeenCalled() // no warnings
  })

  it('asks before taking away your own admin rights', async () => {
    backend.checkUserChange.mockResolvedValue(['This removes your own right to manage users and roles.'])
    app.confirm.mockResolvedValue(false)
    render(<UsersView tab={tab} />)
    fireEvent.click((await screen.findByText('you')).closest('tr')!)
    const panel = await screen.findByRole('complementary', { name: 'User admin' })
    fireEvent.click(within(panel).getByRole('checkbox', { name: 'root' }))
    fireEvent.click(within(panel).getByRole('button', { name: 'Save roles' }))
    await waitFor(() => expect(app.confirm).toHaveBeenCalledWith(expect.objectContaining({ danger: true })))
    expect(backend.setUserRoles).not.toHaveBeenCalled()
  })

  it('checks what a user can do and drops it after typing its name', async () => {
    backend.userCan.mockResolvedValue(['insert'])
    app.confirm.mockResolvedValue(true)
    render(<UsersView tab={tab} />)
    fireEvent.click(await screen.findByText('alice'))
    const panel = await screen.findByRole('complementary', { name: 'User alice' })
    fireEvent.change(within(panel).getByLabelText('Action'), { target: { value: 'insert' } })
    fireEvent.change(within(panel).getByLabelText('Collection to check'), { target: { value: 'orders' } })
    fireEvent.click(within(panel).getByRole('button', { name: 'Check' }))
    expect(await within(panel).findByText('✗ Not allowed: insert on shop.orders')).toBeInTheDocument()
    expect(backend.userCan).toHaveBeenCalledWith('c1', 'shop', 'alice', 'shop', 'orders', ['insert'])

    fireEvent.click(within(panel).getByRole('button', { name: 'Drop user…' }))
    await waitFor(() => expect(backend.dropUser).toHaveBeenCalledWith('c1', 'shop', 'alice', 'alice'))
    expect(app.confirm).toHaveBeenCalledWith(expect.objectContaining({ typeToConfirm: 'alice' }))
  })

  it('creates a user with a password and roles', async () => {
    backend.createUser.mockResolvedValue(undefined)
    render(<UsersView tab={tab} />)
    await screen.findByText('alice')
    fireEvent.change(screen.getByRole('combobox', { name: 'Database' }), { target: { value: 'shop' } })
    fireEvent.click(screen.getByRole('button', { name: 'New user…' }))
    fireEvent.change(await screen.findByLabelText('User name'), { target: { value: 'bob' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'pw 1' } })
    fireEvent.change(screen.getByLabelText('Repeat password'), { target: { value: 'pw 1' } })
    fireEvent.click(screen.getByRole('button', { name: /Create user/ }))
    await waitFor(() => expect(backend.createUser).toHaveBeenCalledWith({ connId: 'c1', db: 'shop', user: 'bob', password: 'pw 1', roles: [{ role: 'read', db: 'shop' }] }))
  })

  it('creates a custom role from the privilege matrix', async () => {
    render(<UsersView tab={tab} />)
    await screen.findByText('alice')
    fireEvent.click(screen.getByRole('radio', { name: 'Roles' }))
    fireEvent.change(screen.getByRole('combobox', { name: 'Database' }), { target: { value: 'shop' } })
    fireEvent.click(await screen.findByRole('button', { name: 'New role…' }))
    const editor = await screen.findByRole('complementary', { name: 'New role' })
    fireEvent.change(within(editor).getByLabelText('Role name'), { target: { value: 'reporter' } })
    fireEvent.change(within(editor).getByLabelText('Collection'), { target: { value: 'reports' } })
    fireEvent.click(within(editor).getByRole('checkbox', { name: 'insert' }))
    fireEvent.click(within(editor).getByRole('button', { name: 'Create role' }))
    await waitFor(() =>
      expect(backend.saveRole).toHaveBeenCalledWith(
        { connId: 'c1', db: 'shop', role: 'reporter', privileges: [{ resource: { db: 'shop', collection: 'reports' }, actions: ['find', 'insert'] }], roles: [] },
        true,
      ),
    )
  })

  it('parses roles and groups privileges', () => {
    expect(parseRole(' read@shop ', 'admin')).toEqual({ role: 'read', db: 'shop' })
    expect(parseRole('readWrite', 'logs')).toEqual({ role: 'readWrite', db: 'logs' })
    expect(parseRole('  ', 'logs')).toBeNull()
    expect(
      groupPrivileges([
        { resource: { db: 'shop', collection: '' }, actions: ['find'] },
        { resource: { db: 'shop', collection: '' }, actions: ['insert', 'find'] },
        { resource: { cluster: true }, actions: ['serverStatus'] },
      ]),
    ).toEqual([
      { label: 'cluster', actions: ['serverStatus'] },
      { label: 'shop (every collection)', actions: ['find', 'insert'] },
    ])
  })
})
