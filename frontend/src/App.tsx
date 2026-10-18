import { Component, useEffect } from 'react'
import { GearIcon } from './components/icons'
import { ShortcutsDialog } from './components/ShortcutsDialog'
import { onMenu } from './lib/backend'
import { recentProfiles } from './lib/tabs'
import { shortcutHint } from './lib/shortcuts'
import { TabActiveContext, runShortcut, useShortcut, useShortcutDispatcher } from './state/shortcuts'
import { ConnectionView } from './views/ConnectionView'
import type { ErrorInfo, ReactNode } from 'react'
import { ConfirmDialog, PromptDialog, Toasts } from './components/common'
import { AboutDialog } from './components/AboutDialog'
import { ConnectionLogDialog } from './components/ConnectionLogDialog'
import { ConnectionManager } from './components/ConnectionManager'
import { JobsIndicator } from './components/JobsPanel'
import { Sidebar } from './components/Sidebar'
import type { Tab } from './lib/tabs'
import { AppProvider, useApp } from './state/AppContext'
import { AggregationView } from './views/AggregationView'
import { CollectionView } from './views/CollectionView'
import { CompareView } from './views/CompareView'
import { MigrateView } from './views/MigrateView'
import { TasksView } from './views/TasksView'
import { TransformView } from './views/TransformView'
import { LibraryView } from './views/LibraryView'
import { SchemaView } from './views/SchemaView'
import { SettingsView } from './views/SettingsView'
import { ShellView } from './views/ShellView'
import { SqlView } from './views/SqlView'
import { WelcomeView } from './views/WelcomeView'

class TabErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }
  static getDerivedStateFromError(error: Error) {
    return { error }
  }
  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('UI error', error, info.componentStack)
  }
  render() {
    if (this.state.error) {
      return (
        <div className="view">
          <div className="error-banner" role="alert">
            <div>
              <strong>This tab crashed</strong>
              <div className="error-message">{this.state.error.message}</div>
            </div>
            <button onClick={() => this.setState({ error: null })}>Retry</button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

function TabContent({ tab }: { tab: Tab }) {
  switch (tab.kind) {
    case 'welcome':
      return <WelcomeView />
    case 'settings':
      return <SettingsView tab={tab} />
    case 'collection':
      return <CollectionView tab={tab} />
    case 'aggregation':
      return <AggregationView tab={tab} />
    case 'shell':
      return <ShellView tab={tab} />
    case 'schema':
      return <SchemaView tab={tab} />
    case 'sql':
      return <SqlView tab={tab} />
    case 'library':
      return <LibraryView tab={tab} />
    case 'compare':
      return <CompareView tab={tab} />
    case 'reschema':
    case 'mask':
      return <TransformView tab={tab} />
    case 'migrate':
      return <MigrateView tab={tab} />
    case 'tasks':
      return <TasksView />
    case 'connection':
      return <ConnectionView tab={tab} />
  }
}

function TabBar() {
  const { tabs, dispatchTabs, closeTab, dirtyTabs, setShortcutsOpen } = useApp()
  return (
    <div className="tabbar" role="tablist">
      {tabs.tabs.map((t, i) => {
        const dirty = dirtyTabs[t.id]
        return (
          <div
            key={t.id}
            role="tab"
            aria-selected={t.id === tabs.activeId}
            className={`tab${t.id === tabs.activeId ? ' active' : ''}${dirty ? ' dirty' : ''}`}
            onClick={() => dispatchTabs({ type: 'activate', id: t.id })}
            onAuxClick={(e) => e.button === 1 && void closeTab(t.id)}
            draggable={t.kind !== 'welcome'}
            onDragStart={(e) => e.dataTransfer.setData('text/tab-index', String(i))}
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              const from = Number(e.dataTransfer.getData('text/tab-index'))
              if (!Number.isNaN(from)) dispatchTabs({ type: 'move', from, to: i })
            }}
            title={(t.connName && t.db ? `${t.connName} / ${t.db}${t.coll ? ' / ' + t.coll : ''}` : t.title) + (dirty ? ` — unsaved: ${dirty}` : '')}
          >
            <span className={`tab-icon tab-icon-${t.kind}`} aria-hidden />
            <span className="tab-title">{t.title}</span>
            {dirty && <span className="tab-dirty" aria-label="unsaved changes" />}
            <button
              className="tab-close"
              aria-label={`Close ${t.title}`}
              title={`Close (${shortcutHint('tab.close')})`}
              onClick={(e) => {
                e.stopPropagation()
                void closeTab(t.id)
              }}
            >
              ×
            </button>
          </div>
        )
      })}
      <span className="spacer" />
      <button className="small ghost tabbar-btn" onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'tasks' } })} title="Scheduled tasks">
        Tasks
      </button>
      <button className="small ghost tabbar-btn" onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'library' } })} title={`Saved queries and history (${shortcutHint('app.library')})`}>
        Library
      </button>
      <button className="small ghost tabbar-btn tabbar-icon" onClick={() => setShortcutsOpen(true)} title={`Keyboard shortcuts (${shortcutHint('help.shortcuts')})`} aria-label="Keyboard shortcuts">
        ?
      </button>
      <button className="small ghost tabbar-btn tabbar-icon" onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'settings' } })} title={`Settings (${shortcutHint('app.settings')})`} aria-label="Settings">
        <GearIcon />
      </button>
    </div>
  )
}

/** App-wide shortcuts and native menu items. */
function GlobalShortcuts() {
  const { tabs, dispatchTabs, closeTab, openConnectionManager, profiles, sessions, connect, disconnect, shortcutsOpen, setShortcutsOpen, setAboutOpen } = useApp()
  const active = tabs.tabs.find((t) => t.id === tabs.activeId)
  const activeSession = sessions.find((s) => s.id === active?.connId)

  useShortcut('conn.manager', () => openConnectionManager(true))
  useShortcut('conn.new', () => openConnectionManager(true, 'new'))
  useShortcut('conn.quick', ({ index }) => {
    const p = recentProfiles(profiles)[index]
    if (!p) return false
    const live = sessions.find((s) => s.id === p.id)
    if (live) dispatchTabs({ type: 'open', tab: { kind: 'connection', connId: live.id, connName: live.name } })
    else void connect(p.id)
  })
  useShortcut('conn.disconnect', () => (activeSession ? void disconnect(activeSession.id) : false))
  useShortcut('tab.close', () => (tabs.activeId ? void closeTab(tabs.activeId) : false))
  useShortcut('tab.closeNoPrompt', () => (tabs.activeId ? void closeTab(tabs.activeId, true) : false))
  useShortcut('tab.next', () => dispatchTabs({ type: 'step', delta: 1 }))
  useShortcut('tab.prev', () => dispatchTabs({ type: 'step', delta: -1 }))
  useShortcut('tab.goto', ({ index }) => {
    const t = tabs.tabs[index === 8 ? tabs.tabs.length - 1 : index] // Mod+9 is the last tab, as in browsers
    if (!t) return false
    dispatchTabs({ type: 'activate', id: t.id })
  })
  useShortcut('tab.welcome', () => dispatchTabs({ type: 'open', tab: { kind: 'welcome' } }))
  useShortcut('tab.shell', () => {
    if (!activeSession) return false
    dispatchTabs({ type: 'open', tab: { kind: 'shell', connId: activeSession.id, connName: activeSession.name, db: active?.db } })
  })
  useShortcut('tab.sql', () => {
    if (!activeSession) return false
    dispatchTabs({ type: 'open', tab: { kind: 'sql', connId: activeSession.id, connName: activeSession.name, db: active?.db ?? 'test' } })
  })
  useShortcut('app.settings', () => dispatchTabs({ type: 'open', tab: { kind: 'settings' } }))
  useShortcut('app.library', () => dispatchTabs({ type: 'open', tab: { kind: 'library', init: { section: 'history' } } }))
  useShortcut('help.shortcuts', () => setShortcutsOpen(!shortcutsOpen))

  // Native menu (File / View / Help) runs the same actions.
  useEffect(() => {
    const offs = [
      onMenu('connection-manager', () => openConnectionManager(true)),
      onMenu('new-connection', () => openConnectionManager(true, 'new')),
      onMenu('close-tab', () => runShortcut('tab.close')),
      onMenu('welcome', () => dispatchTabs({ type: 'open', tab: { kind: 'welcome' } })),
      onMenu('library', () => dispatchTabs({ type: 'open', tab: { kind: 'library' } })),
      onMenu('tasks', () => dispatchTabs({ type: 'open', tab: { kind: 'tasks' } })),
      onMenu('settings', () => dispatchTabs({ type: 'open', tab: { kind: 'settings' } })),
      onMenu('shortcuts', () => setShortcutsOpen(true)),
      onMenu('about', () => setAboutOpen(true)),
      onMenu('keymap', () => dispatchTabs({ type: 'open', tab: { kind: 'settings', init: { settingsSection: 'keymap' } } })),
    ]
    return () => offs.forEach((off) => off())
  }, [openConnectionManager, dispatchTabs, setShortcutsOpen, setAboutOpen])
  return null
}

function StatusBar() {
  const { sessions, appInfo, tabs } = useApp()
  const active = tabs.tabs.find((t) => t.id === tabs.activeId)
  const session = sessions.find((s) => s.id === active?.connId)
  return (
    <footer className="statusbar">
      <span>
        {sessions.length} connection{sessions.length === 1 ? '' : 's'} open
      </span>
      {session && (
        <span>
          {session.name} · MongoDB {session.serverVersion} · {session.topology}
          {session.readOnly ? ' · read-only' : ''}
        </span>
      )}
      <span className="spacer" />
      <JobsIndicator />
      {appInfo && <span>v{appInfo.version}</span>}
    </footer>
  )
}

function Shell() {
  const { tabs, dispatchTabs, connectionManagerOpen, shortcutsOpen, setShortcutsOpen, aboutOpen, setAboutOpen } = useApp()
  useShortcutDispatcher()

  return (
    <div className="app">
      <Sidebar />
      <main className="main">
        <TabBar />
        <div className="tab-panels">
          {tabs.tabs.length === 0 && (
            <div className="empty-state">
              Open a collection from the explorer, or{' '}
              <button className="link" onClick={() => dispatchTabs({ type: 'open', tab: { kind: 'welcome' } })}>
                show the Welcome tab
              </button>
              .
            </div>
          )}
          {/* Inactive tabs stay mounted so their editors and results survive tab switches. */}
          {tabs.tabs.map((t) => (
            <div key={t.id} className="tab-panel" role="tabpanel" hidden={t.id !== tabs.activeId}>
              <TabActiveContext.Provider value={t.id === tabs.activeId}>
                <TabErrorBoundary>
                  <TabContent tab={t} />
                </TabErrorBoundary>
              </TabActiveContext.Provider>
            </div>
          ))}
        </div>
      </main>
      <StatusBar />
      <GlobalShortcuts />
      {connectionManagerOpen && <ConnectionManager />}
      {shortcutsOpen && <ShortcutsDialog onClose={() => setShortcutsOpen(false)} />}
      <ConnectionLogDialog />
      {aboutOpen && <AboutDialog onClose={() => setAboutOpen(false)} />}
      <ConfirmDialog />
      <PromptDialog />
      <Toasts />
    </div>
  )
}

export default function App() {
  return (
    <AppProvider>
      <Shell />
    </AppProvider>
  )
}
