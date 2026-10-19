// Explorer icons: small, crisp SVGs that inherit color from CSS
// (.icon-db, .icon-coll, …) so they follow the light and dark themes.
import type { SVGProps } from 'react'

const base = (props: SVGProps<SVGSVGElement>) => ({
  width: 16,
  height: 16,
  viewBox: '0 0 16 16',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.4,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
  'aria-hidden': true,
  focusable: false,
  ...props,
})

/** A server / connection. */
export function ServerIcon({ live, ...p }: SVGProps<SVGSVGElement> & { live?: boolean }) {
  return (
    <svg {...base(p)} className={`icon icon-server${live ? ' live' : ''}`}>
      <rect x="2" y="2.5" width="12" height="4.5" rx="1.2" />
      <rect x="2" y="9" width="12" height="4.5" rx="1.2" />
      <circle cx="4.6" cy="4.75" r="0.6" fill="currentColor" stroke="none" />
      <circle cx="4.6" cy="11.25" r="0.6" fill="currentColor" stroke="none" />
      <path d="M8 4.75h4M8 11.25h4" />
    </svg>
  )
}

/** A database: a cylinder. */
export function DatabaseIcon(p: SVGProps<SVGSVGElement>) {
  return (
    <svg {...base(p)} className="icon icon-db">
      <ellipse cx="8" cy="3.6" rx="5.2" ry="1.9" fill="currentColor" fillOpacity="0.18" />
      <path d="M2.8 3.6v8.8c0 1.05 2.33 1.9 5.2 1.9s5.2-.85 5.2-1.9V3.6" />
      <path d="M2.8 8c0 1.05 2.33 1.9 5.2 1.9s5.2-.85 5.2-1.9" />
    </svg>
  )
}

export type CollectionKind = 'collection' | 'view' | 'system' | 'timeseries'

/** Collections (a document grid), views (an eye), system collections (a gear) and time series (a clock). */
export function CollectionIcon({ kind, ...p }: SVGProps<SVGSVGElement> & { kind: CollectionKind }) {
  switch (kind) {
    case 'view':
      return (
        <svg {...base(p)} className="icon icon-view">
          <path d="M1.5 8s2.4-4.3 6.5-4.3S14.5 8 14.5 8 12.1 12.3 8 12.3 1.5 8 1.5 8Z" />
          <circle cx="8" cy="8" r="2" fill="currentColor" fillOpacity="0.25" />
        </svg>
      )
    case 'system':
      return (
        <svg {...base(p)} className="icon icon-system">
          <circle cx="8" cy="8" r="2.2" />
          <path d="M8 1.8v1.7M8 12.5v1.7M14.2 8h-1.7M3.5 8H1.8M12.4 3.6l-1.2 1.2M4.8 11.2l-1.2 1.2M12.4 12.4l-1.2-1.2M4.8 4.8 3.6 3.6" />
        </svg>
      )
    case 'timeseries':
      return (
        <svg {...base(p)} className="icon icon-timeseries">
          <circle cx="8" cy="8" r="6" />
          <path d="M8 4.5V8l2.4 1.6" />
        </svg>
      )
    default:
      return (
        <svg {...base(p)} className="icon icon-coll">
          <rect x="2" y="2.5" width="12" height="11" rx="1.5" fill="currentColor" fillOpacity="0.12" />
          <path d="M2 6h12M2 9.5h12M6 6v7.5" />
        </svg>
      )
  }
}

/** A folder (explorer groups and connection folders). */
export function FolderIcon({ open, ...p }: SVGProps<SVGSVGElement> & { open?: boolean }) {
  return (
    <svg {...base(p)} className="icon icon-folder">
      {open ? (
        <path d="M1.8 12.8V4.2c0-.6.5-1.1 1.1-1.1h3l1.4 1.5h5.2c.6 0 1.1.5 1.1 1.1V7M1.8 12.8l1.9-5c.2-.5.6-.8 1.1-.8h9.5c.4 0 .7.4.5.8l-1.9 5H1.8Z" />
      ) : (
        <path d="M1.8 12.4V4.2c0-.6.5-1.1 1.1-1.1h3l1.4 1.5h5.8c.6 0 1.1.5 1.1 1.1v6.7c0 .6-.5 1.1-1.1 1.1H2.9c-.6 0-1.1-.5-1.1-1.1Z" fill="currentColor" fillOpacity="0.15" />
      )}
    </svg>
  )
}

const TYPE_GLYPHS: Record<string, { glyph: string; kind: string; label: string }> = {
  string: { glyph: 'ab', kind: 'str', label: 'String' },
  int32: { glyph: '32', kind: 'num', label: 'Int32' },
  int64: { glyph: '64', kind: 'num', label: 'Int64 (long)' },
  double: { glyph: '1.5', kind: 'num', label: 'Double' },
  decimal: { glyph: 'd', kind: 'num', label: 'Decimal128' },
  boolean: { glyph: 't/f', kind: 'bool', label: 'Boolean' },
  date: { glyph: 'dt', kind: 'date', label: 'Date' },
  timestamp: { glyph: 'ts', kind: 'date', label: 'Timestamp' },
  objectId: { glyph: 'id', kind: 'oid', label: 'ObjectId' },
  document: { glyph: '{}', kind: 'obj', label: 'Document' },
  array: { glyph: '[]', kind: 'obj', label: 'Array' },
  null: { glyph: '∅', kind: 'null', label: 'Null' },
  undefined: { glyph: '∅', kind: 'null', label: 'Undefined' },
  binary: { glyph: '01', kind: 'bin', label: 'Binary' },
  regex: { glyph: '.*', kind: 'bin', label: 'Regular expression' },
  javascript: { glyph: 'js', kind: 'bin', label: 'JavaScript' },
  minKey: { glyph: 'min', kind: 'null', label: 'MinKey' },
  maxKey: { glyph: 'max', kind: 'null', label: 'MaxKey' },
}

/** A small badge for a BSON type (Table and Tree views). */
export function TypeIcon({ type }: { type: string }) {
  const t = TYPE_GLYPHS[type]
  if (!t) return null
  return (
    <span className={`type-icon type-${t.kind}`} title={t.label} aria-label={t.label} role="img">
      {t.glyph}
    </span>
  )
}

/** The settings gear in the tab bar. */
export function GearIcon(p: SVGProps<SVGSVGElement>) {
  return (
    <svg {...base({ width: 18, height: 18, viewBox: '0 0 24 24', strokeWidth: 1.8, ...p })} className="icon icon-gear">
      <circle cx="12" cy="12" r="3.2" />
      <path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1.03 1.56V21a2 2 0 1 1-4 0v-.09a1.7 1.7 0 0 0-1.11-1.56 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.7 1.7 0 0 0 .34-1.87 1.7 1.7 0 0 0-1.56-1.03H3a2 2 0 1 1 0-4h.09a1.7 1.7 0 0 0 1.56-1.11 1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.7 1.7 0 0 0 1.87.34H9a1.7 1.7 0 0 0 1.03-1.56V3a2 2 0 1 1 4 0v.09a1.7 1.7 0 0 0 1.03 1.56 1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.7 1.7 0 0 0-.34 1.87V9c.26.6.86 1 1.51 1H21a2 2 0 1 1 0 4h-.09c-.65 0-1.25.4-1.51 1Z" />
    </svg>
  )
}

/** An open eye: "show the password". */
export function EyeIcon(p: SVGProps<SVGSVGElement>) {
  return (
    <svg {...base(p)} className="icon icon-eye">
      <path d="M1.5 8s2.4-4.5 6.5-4.5S14.5 8 14.5 8s-2.4 4.5-6.5 4.5S1.5 8 1.5 8z" />
      <circle cx="8" cy="8" r="2" />
    </svg>
  )
}

/** A struck-through eye: "hide the password". */
export function EyeOffIcon(p: SVGProps<SVGSVGElement>) {
  return (
    <svg {...base(p)} className="icon icon-eye">
      <path d="M6.2 3.8A6.6 6.6 0 0 1 8 3.5c4.1 0 6.5 4.5 6.5 4.5a11 11 0 0 1-1.9 2.4M10.1 10.6A2 2 0 0 1 6 8a2 2 0 0 1 .6-1.4M4.2 4.9C2.4 6.1 1.5 8 1.5 8s2.4 4.5 6.5 4.5c1.4 0 2.6-.5 3.6-1.2" />
      <path d="M2 2l12 12" />
    </svg>
  )
}
