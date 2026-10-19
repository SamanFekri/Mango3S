import type { ReactNode } from 'react'
import { go } from '@codemirror/lang-go'
import { java } from '@codemirror/lang-java'
import { javascript } from '@codemirror/lang-javascript'
import { python } from '@codemirror/lang-python'
import { sql } from '@codemirror/lang-sql'
import { StreamLanguage, syntaxHighlighting, type LanguageSupport, type Language } from '@codemirror/language'
import { csharp } from '@codemirror/legacy-modes/mode/clike'
import { highlightTree, tagHighlighter, tags as t } from '@lezer/highlight'

/**
 * One syntax palette for every editor and every read-only code block, as CSS
 * classes (`.hl-*` in styles.css) so light and dark themes both apply.
 * Strings are orange: easy to read on both backgrounds and distinct from
 * keywords (purple), numbers (blue) and properties (teal).
 */
export const codeHighlighter = tagHighlighter([
  { tag: [t.keyword, t.controlKeyword, t.operatorKeyword, t.definitionKeyword, t.moduleKeyword, t.modifier, t.self], class: 'hl-keyword' },
  { tag: [t.string, t.special(t.string), t.character, t.docString], class: 'hl-string' },
  { tag: [t.regexp, t.escape], class: 'hl-regexp' },
  { tag: [t.number, t.integer, t.float], class: 'hl-number' },
  { tag: [t.bool, t.null, t.atom, t.unit], class: 'hl-atom' },
  { tag: [t.propertyName, t.attributeName, t.labelName], class: 'hl-property' },
  { tag: [t.function(t.variableName), t.function(t.propertyName), t.macroName], class: 'hl-function' },
  { tag: [t.typeName, t.className, t.namespace, t.standard(t.variableName)], class: 'hl-type' },
  { tag: [t.definition(t.variableName), t.definition(t.propertyName)], class: 'hl-definition' },
  { tag: [t.comment, t.lineComment, t.blockComment, t.docComment], class: 'hl-comment' },
  { tag: [t.meta, t.annotation, t.processingInstruction], class: 'hl-meta' },
  { tag: [t.operator, t.punctuation, t.separator, t.bracket, t.derefOperator], class: 'hl-punct' },
  { tag: t.invalid, class: 'hl-invalid' },
])

/** The editor extension: our palette instead of CodeMirror's default (red strings). */
export const codeHighlighting = syntaxHighlighting(codeHighlighter)

export type CodeLang = 'javascript' | 'mongosh' | 'python' | 'java' | 'csharp' | 'go' | 'sql' | 'json'

const supports: Partial<Record<CodeLang, () => LanguageSupport | Language>> = {
  javascript: () => javascript(),
  mongosh: () => javascript(),
  json: () => javascript(), // JSON / Extended JSON / shell syntax are JavaScript-shaped
  python: () => python(),
  java: () => java(),
  go: () => go(),
  sql: () => sql(),
  csharp: () => StreamLanguage.define(csharp),
}

const cache = new Map<CodeLang, Language>()

/** The CodeMirror language for a code-generation target or editor mode. */
export function languageFor(lang: string): Language | null {
  const key = (lang === 'node' ? 'javascript' : lang) as CodeLang
  const cached = cache.get(key)
  if (cached) return cached
  const make = supports[key]
  if (!make) return null
  const made = make()
  const language = 'language' in made ? made.language : made
  cache.set(key, language)
  return language
}

/** Highlights code into React nodes (no editor instance) — for read-only blocks. */
export function highlightCode(code: string, lang: string): ReactNode[] {
  const language = languageFor(lang)
  if (!language || code.length > 200_000) return [code]
  const tree = language.parser.parse(code)
  const out: ReactNode[] = []
  let pos = 0
  highlightTree(tree, codeHighlighter, (from, to, classes) => {
    if (from > pos) out.push(code.slice(pos, from))
    out.push(
      <span key={out.length} className={classes}>
        {code.slice(from, to)}
      </span>,
    )
    pos = to
  })
  if (pos < code.length) out.push(code.slice(pos))
  return out
}

/** A read-only, highlighted code block. */
export function CodeBlock({ code, lang, className = 'code-output', label }: { code: string; lang: string; className?: string; label?: string }) {
  return (
    <pre className={`${className} hl-code`} aria-label={label} data-lang={lang}>
      <code>{highlightCode(code, lang)}</code>
    </pre>
  )
}
