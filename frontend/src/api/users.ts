import { apiClient, type ApiResponse } from '@/composables/useApi'

// Users and the assignee-options read used by the lead forms.

export interface User {
  id: string
  name: string
  email: string
  phone?: string
  role?: { id: string; name: string; is_system?: boolean } | null
  protected?: boolean
  // active is false for deactivated (soft-deleted) accounts.
  active?: boolean
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

export function createUser(body: {
  name: string
  email: string
  password: string
  role_id?: string
}): Promise<ApiResponse<User>> {
  return apiClient.post('/api/users', body)
}

// updateUser edits a live user's identity fields; only provided fields change.
export function updateUser(
  id: string,
  body: { name?: string; email?: string; phone?: string },
): Promise<ApiResponse<User>> {
  return apiClient.patch(`/api/users/${id}`, body)
}

// resetUserPassword sets a temporary password; the user must change it at
// their next login.
export function resetUserPassword(id: string, password: string): Promise<ApiResponse<null>> {
  return apiClient.post(`/api/users/${id}/reset-password`, { password })
}

export function deleteUser(id: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/users/${id}`)
}

// reactivateUser lifts a deactivated account.
export function reactivateUser(id: string): Promise<ApiResponse<User>> {
  return apiClient.post(`/api/users/${id}/reactivate`)
}

export function setUserRole(userId: string, roleId: string): Promise<ApiResponse<null>> {
  return apiClient.put(`/api/users/${userId}/role`, { role_id: roleId })
}