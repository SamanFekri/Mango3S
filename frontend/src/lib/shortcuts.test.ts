import { render } from '@testing-library/react'
import { createElement, useRef } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { handleKeyDown, resetShortcuts, setShortcutPlatform, useShortcut, useShortcutDispatcher } from '../state/shortcuts'
import { SHORTCUTS, eventKey, formatBinding, formatShortcut, isTextTarget, matchIndex, normalizeBinding, SHORTCUT_BY_ID } from './shortcuts'

afterEach(() => {
  resetShortcuts()
  document.body.innerHTML = ''
})

const ev = (init: KeyboardEventInit & { code?: string }) => new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init })

describe('bindings', () => {
  it('resolve Mod per platform and match layout-independent keys', () => {
    expect(normalizeBinding('Mod+Shift+n', 'mac')).toBe('Shift+Meta+N')
    expect(normalizeBinding('Mod+Shift+n', 'other')).toBe('Ctrl+Shift+N')
    const def = SHORTCUT_BY_ID['conn.manager']
    expect(matchIndex(def, ev({ key: 'n', code: 'KeyN', metaKey: true }), 'mac')).toBe(0)
    expect(matchIndex(def, ev({ key: 'n', code: 'KeyN', ctrlKey: true }), 'mac')).toBe(-1)
    expect(matchIndex(def, ev({ key: 'n', code: 'KeyN', ctrlKey: true }), 'other')).toBe(0)
    // ⌥1 types "¡" on a Mac; the physical key still matches Alt+1.
    expect(eventKey(ev({ key: '¡', code: 'Digit1', altKey: true }))).toBe('1')
    expect(matchIndex(SHORTCUT_BY_ID['conn.quick'], ev({ key: '¡', code: 'Digit3', altKey: true }), 'mac')).toBe(2)
    // Shift+/ is "?" on US layouts; the binding is written with the physical key.
    expect(matchIndex(SHORTCUT_BY_ID['help.shortcuts'], ev({ key: '?', code: 'Slash', shiftKey: true, ctrlKey: true }), 'other')).toBe(1)
  })

  it('format for each platform', () => {
    expect(formatBinding('Mod+Shift+N', 'mac')).toBe('⇧⌘N')
    expect(formatBinding('Mod+Shift+N', 'other')).toBe('Ctrl+Shift+N')
    expect(formatBinding('Mod+Enter', 'mac')).toBe('⌘↩')
    expect(formatShortcut(SHORTCUT_BY_ID['conn.quick'], 'other')).toBe('Alt+1…9')
    expect(formatShortcut(SHORTCUT_BY_ID['tab.close'], 'other')).toBe('Ctrl+W  /  Ctrl+F4')
  })

  it('never claims standard text-editing keys outside results and editors', () => {
    const textKeys = ['Mod+C', 'Mod+V', 'Mod+X', 'Mod+Z', 'Mod+A', 'Mod+Shift+Z', 'Ctrl+Y', 'Delete', 'Backspace', 'Enter', 'Tab', 'Escape']
    for (const s of SHORTCUTS) {
      if (s.scope === 'results' || s.scope === 'editor' || s.scope === 'explorer') continue
      for (const k of [...s.mac, ...s.other]) {
        expect(textKeys, `${s.id} uses ${k}`).not.toContain(k)
      }
    }
  })

  it('has no duplicate binding within a scope on one platform', () => {
    for (const platform of ['mac', 'other'] as const) {
      const seen = new Map<string, string>()
      for (const s of SHORTCUTS) {
        if (s.handler !== 'app') continue
        for (const k of platform === 'mac' ? s.mac : s.other) {
          const key = `${s.scope === 'global' || s.scope === 'tab' ? 'app' : s.scope}:${normalizeBinding(k, platform)}`
          expect(seen.get(key), `${s.id} and ${seen.get(key)} both use ${k} on ${platform}`).toBeUndefined()
          seen.set(key, s.id)
        }
      }
    }
  })

  it('every shortcut the app handles has a registered handler in the source', () => {
    const sources = import.meta.glob(['../**/*.tsx', '!../**/*.test.tsx'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>
    const code = Object.values(sources).join('\n')
    for (const s of SHORTCUTS.filter((x) => x.handler === 'app')) {
      expect(code.includes(`useShortcut('${s.id}'`) || code.includes(`useShortcut(\n    '${s.id}'`), `no useShortcut('${s.id}') anywhere`).toBe(true)
    }
  })

  it('knows which targets are text fields', () => {
    const input = document.createElement('input')
    const box = document.createElement('input')
    box.type = 'checkbox'
    const ce = document.createElement('div')
    ce.className = 'cm-content'
    ce.contentEditable = 'true'
    document.body.append(input, box, ce)
    expect(isTextTarget(input)).toBe(true)
    expect(isTextTarget(box)).toBe(false)
    expect(isTextTarget(ce)).toBe(true)
    expect(isTextTarget(document.body)).toBe(false)
  })
})

function Probe({ id, fn, scoped }: { id: string; fn: () => void; scoped?: boolean }) {
  useShortcutDispatcher()
  const ref = useRef<HTMLDivElement>(null)
  useShortcut(id, fn, scoped ? { within: ref } : {})
  return createElement('div', null, createElement('input', { 'aria-label': 'field' }), createElement('div', { ref, tabIndex: 0, 'data-testid': 'zone' }))
}

describe('dispatcher', () => {
  it('runs global shortcuts even while typing, but only modifier combinations', () => {
    setShortcutPlatform('other')
    const fn = vi.fn()
    const { getByLabelText } = render(createElement(Probe, { id: 'conn.manager', fn }))
    const input = getByLabelText('field')
    input.focus()
    input.dispatchEvent(ev({ key: 'n', code: 'KeyN', ctrlKey: true }))
    expect(fn).toHaveBeenCalledTimes(1)
    input.dispatchEvent(ev({ key: 'n', code: 'KeyN' })) // plain typing
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('does not fire shortcuts that would clash with typing in a text field', () => {
    setShortcutPlatform('other')
    const fn = vi.fn()
    const { getByLabelText } = render(createElement(Probe, { id: 'conn.quick', fn }))
    const input = getByLabelText('field')
    input.focus()
    input.dispatchEvent(ev({ key: '1', code: 'Digit1', altKey: true }))
    expect(fn).not.toHaveBeenCalled()
    document.body.dispatchEvent(ev({ key: '1', code: 'Digit1', altKey: true }))
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('scopes results shortcuts to focus inside their element', () => {
    setShortcutPlatform('other')
    const fn = vi.fn()
    const { getByTestId } = render(createElement(Probe, { id: 'field.remove', fn, scoped: true }))
    document.body.dispatchEvent(ev({ key: 'Delete' }))
    expect(fn).not.toHaveBeenCalled()
    const zone = getByTestId('zone')
    zone.focus()
    zone.dispatchEvent(ev({ key: 'Delete' }))
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('ignores AltGr combinations (they type characters such as @)', () => {
    setShortcutPlatform('other')
    const fn = vi.fn()
    render(createElement(Probe, { id: 'conn.disconnect', fn }))
    const e = ev({ key: 'd', code: 'KeyD', ctrlKey: true, altKey: true })
    Object.defineProperty(e, 'getModifierState', { value: (k: string) => k === 'AltGraph' })
    expect(handleKeyDown(e)).toBe(false)
    expect(fn).not.toHaveBeenCalled()
  })

  it('leaves keys a component already handled alone', () => {
    setShortcutPlatform('other')
    const fn = vi.fn()
    render(createElement(Probe, { id: 'query.run', fn }))
    const e = ev({ key: 'Enter', ctrlKey: true })
    e.preventDefault() // e.g. CodeMirror ran the editor's own Mod+Enter
    document.body.dispatchEvent(e)
    expect(fn).not.toHaveBeenCalled()
  })
})
