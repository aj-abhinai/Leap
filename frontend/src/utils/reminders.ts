import { Phone, MessageCircle, Mail, NotepadText, CalendarClock, CheckCheck } from '@lucide/vue'

export interface ReminderLike {
  type: string
  description?: string | null
  scheduled_at?: string | null
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
    return `Scheduled for ${new Date(r.scheduled_at).toLocaleString()}`
  }
  if (r.remind_at) {
    return `Reminder at ${new Date(r.remind_at).toLocaleString()}`
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

export function snoozeRemindAt(minutes: number): string {
  return new Date(Date.now() + minutes * 60_000).toISOString()
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
