import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

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
  const entries = ref<ActivityEntry[]>([])
  const total = ref(0)
  const loading = ref(false)

  async function fetchActivity(page = 1, perPage = 20, filters: { action?: string; resourceType?: string } = {}) {
    loading.value = true
    try {
      const params = new URLSearchParams()
      params.set('page', String(page))
      params.set('per_page', String(perPage))
      if (filters.action) params.set('action', filters.action)
      if (filters.resourceType) params.set('resource_type', filters.resourceType)
      const res = await apiClient.get(`/api/activity?${params}`)
      entries.value = res.data
      total.value = res.meta?.total || 0
    } finally {
      loading.value = false
    }
  }

  return { entries, total, loading, fetchActivity }
})
