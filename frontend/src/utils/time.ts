export function timeAgo(dateStr: string): string {
  if (!dateStr) return ''
  const then = new Date(dateStr).getTime()
  if (isNaN(then)) return ''
  const now = Date.now()
  const diff = now - then
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

export function formatDateTime(date: string): string {
  return new Date(date).toLocaleString()
}

export function formatDate(date: string): string {
  return new Date(date).toLocaleDateString()
}

// Local wall-clock components for <input type="date"> / <input type="time">,
// matching the local-time semantics of mergeDateTime (local wall-clock in,
// UTC instant out).
export function toLocalDateInput(iso: string): string {
  const d = new Date(iso)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function toLocalTimeInput(iso: string): string {
  const d = new Date(iso)
  const h = String(d.getHours()).padStart(2, '0')
  const min = String(d.getMinutes()).padStart(2, '0')
  return `${h}:${min}`
}
