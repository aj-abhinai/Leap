import { defineStore } from 'pinia'
import { apiClient } from '@/composables/useApi'
import { usePagination } from '@/composables/usePagination'

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

export const useActivityStore = defineStore('activity', () => {
  const { items: entries, total, loading, fetch: fetchPage } = usePagination<ActivityEntry>()

  async function fetchActivity(page = 1, perPage = 20, filters: { action?: string; resourceType?: string } = {}) {
    await fetchPage(async (p, pp) => {
      const params = new URLSearchParams()
      params.set('page', String(p))
      params.set('per_page', String(pp))
      if (filters.action) params.set('action', filters.action)
      if (filters.resourceType) params.set('resource_type', filters.resourceType)
      return apiClient.get(`/api/activity?${params}`)
    }, page, perPage)
  }

  return { entries, total, loading, fetchActivity }
})
