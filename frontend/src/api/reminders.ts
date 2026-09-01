import { apiClient, type ApiResponse } from '@/composables/useApi'

// Reminders: the global pending list plus the lead-scoped dismiss/snooze
// mutations. The reminders store is the single owner of these calls.

export interface Reminder {
  id: string
  lead_id: string
  contact_id?: string
  lead_display_name?: string
  stage_id: string
  stage_name?: string
  user_id?: string
  user_name?: string
  type: string
  description: string
  scheduled_at?: string
  scheduled_end_at?: string
  remind_at?: string
  is_done: boolean
  is_cancelled: boolean
  is_reminded: boolean
  created_at: string
}

export function listReminders(): Promise<ApiResponse<Reminder[]>> {
  return apiClient.get('/api/reminders')
}

export function dismissReminder(leadId: string, reminderId: string): Promise<ApiResponse<null>> {
  return apiClient.patch(`/api/leads/${leadId}/reminders/${reminderId}`)
}

export function snoozeReminder(
  leadId: string,
  reminderId: string,
  remindAt: string,
): Promise<ApiResponse<null>> {
  return apiClient.post(`/api/leads/${leadId}/reminders/${reminderId}/snooze`, { remind_at: remindAt })
}