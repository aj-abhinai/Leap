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

  async function fetchActivity(page = 1, perPage = 20) {
    loading.value = true
    try {
      const res = await apiClient.get(`/api/activity?page=${page}&per_page=${perPage}`)
      entries.value = res.data
      total.value = res.meta?.total || 0
    } finally {
      loading.value = false
    }
  }

  return { entries, total, loading, fetchActivity }
})
