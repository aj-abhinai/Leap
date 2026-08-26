import { apiClient, type ApiResponse } from '@/composables/useApi'

// Users and the assignee-options read used by the lead forms.

export interface User {
  id: string
  name: string
  email: string
  role?: { id: string; name: string } | null
  protected?: boolean
  created_at: string
}

export interface UserOption {
  id: string
  name: string
}

export function listUsers(): Promise<ApiResponse<User[]>> {
  return apiClient.get('/api/users')
}

export function listUserOptions(): Promise<ApiResponse<UserOption[]>> {
  return apiClient.get('/api/users/options')
}

export function createUser(body: { name: string; email: string; password: string }): Promise<ApiResponse<User>> {
  return apiClient.post('/api/users', body)
}

export function deleteUser(id: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/users/${id}`)
}

export function setUserRole(userId: string, roleId: string): Promise<ApiResponse<null>> {
  return apiClient.put(`/api/users/${userId}/role`, { role_id: roleId })
}