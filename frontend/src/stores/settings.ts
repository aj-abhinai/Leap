import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

export interface Tag {
  id: string
  name: string
  type: string
  color?: string
  created_at: string
}

export const useSettingsStore = defineStore('settings', () => {
  const tags = ref<Tag[]>([])
  const statuses = ref<Tag[]>([])
  const loading = ref(false)

  async function fetchTags() {
    loading.value = true
    try {
      const res = await apiClient.get('/api/tags?type=tag')
      tags.value = res.data
      const statusRes = await apiClient.get('/api/tags?type=status')
      statuses.value = statusRes.data
    } finally {
      loading.value = false
    }
  }

  async function createTag(name: string, type: string) {
    await apiClient.post('/api/tags', { name, type })
    await fetchTags()
  }

  async function deleteTag(id: string) {
    await apiClient.delete(`/api/tags/${id}`)
    await fetchTags()
  }

  return { tags, statuses, loading, fetchTags, createTag, deleteTag }
})
