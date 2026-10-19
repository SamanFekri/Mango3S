import { useMemo } from 'react'
import { shortcutHint } from '../lib/shortcuts'
import { useApp } from '../state/AppContext'
import { MangoBuddy, type BuddyMood } from './MangoBuddy'

/** Mango at the bottom of the explorer: cheers when something succeeds, looks
 * worried at an error, and shares a tip when clicked. Settings → General →
 * "Show Mango" turns it on or off (the × does the same). */
export function AppBuddy() {
  const { settings, saveSettings, toasts, toast, toastError } = useApp()
  const latest = toasts.at(-1)
  const lines = useMemo(
    () => [
      "Hi, I'm Mango! Say Mango3S fast: Mango Trees.",
      `Run a query with ${shortcutHint('query.run') || '⌘↵'} — no need to reach for the mouse.`,
      'Double-click any value in the Tree or Table view to edit just that field.',
      'Generate code turns a query into Python, Java, C#, Go or Node.js.',
      'Schema shows every field, its types and how often it appears.',
      'Explain shows how MongoDB runs a query, and which index would help.',
      `Press ${shortcutHint('help.shortcuts') || 'F1'} for every keyboard shortcut.`,
      'Settings → Account & sync backs your connections up, encrypted.',
    ],
    [],
  )
  if (!settings || settings.showMascot === false) return null

  const mood: BuddyMood | undefined = latest ? (latest.kind === 'success' ? 'happy' : latest.kind === 'error' ? 'worried' : undefined) : undefined
  const hide = async () => {
    try {
      await saveSettings({ ...settings, showMascot: false })
      toast({ kind: 'info', title: 'Mango is resting', message: 'Bring it back in Settings → General.' })
    } catch (err) {
      toastError(err, 'Could not save the setting')
    }
  }
  return (
    <div className="sidebar-buddy">
      <MangoBuddy lines={lines} mood={mood} moodKey={latest?.id} onHide={hide} size={58} />
    </div>
  )
}
