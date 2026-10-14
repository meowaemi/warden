// Formatting helpers. The backend sends times in UTC; they are shown in the
// viewer's local time zone.

const pad = (n) => String(n).padStart(2, '0')

/** Formats an ISO time in local time as "2026-10-14 20:08:05". */
export function formatTime(iso) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ` +
    `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  )
}

/** Formats how long ago an ISO time was, e.g. "5 min ago". */
export function formatAgo(iso, now = Date.now()) {
  const sec = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 1000))
  if (sec < 60) return `${sec} s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min} min ago`
  const h = Math.floor(min / 60)
  if (h < 24) return `${h} h ${min % 60} min ago`
  const days = Math.floor(h / 24)
  return `${days} d ${h % 24} h ago`
}

/** Severity order for sorting: most severe first. */
export const severityRank = { ERROR: 0, WARNING: 1, INFO: 2 }
