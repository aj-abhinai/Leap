export function timeAgo(dateStr: string): string {
  if (!dateStr) return ''
  const then = new Date(dateStr).getTime()
  if (isNaN(then)) return ''
  const now = Date.now()
  const diff = now - then
  // Future times read "in 5m" / "in 2h" / "in 3d"; the Dashboard reminder rows
  // pass nudge times, which are future by definition. A sub-minute future stamp
  // is clock skew on a server-stamped "now", so it keeps the "just now" reading.
  if (diff < 0) {
    if (diff > -60_000) return 'just now'
    const ahead = Math.ceil(-diff / 60000)
    if (ahead < 60) return `in ${Math.max(1, ahead)}m`
    const hours = Math.floor(ahead / 60)
    if (hours < 24) return `in ${hours}h`
    return `in ${Math.floor(hours / 24)}d`
  }
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

// mergeDateTime combines a local wall-clock date and time into a UTC ISO
// instant, or null when either part is empty.
export function mergeDateTime(date: string, time: string): string | null {
  if (!date || !time) return null
  return new Date(`${date}T${time}`).toISOString()
}

// DatePreset names a created-at window for a board: a local-day preset, an
// explicit from/to range ('custom'), or no window ('all').
export type DatePreset = 'all' | 'today' | 'yesterday' | 'week' | 'month' | 'custom'

interface DayWindow {
  from: string
  to: string
}

// localDay returns the local calendar day containing date, shifted by days.
function localDay(date: Date, days = 0): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days)
}

// presetWindow returns the instant window for a named preset. Boundaries are
// the viewer's local days — the week starts Sunday, the month starts on the
// 1st — and travel as UTC instants, so "today" is the viewer's today wherever
// the server runs.
export function presetWindow(
  preset: Exclude<DatePreset, 'all' | 'custom'>,
  now = new Date(),
): DayWindow {
  const startOfToday = localDay(now)
  const endOfToday = new Date(localDay(now, 1).getTime() - 1)
  switch (preset) {
    case 'yesterday':
      return {
        from: localDay(now, -1).toISOString(),
        to: new Date(startOfToday.getTime() - 1).toISOString(),
      }
    case 'week':
      return {
        from: localDay(now, -now.getDay()).toISOString(),
        to: endOfToday.toISOString(),
      }
    case 'month':
      return {
        from: new Date(now.getFullYear(), now.getMonth(), 1).toISOString(),
        to: endOfToday.toISOString(),
      }
    case 'today':
      return { from: startOfToday.toISOString(), to: endOfToday.toISOString() }
  }
}

// dayWindow returns one local calendar day ("YYYY-MM-DD") as an instant
// window ending on its last millisecond, or null when the day is malformed.
export function dayWindow(day: string): DayWindow | null {
  const parts = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day)
  if (!parts) return null
  const year = Number(parts[1])
  const month = Number(parts[2])
  const date = Number(parts[3])
  const start = new Date(year, month - 1, date)
  // Date rolls impossible days over (Feb 31 → Mar 3); reject those instead.
  if (start.getFullYear() !== year || start.getMonth() !== month - 1 || start.getDate() !== date) {
    return null
  }
  const end = new Date(year, month - 1, date + 1)
  return { from: start.toISOString(), to: new Date(end.getTime() - 1).toISOString() }
}

// AllDayRange is the UTC window of a local calendar day plus its 09:00 nudge.
export interface AllDayRange {
  start: string
  end: string
  remind: string
}

// allDayRange turns a "YYYY-MM-DD" day into the local day it spans: the first
// and last millisecond, with the nudge at 09:00 local. dayWindow owns the
// malformed and impossible-day checks.
export function allDayRange(date: string): AllDayRange | null {
  const w = dayWindow(date)
  if (!w) return null
  const remind = new Date(w.from)
  remind.setHours(9, 0, 0, 0)
  return { start: w.from, end: w.to, remind: remind.toISOString() }
}

// isAllDayRange reports whether a schedule is one local day: 00:00:00.000 to
// 23:59:59.999 on the same local date.
export function isAllDayRange(startIso?: string | null, endIso?: string | null): boolean {
  if (!startIso || !endIso) return false
  const s = new Date(startIso)
  const e = new Date(endIso)
  if (isNaN(s.getTime()) || isNaN(e.getTime())) return false
  return (
    s.getHours() === 0 && s.getMinutes() === 0 && s.getSeconds() === 0 && s.getMilliseconds() === 0 &&
    e.getFullYear() === s.getFullYear() && e.getMonth() === s.getMonth() && e.getDate() === s.getDate() &&
    e.getHours() === 23 && e.getMinutes() === 59 && e.getSeconds() === 59 && e.getMilliseconds() === 999
  )
}

// dateWindow resolves the window for a preset. null means no window: 'all'
// dates, or a custom range that is not complete yet.
export function dateWindow(
  preset: DatePreset,
  from: string,
  to: string,
  now = new Date(),
): DayWindow | null {
  if (preset === 'all') return null
  if (preset === 'custom') {
    const start = dayWindow(from)
    const end = dayWindow(to)
    return start && end ? { from: start.from, to: end.to } : null
  }
  return presetWindow(preset, now)
}
