import { useMemo, useState } from 'react'
import { SHORTCUTS, formatShortcut } from '../lib/shortcuts'
import { getShortcutPlatform } from '../state/shortcuts'
import { Modal } from './common'

const SCOPE_LABEL: Record<string, string> = {
  global: 'anywhere',
  tab: 'in the active tab',
  explorer: 'in the explorer',
  results: 'in results',
  editor: 'in editors',
}

/** The keyboard shortcut reference (Help → Keyboard Shortcuts, F1). */
export function ShortcutsDialog({ onClose }: { onClose: () => void }) {
  const platform = getShortcutPlatform()
  const [filter, setFilter] = useState('')
  const groups = useMemo(() => {
    const f = filter.trim().toLowerCase()
    const out = new Map<string, typeof SHORTCUTS>()
    for (const s of SHORTCUTS) {
      const keys = formatShortcut(s, platform)
      if (f && !`${s.label} ${s.category} ${keys}`.toLowerCase().includes(f)) continue
      out.set(s.category, [...(out.get(s.category) ?? []), s])
    }
    return [...out.entries()]
  }, [filter, platform])

  return (
    <Modal title="Keyboard shortcuts" onClose={onClose} wide>
      <div className="shortcuts-head">
        <input autoFocus placeholder="Search shortcuts" value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Search shortcuts" />
        <span className="muted small">
          {platform === 'mac' ? 'macOS bindings' : 'Windows / Linux bindings'}. <span className="source-badge studio3t">Studio 3T</span> matches Studio 3T’s
          documented default; <span className="source-badge default">Mango3S</span> is a Mango3S default where Studio 3T has none (or it would clash).
        </span>
      </div>
      <div className="shortcuts-grid">
        {groups.map(([cat, list]) => (
          <section key={cat} className="shortcut-group">
            <h3>{cat}</h3>
            <table className="shortcut-table">
              <tbody>
                {list.map((s) => (
                  <tr key={s.id}>
                    <td>
                      {s.label}
                      {s.note && <div className="muted small">{s.note}</div>}
                    </td>
                    <td className="keys">
                      {s.id === 'cell.edit' ? <kbd>Double-click</kbd> : <kbd>{formatShortcut(s, platform)}</kbd>}
                    </td>
                    <td className="muted small">{SCOPE_LABEL[s.scope]}</td>
                    <td>
                      <span className={`source-badge ${s.source}`}>{s.source === 'studio3t' ? 'Studio 3T' : 'Mango3S'}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        ))}
        {groups.length === 0 && <p className="muted">No shortcut matches “{filter}”.</p>}
      </div>
    </Modal>
  )
}
