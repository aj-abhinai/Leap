import { apiClient, type ApiResponse } from '@/composables/useApi'

// Org settings (GET/PUT /api/settings/*). The nudge lead time is the minutes
// before a task's start time that its reminder fires.

export function getNudgeLeadMinutes(): Promise<ApiResponse<{ minutes: number }>> {
  return apiClient.get('/api/settings/nudge-lead-minutes')
}

export function setNudgeLeadMinutes(minutes: number): Promise<ApiResponse<{ minutes: number }>> {
  return apiClient.put('/api/settings/nudge-lead-minutes', { minutes })
}
