import { apiClient, type ApiResponse } from '@/composables/useApi'

// Lead entities and the lead-scoped endpoints (leads, their activities, and
// stage history). Reminders on a lead live in api/reminders.ts.

export interface Lead {
  id: string
  nickname?: string
  display_name: string
  contact_id: string
  contact_name?: string
  contact_phone?: string
  contact_email?: string
  pipeline_id: string
  stage_id: string
  stage_name?: string
  stage_outcome?: 'open' | 'won' | 'lost'
  outcome?: string
  lost_reason?: string
  program_id?: string
  program_name?: string
  value?: number
  notes?: string
  assigned_to?: string
  created_at: string
  updated_at: string
  next_task_type?: string
  next_task_at?: string
  last_touch_type?: string
  last_touch_at?: string
}

export interface LeadListQuery {
  pipelineId?: string
  stageId?: string
  q?: string
  outcome?: 'open' | 'won' | 'lost'
  assignedTo?: string
}

export interface LeadActivity {
  id: string
  lead_id: string
  type: string
  description: string
  quick_reply_id?: string
  quick_reply_name?: string
  scheduled_at?: string
  scheduled_end_at?: string
  remind_at?: string
  occurred_at?: string
  responded_at?: string
  is_done: boolean
  is_cancelled: boolean
  is_reminded: boolean
  user_name?: string
  created_at: string
}

export interface StageHistoryEntry {
  id: string
  from_stage_name?: string
  to_stage_name?: string
  moved_at: string
}

export function listLeads(params: {
  page: number
  perPage: number
  query?: LeadListQuery
}): Promise<ApiResponse<Lead[]>> {
  const p = new URLSearchParams({ page: String(params.page), per_page: String(params.perPage) })
  const q = params.query
  if (q?.pipelineId) p.set('pipeline_id', q.pipelineId)
  if (q?.stageId) p.set('stage_id', q.stageId)
  if (q?.q) p.set('q', q.q)
  if (q?.outcome) p.set('outcome', q.outcome)
  if (q?.assignedTo) p.set('assigned_to', q.assignedTo)
  return apiClient.get(`/api/leads?${p}`)
}

// BoardStage is one kanban column: the newest-window leads plus the true
// count of matching leads in the stage.
export interface BoardStage {
  stage_id: string
  count: number
  leads: Lead[]
}

// Board is the kanban payload (GET /api/leads/board).
export interface Board {
  stages: BoardStage[]
}

export function fetchBoard(params: {
  pipelineId: string
  q?: string
  outcome?: string
  assignedTo?: string
  from?: string
  to?: string
}): Promise<ApiResponse<Board>> {
  const p = new URLSearchParams({ pipeline_id: params.pipelineId })
  if (params.q) p.set('q', params.q)
  if (params.outcome) p.set('outcome', params.outcome)
  if (params.assignedTo) p.set('assigned_to', params.assignedTo)
  if (params.from) p.set('from', params.from)
  if (params.to) p.set('to', params.to)
  return apiClient.get(`/api/leads/board?${p}`)
}

export function getLead(id: string): Promise<ApiResponse<Lead>> {
  return apiClient.get(`/api/leads/${id}`)
}

export type LeadSaveBody = Record<string, any>

export function createLead(body: LeadSaveBody): Promise<ApiResponse<Lead>> {
  return apiClient.post('/api/leads', body)
}

export function updateLead(id: string, body: LeadSaveBody): Promise<ApiResponse<Lead>> {
  return apiClient.patch(`/api/leads/${id}`, body)
}

export function deleteLead(id: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/leads/${id}`)
}

export function listLeadActivities(leadId: string): Promise<ApiResponse<LeadActivity[]>> {
  return apiClient.get(`/api/leads/${leadId}/activities`)
}

export function createLeadActivity(leadId: string, body: Record<string, unknown>): Promise<ApiResponse<LeadActivity>> {
  return apiClient.post(`/api/leads/${leadId}/activities`, body)
}

export function updateLeadActivity(
  leadId: string,
  activityId: string,
  body: Record<string, unknown>,
): Promise<ApiResponse<LeadActivity>> {
  return apiClient.patch(`/api/leads/${leadId}/activities/${activityId}`, body)
}

export function deleteLeadActivity(leadId: string, activityId: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/leads/${leadId}/activities/${activityId}`)
}

export function listLeadHistory(leadId: string): Promise<ApiResponse<StageHistoryEntry[]>> {
  return apiClient.get(`/api/leads/${leadId}/history`)
}

export function listLeadsByContact(contactId: string): Promise<ApiResponse<Lead[]>> {
  return apiClient.get(`/api/leads?contact_id=${encodeURIComponent(contactId)}`)
}