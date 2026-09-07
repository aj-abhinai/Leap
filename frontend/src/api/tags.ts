import { apiClient, type ApiResponse } from '@/composables/useApi'

// Tag catalogs (tags, statuses, quick replies, activity types, loss reasons)
// and their settings:manage mutations. One endpoint family, five kinds.

export interface Tag {
  id: string
  name: string
  type: string
  color?: string
  group_name?: string
  sort_order: number
  behavior: 'log' | 'next' | 'close_lost'
  // usage_count reports how many contacts carry the tag (contact_tags rows),
  // so the delete confirm dialog can warn before removing a live label.
  usage_count?: number
  created_at: string
}

export function listTags(type: string): Promise<ApiResponse<Tag[]>> {
  return apiClient.get(`/api/tags?type=${type}`)
}

export function createTag(body: { name: string; type: string }): Promise<ApiResponse<Tag>> {
  return apiClient.post('/api/tags', body)
}

export function updateTag(
  id: string,
  body: { name?: string; color?: string; group_name?: string; sort_order?: number; behavior?: string },
): Promise<ApiResponse<Tag>> {
  return apiClient.patch(`/api/tags/${id}`, body)
}

export function deleteTag(id: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/tags/${id}`)
}