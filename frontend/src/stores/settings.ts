import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

export interface Tag {
  id: string
  name: string
  type: 'tag' | 'status' | 'activity_type' | 'loss_reason'
  color?: string
  created_at: string
}

export const useSettingsStore = defineStore('settings', () => {
  const tags = ref<Tag[]>([])
  const statuses = ref<Tag[]>([])
  const activityTypes = ref<Tag[]>([])
  const lossReasons = ref<Tag[]>([])
  const loading = ref(false)

  async function fetchTags() {
    loading.value = true
    try {
      const [tagsRes, statusesRes, activityTypesRes, lossReasonsRes] = await Promise.all([
        apiClient.get('/api/tags?type=tag'),
        apiClient.get('/api/tags?type=status'),
        apiClient.get('/api/tags?type=activity_type'),
        apiClient.get('/api/tags?type=loss_reason'),
      ])
      tags.value = tagsRes.data
      statuses.value = statusesRes.data
      activityTypes.value = activityTypesRes.data
      lossReasons.value = lossReasonsRes.data
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

  return { tags, statuses, activityTypes, lossReasons, loading, fetchTags, createTag, deleteTag }
})
