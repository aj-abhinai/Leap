import { apiClient, type ApiResponse } from '@/composables/useApi'

// Roles and their permission sets (settings:manage).

export interface Role {
  id: string
  name: string
}

export interface Permission {
  id: string
  name: string
  description: string
}

export function listRoles(): Promise<ApiResponse<Role[]>> {
  return apiClient.get('/api/roles')
}

export function listPermissions(): Promise<ApiResponse<Permission[]>> {
  return apiClient.get('/api/permissions')
}

export function createRole(body: { name: string; description?: string }): Promise<ApiResponse<Role>> {
  return apiClient.post('/api/roles', body)
}

export function updateRole(id: string, body: { name: string; description?: string }): Promise<ApiResponse<Role>> {
  return apiClient.patch(`/api/roles/${id}`, body)
}

export function setRolePermissions(id: string, permissionIds: string[]): Promise<ApiResponse<null>> {
  return apiClient.put(`/api/roles/${id}/permissions`, { permission_ids: permissionIds })
}

export function deleteRole(id: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/roles/${id}`)
}