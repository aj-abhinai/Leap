import { Phone, MessageCircle, Mail, NotepadText, CalendarClock, CheckCheck } from '@lucide/vue'
import { formatDate, formatDateTime, isAllDayRange, toLocalDateInput, toLocalTimeInput } from './time'

export interface ReminderLike {
  type: string
  description?: string | null
  scheduled_at?: string | null
  scheduled_end_at?: string | null
  remind_at?: string | null
}

// Icons are keyed on the activity's free-text type (a configured activity_type
// tag), matching how the kanban and timeline label tasks.
export function reminderIcon(type: string) {
  const t = (type || '').toLowerCase()
  if (t.includes('call')) return Phone
  if (t.includes('whatsapp') || t.includes('wa ') || t.includes('message')) return MessageCircle
  if (t.includes('mail') || t.includes('email')) return Mail
  if (t.includes('meeting') || t.includes('visit')) return CalendarClock
  if (t.includes('follow')) return CheckCheck
  return NotepadText
}

export function formatReminderText(r: ReminderLike): string {
  const label = r.type || 'Task'
  if (r.description) return `${label}: ${r.description}`
  return label
}

export function formatReminderTime(r: ReminderLike): string {
  if (r.scheduled_at) {
    if (isAllDayRange(r.scheduled_at, r.scheduled_end_at)) {
      return `Scheduled for ${formatDate(r.scheduled_at)} (all day)`
    }
    if (r.scheduled_end_at) {
      return `Scheduled for ${formatDateTime(r.scheduled_at)} – ${formatDateTime(r.scheduled_end_at)}`
    }
    return `Scheduled for ${formatDateTime(r.scheduled_at)}`
  }
  if (r.remind_at) {
    return `Reminder at ${formatDateTime(r.remind_at)}`
  }
  return ''
}

export interface SnoozePreset {
  label: string
  minutes: number
}

export const snoozePresets: SnoozePreset[] = [
  { label: '15 minutes', minutes: 15 },
  { label: '1 hour', minutes: 60 },
  { label: '3 hours', minutes: 180 },
  { label: 'Tomorrow', minutes: 24 * 60 },
]

// An all-day task keeps its day grid: its snooze presets move the nudge by
// whole days, so the window (midnight to 23:59:59.999) survives the shift.
export const allDaySnoozePresets: SnoozePreset[] = [
  { label: 'Tomorrow', minutes: 24 * 60 },
  { label: 'In 2 days', minutes: 2 * 24 * 60 },
  { label: 'Next week', minutes: 7 * 24 * 60 },
]

export function snoozeRemindAt(minutes: number): string {
  return new Date(Date.now() + minutes * 60_000).toISOString()
}

// snoozeTarget resolves the new remind_at for a snooze tap. A timed task moves
// to now + minutes. An all-day task keeps its day grid: the target is the
// task's nudge time-of-day on the preset day from today, so the window shift
// stays a whole number of days and the target is always in the future, even
// when the task is overdue.
export function snoozeTarget(r: ReminderLike, minutes: number): string {
  if (r.remind_at && isAllDayRange(r.scheduled_at, r.scheduled_end_at)) {
    const nudge = new Date(r.remind_at)
    const days = Math.max(1, Math.round(minutes / (24 * 60)))
    const target = new Date()
    target.setDate(target.getDate() + days)
    target.setHours(nudge.getHours(), nudge.getMinutes(), nudge.getSeconds(), nudge.getMilliseconds())
    return target.toISOString()
  }
  return snoozeRemindAt(minutes)
}

// One-tap follow-up slots for "log attempt + next" quick replies (Busy,
// No reply, ...). Each preset returns a local wall-clock Date; consumers
// convert it with toLocalDateInput/toLocalTimeInput to fill the inputs.
export interface NextPreset {
  label: string
  at: () => Date
}

// Day-based presets anchor on the current wall-clock time (setDate keeps the
// hours/minutes) so they don't drift across DST transitions.
function inDays(days: number): Date {
  const d = new Date()
  d.setDate(d.getDate() + days)
  return d
}

export const nextPresets: NextPreset[] = [
  { label: 'In 2 hours', at: () => new Date(Date.now() + 2 * 60 * 60_000) },
  { label: 'Tomorrow', at: () => inDays(1) },
  { label: 'In 2 days', at: () => inDays(2) },
  { label: 'In 3 days', at: () => inDays(3) },
  { label: 'Next week', at: () => inDays(7) },
]

// groupQuickReplies buckets quick-reply chips by group (defaulting to
// "Quick reply") and sorts each bucket by sort_order, for the ordered
// palette rendered in task forms.
export interface QuickReplyGroup<T> {
  group: string
  items: T[]
}

export function groupQuickReplies<T extends { group_name?: string | null; sort_order: number }>(
  chips: T[],
): QuickReplyGroup<T>[] {
  const groups = new Map<string, T[]>()
  for (const chip of chips) {
    const key = chip.group_name || 'Quick reply'
    const bucket = groups.get(key) ?? []
    bucket.push(chip)
    groups.set(key, bucket)
  }
  return [...groups.entries()].map(([group, items]) => ({
    group,
    items: items.slice().sort((a, b) => a.sort_order - b.sort_order),
  }))
}

// findSelectedPreset highlights the preset that produced the current
// date/time inputs, or '' when none matches.
export function findSelectedPreset(date: string, time: string, presets: NextPreset[]): string {
  if (!date || !time) return ''
  const preset = presets.find((p) => {
    const at = p.at().toISOString()
    return toLocalDateInput(at) === date && toLocalTimeInput(at) === time
  })
  return preset?.label ?? ''
}
