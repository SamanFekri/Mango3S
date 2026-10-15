import type { ReactNode } from 'react'

// Lightweight tokenizer for read-only shell-syntax documents. Much cheaper
// than mounting an editor per document in long result lists.
const TOKEN =
  /("(?:[^"\\]|\\.)*")(\s*:)?|\b(ObjectId|ISODate|NumberLong|NumberInt|NumberDecimal|UUID|BinData|Timestamp|RegExp|MinKey|MaxKey|Code)\b|\b(true|false|null|undefined|NaN|Infinity)\b|(-?\b\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b)|(\/(?:[^/\\\n]|\\.)+\/[imsxlu]*)/g

export function highlight(text: string): ReactNode[] {
  const out: ReactNode[] = []
  let last = 0
  let m: RegExpExecArray | null
  let i = 0
  TOKEN.lastIndex = 0
  while ((m = TOKEN.exec(text))) {
    if (m.index > last) out.push(text.slice(last, m.index))
    if (m[1] !== undefined) {
      if (m[2] !== undefined) {
        out.push(
          <span key={i++} className="tok-key">
            {m[1]}
          </span>,
          m[2],
        )
      } else {
        out.push(
          <span key={i++} className="tok-str">
            {m[1]}
          </span>,
        )
      }
    } else if (m[3]) {
      out.push(
        <span key={i++} className="tok-fn">
          {m[3]}
        </span>,
      )
    } else if (m[4]) {
      out.push(
        <span key={i++} className="tok-kw">
          {m[4]}
        </span>,
      )
    } else if (m[5]) {
      out.push(
        <span key={i++} className="tok-num">
          {m[5]}
        </span>,
      )
    } else if (m[6]) {
      out.push(
        <span key={i++} className="tok-re">
          {m[6]}
        </span>,
      )
    }
    last = TOKEN.lastIndex
  }
  if (last < text.length) out.push(text.slice(last))
  return out
}
