import { Phone, MessageCircle, NotepadText } from '@lucide/vue'

export interface ReminderLike {
  type: string
  description?: string | null
  scheduled_at?: string | null
  remind_at?: string | null
}

export function reminderIcon(type: string) {
  switch (type) {
    case 'call_scheduled':
    case 'call_rescheduled':
      return Phone
    case 'wa_message':
      return MessageCircle
    default:
      return NotepadText
  }
}

export function formatReminderText(r: ReminderLike): string {
  switch (r.type) {
    case 'call_scheduled': return `Call scheduled: ${r.description || 'Follow-up call'}`
    case 'call_rescheduled': return `Call rescheduled: ${r.description || 'Follow-up call'}`
    case 'wa_message': return `WhatsApp: ${r.description || 'Send message'}`
    default: return r.description || 'Reminder'
  }
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
