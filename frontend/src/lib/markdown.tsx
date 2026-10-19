import type { ReactNode } from 'react'

/**
 * A small, safe Markdown renderer for server-provided text (the privacy
 * policy): headings, paragraphs, bullet and numbered lists, **bold**,
 * *italic*, `code` and [links](https://…). It builds React elements, never
 * HTML, so nothing in the text can run as markup. Links are handed to
 * onLink (they open in the browser) and only http(s) links are kept.
 */
export function renderMarkdown(text: string, onLink: (url: string) => void): ReactNode[] {
  const lines = text.replace(/\r\n?/g, '\n').split('\n')
  const out: ReactNode[] = []
  let para: string[] = []
  let list: { ordered: boolean; items: string[] } | null = null

  const flushPara = () => {
    if (para.length) out.push(<p key={out.length}>{inline(para.join(' '), onLink)}</p>)
    para = []
  }
  const flushList = () => {
    if (!list) return
    const items = list.items.map((it, i) => <li key={i}>{inline(it, onLink)}</li>)
    out.push(list.ordered ? <ol key={out.length}>{items}</ol> : <ul key={out.length}>{items}</ul>)
    list = null
  }

  for (const raw of lines) {
    const line = raw.trimEnd()
    const heading = /^(#{1,6})\s+(.*)$/.exec(line)
    const bullet = /^\s*[-*+]\s+(.*)$/.exec(line)
    const numbered = /^\s*\d+[.)]\s+(.*)$/.exec(line)
    if (!line.trim()) {
      flushPara()
      flushList()
    } else if (heading) {
      flushPara()
      flushList()
      const level = Math.min(heading[1]!.length + 2, 6) // inside a dialog: ## → h4
      const Tag = `h${level}` as 'h3'
      out.push(<Tag key={out.length}>{inline(heading[2]!, onLink)}</Tag>)
    } else if (bullet || numbered) {
      flushPara()
      const ordered = !!numbered
      if (list && list.ordered !== ordered) flushList()
      list ??= { ordered, items: [] }
      list.items.push((bullet ?? numbered)![1]!)
    } else if (list && /^\s{2,}\S/.test(raw)) {
      list.items[list.items.length - 1] += ' ' + line.trim() // continuation of a list item
    } else {
      flushList()
      para.push(line.trim())
    }
  }
  flushPara()
  flushList()
  return out
}

const INLINE = /(\*\*[^*]+\*\*|__[^_]+__|\*[^*\s][^*]*\*|_[^_\s][^_]*_|`[^`]+`|\[[^\]]+\]\([^)\s]+\))/g

function inline(text: string, onLink: (url: string) => void): ReactNode[] {
  const parts: ReactNode[] = []
  let last = 0
  for (const m of text.matchAll(INLINE)) {
    if (m.index! > last) parts.push(text.slice(last, m.index))
    const tok = m[0]
    const key = parts.length
    if (tok.startsWith('**') || tok.startsWith('__')) parts.push(<strong key={key}>{tok.slice(2, -2)}</strong>)
    else if (tok.startsWith('`')) parts.push(<code key={key}>{tok.slice(1, -1)}</code>)
    else if (tok.startsWith('[')) {
      const link = /^\[([^\]]+)\]\(([^)\s]+)\)$/.exec(tok)!
      const url = link[2]!
      parts.push(
        /^https?:\/\//i.test(url) ? (
          <button key={key} type="button" className="link" onClick={() => onLink(url)}>
            {link[1]}
          </button>
        ) : (
          link[1]
        ),
      )
    } else parts.push(<em key={key}>{tok.slice(1, -1)}</em>)
    last = m.index! + tok.length
  }
  if (last < text.length) parts.push(text.slice(last))
  return parts
}
