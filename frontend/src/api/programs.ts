import { apiClient, type ApiResponse } from '@/composables/useApi'

// Programs: the active catalog (lead:read) and the settings:manage CRUD with
// archive/restore.

export interface Program {
  id: string
  name: string
  description?: string
  price: number
  archived: boolean
}

export function listPrograms(): Promise<ApiResponse<Program[]>> {
  return apiClient.get('/api/programs')
}

export function listProgramsManage(): Promise<ApiResponse<Program[]>> {
  return apiClient.get('/api/programs/manage')
}

export function createProgram(body: { name: string; description?: string; price: number }): Promise<ApiResponse<Program>> {
  return apiClient.post('/api/programs', body)
}

export function updateProgram(
  id: string,
  body: { name?: string; description?: string; price?: number },
): Promise<ApiResponse<Program>> {
  return apiClient.patch(`/api/programs/${id}`, body)
}

export function archiveProgram(id: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/programs/${id}`)
}

export function restoreProgram(id: string): Promise<ApiResponse<null>> {
  return apiClient.post(`/api/programs/${id}/restore`)
}