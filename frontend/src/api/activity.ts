import { apiClient, type ApiResponse } from '@/composables/useApi'

// Audit-log entries (the Settings → Activity Log tab).

export interface ActivityEntry {
  id: string
  description: string
  user_id?: string
  user_name?: string
  action: string
  resource_type: string
  resource_id?: string
  changes?: any
  created_at: string
}

export function listAuditLog(params: {
  page: number
  perPage: number
  filters: { action?: string; resourceType?: string }
}): Promise<ApiResponse<ActivityEntry[]>> {
  const p = new URLSearchParams({ page: String(params.page), per_page: String(params.perPage) })
  if (params.filters.action) p.set('action', params.filters.action)
  if (params.filters.resourceType) p.set('resource_type', params.filters.resourceType)
  return apiClient.get(`/api/activity?${p}`)
}