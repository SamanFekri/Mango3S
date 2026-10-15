export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return '—'
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB', 'TB', 'PB']
  let v = n
  let i = -1
  do {
    v /= 1024
    i++
  } while (v >= 1024 && i < units.length - 1)
  return `${v.toFixed(v < 10 ? 2 : v < 100 ? 1 : 0)} ${units[i]}`
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms} ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)} s`
  const m = Math.floor(ms / 60_000)
  const s = Math.round((ms % 60_000) / 1000)
  return `${m}m ${s}s`
}

export function formatCount(n: number): string {
  return new Intl.NumberFormat('en-US').format(n)
}

/** "Documents 51–100" style range for a results page. */
export function pageRange(page: number, pageSize: number, count: number): string {
  if (count === 0) return 'No documents'
  const start = page * pageSize + 1
  return `${formatCount(start)}–${formatCount(start + count - 1)}`
}
