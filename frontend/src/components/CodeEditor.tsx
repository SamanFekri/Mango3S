import { useEffect, useRef } from 'react'
import type { RefObject } from 'react'
import { autocompletion, type Completion, type CompletionContext, type CompletionSource } from '@codemirror/autocomplete'
import { javascript } from '@codemirror/lang-javascript'
import { sql as sqlLang } from '@codemirror/lang-sql'
import { Compartment, EditorState, Prec } from '@codemirror/state'
import { EditorView, keymap, placeholder as placeholderExt, type KeyBinding } from '@codemirror/view'
import { basicSetup, minimalSetup } from 'codemirror'

const OPERATORS = [
  '$eq', '$ne', '$gt', '$gte', '$lt', '$lte', '$in', '$nin', '$and', '$or', '$nor', '$not',
  '$exists', '$type', '$regex', '$options', '$elemMatch', '$size', '$all', '$expr', '$text', '$search',
  '$set', '$unset', '$inc', '$push', '$pull', '$addToSet', '$rename', '$mul', '$min', '$max',
  '$match', '$project', '$group', '$sort', '$limit', '$skip', '$unwind', '$lookup', '$addFields',
  '$count', '$facet', '$sum', '$avg', '$first', '$last', '$cond', '$ifNull', '$concat', '$toString',
  '$dateToString', '$meta',
]
const HELPERS = ['ObjectId("")', 'ISODate("")', 'NumberLong()', 'NumberInt()', 'NumberDecimal("")', 'UUID("")']

const theme = EditorView.theme({
  '&': {
    fontSize: 'var(--editor-font-size, 13px)',
    backgroundColor: 'var(--editor-bg)',
    color: 'var(--text)',
    border: '1px solid var(--border)',
    borderRadius: '6px',
  },
  '&.cm-focused': { outline: 'none', borderColor: 'var(--accent)' },
  '.cm-content': { fontFamily: 'var(--font-mono)', caretColor: 'var(--accent)', padding: '6px 0' },
  '.cm-gutters': {
    backgroundColor: 'var(--editor-bg)',
    color: 'var(--text-faint)',
    border: 'none',
    borderRadius: '6px 0 0 6px',
  },
  '.cm-activeLine': { backgroundColor: 'var(--editor-active-line)' },
  '.cm-activeLineGutter': { backgroundColor: 'transparent', color: 'var(--text-muted)' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection': {
    backgroundColor: 'var(--selection) !important',
  },
  '.cm-placeholder': { color: 'var(--text-faint)' },
  '.cm-tooltip': { backgroundColor: 'var(--surface-2)', border: '1px solid var(--border)', color: 'var(--text)' },
  '.cm-tooltip-autocomplete > ul > li[aria-selected]': { backgroundColor: 'var(--accent)', color: 'white' },
  '.cm-scroller': { overflow: 'auto' },
})

export interface CodeEditorProps {
  value: string
  onChange?: (v: string) => void
  onRun?: () => void
  placeholder?: string
  /** Field names offered by autocomplete. */
  fields?: string[]
  minHeight?: number
  maxHeight?: number
  compact?: boolean
  readOnly?: boolean
  ariaLabel?: string
  editorRef?: RefObject<EditorView | null>
  autoFocus?: boolean
  language?: 'javascript' | 'sql'
  /** Replaces the default operator/field completion (e.g. the shell's context-aware source). */
  completionSource?: CompletionSource
  /** Extra key bindings, checked before the defaults. */
  keyBindings?: KeyBinding[]
}

export function CodeEditor({
  value,
  onChange,
  onRun,
  placeholder,
  fields,
  minHeight,
  maxHeight,
  compact,
  readOnly,
  ariaLabel,
  editorRef,
  autoFocus,
  language = 'javascript',
  completionSource,
  keyBindings,
}: CodeEditorProps) {
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const onChangeRef = useRef(onChange)
  const onRunRef = useRef(onRun)
  const fieldsRef = useRef<string[]>(fields ?? [])
  const sourceRef = useRef(completionSource)
  const keysRef = useRef(keyBindings)
  sourceRef.current = completionSource
  keysRef.current = keyBindings
  const readOnlyComp = useRef(new Compartment())
  onChangeRef.current = onChange
  onRunRef.current = onRun
  fieldsRef.current = fields ?? []

  useEffect(() => {
    if (!host.current) return
    const complete = (ctx: CompletionContext) => {
      if (sourceRef.current) return sourceRef.current(ctx)
      const word = ctx.matchBefore(/[$\w.]+/)
      if (!word || (word.from === word.to && !ctx.explicit)) return null
      const options: Completion[] = [
        ...OPERATORS.map((o) => ({ label: o, type: 'keyword' })),
        ...HELPERS.map((h) => ({ label: h, type: 'function' })),
        ...fieldsRef.current.map((f) => ({ label: f, type: 'property' })),
      ]
      return { from: word.from, options, validFor: /^[$\w.]*$/ }
    }
    const v = new EditorView({
      parent: host.current,
      state: EditorState.create({
        doc: value,
        extensions: [
          compact ? minimalSetup : basicSetup,
          language === 'sql' ? sqlLang() : javascript(),
          theme,
          EditorView.lineWrapping,
          language === 'sql' ? [] : autocompletion({ override: [complete], activateOnTyping: true }),
          Prec.highest(
            keymap.of([
              // Delegate to the latest keyBindings prop without recreating the editor.
              ...(keysRef.current ?? []).map((b) => ({
                ...b,
                run: (v: EditorView) => keysRef.current?.find((k) => k.key === b.key)?.run?.(v) ?? false,
              })),
              {
                key: 'Mod-Enter',
                run: () => {
                  onRunRef.current?.()
                  return true
                },
              },
            ]),
          ),
          placeholder ? placeholderExt(placeholder) : [],
          readOnlyComp.current.of(EditorState.readOnly.of(!!readOnly)),
          EditorView.contentAttributes.of(ariaLabel ? { 'aria-label': ariaLabel } : {}),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) onChangeRef.current?.(u.state.doc.toString())
          }),
          EditorView.theme({
            '&': {
              ...(minHeight ? { minHeight: `${minHeight}px` } : {}),
              ...(maxHeight ? { maxHeight: `${maxHeight}px` } : {}),
            },
            '.cm-scroller': minHeight ? { minHeight: `${minHeight}px` } : {},
          }),
        ],
      }),
    })
    view.current = v
    if (editorRef) editorRef.current = v
    if (autoFocus) v.focus()
    return () => {
      v.destroy()
      view.current = null
      if (editorRef) editorRef.current = null
    }
    // The editor is created once; value/readOnly updates are synced below.
  }, [])

  // Sync external value changes (e.g. from the query builder) without losing undo history.
  useEffect(() => {
    const v = view.current
    if (v && v.state.doc.toString() !== value) {
      v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: value } })
    }
  }, [value])

  useEffect(() => {
    view.current?.dispatch({ effects: readOnlyComp.current.reconfigure(EditorState.readOnly.of(!!readOnly)) })
  }, [readOnly])

  return <div className={compact ? 'code-editor compact' : 'code-editor'} ref={host} />
}

/** Returns the selected text, or the whole document when nothing is selected. */
export function selectionOrAll(v: EditorView | null): string {
  if (!v) return ''
  const sel = v.state.selection.main
  return sel.empty ? v.state.doc.toString() : v.state.sliceDoc(sel.from, sel.to)
}
