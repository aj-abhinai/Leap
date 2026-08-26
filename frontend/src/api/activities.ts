import { apiClient, type ApiResponse } from '@/composables/useApi'

// Global activity list (the Activities page) — a read-only aggregation
// across leads, distinct from the per-lead activities in api/leads.ts.

export interface ActivityListItem {
  id: string
  lead_id: string
  contact_id: string
  lead_display_name: string
  stage_id: string
  stage_name?: string
  user_id?: string
  user_name?: string
  type: string
  description: string
  quick_reply_id?: string
  quick_reply_name?: string
  scheduled_at?: string
  remind_at?: string
  responded_at?: string
  occurred_at?: string
  is_done: boolean
  is_cancelled: boolean
  is_reminded: boolean
  created_at: string
}

export interface ActivityListFilters {
  status?: string
  overdue?: string
  mine?: string
  type?: string
  q?: string
  from?: string
  to?: string
  sort?: string
  order?: string
}

export function listActivities(params: {
  page: number
  perPage: number
  filters: ActivityListFilters
}): Promise<ApiResponse<ActivityListItem[]>> {
  const p = new URLSearchParams()
  const f = params.filters
  if (f.status) p.set('status', f.status)
  if (f.overdue) p.set('overdue', f.overdue)
  if (f.mine) p.set('mine', f.mine)
  if (f.type) p.set('type', f.type)
  if (f.q) p.set('q', f.q)
  if (f.from) p.set('from', f.from)
  if (f.to) p.set('to', f.to)
  if (f.sort) p.set('sort', f.sort)
  if (f.order) p.set('order', f.order)
  p.set('page', String(params.page))
  p.set('per_page', String(params.perPage))
  return apiClient.get(`/api/activities?${p}`)
}