import { apiClient, type ApiResponse } from '@/composables/useApi'

// Pipelines and their stages — reference data for the kanban plus the
// settings:manage mutations.

export interface Stage {
  id: string
  pipeline_id: string
  name: string
  order: number
  color?: string
  is_closing: boolean
  outcome?: 'open' | 'won' | 'lost'
}

export interface Pipeline {
  id: string
  name: string
  description?: string
  stages?: Stage[]
}

export function listPipelines(): Promise<ApiResponse<Pipeline[]>> {
  return apiClient.get('/api/pipelines')
}

export function createPipeline(body: { name: string; description?: string }): Promise<ApiResponse<Pipeline>> {
  return apiClient.post('/api/pipelines', body)
}

export function updatePipeline(id: string, body: { name: string; description?: string }): Promise<ApiResponse<Pipeline>> {
  return apiClient.patch(`/api/pipelines/${id}`, body)
}

export function deletePipeline(id: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/pipelines/${id}`)
}

export function addStage(pipelineId: string, body: { name: string; order?: number }): Promise<ApiResponse<Stage>> {
  return apiClient.post(`/api/pipelines/${pipelineId}/stages`, body)
}

export function updateStage(
  stageId: string,
  body: { name?: string; order?: number; is_closing?: boolean; outcome?: string },
): Promise<ApiResponse<Stage>> {
  return apiClient.patch(`/api/stages/${stageId}`, body)
}

export function deleteStage(stageId: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/stages/${stageId}`)
}