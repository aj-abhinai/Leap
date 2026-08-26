import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/tags'

export type { Tag } from '@/api/tags'

export const useSettingsStore = defineStore('settings', () => {
  const tags = ref<api.Tag[]>([])
  const statuses = ref<api.Tag[]>([])
  const quickReplies = ref<api.Tag[]>([])
  const activityTypes = ref<api.Tag[]>([])
  const lossReasons = ref<api.Tag[]>([])
  const loading = ref(false)

  async function fetchTags() {
    loading.value = true
    try {
      const [tagsRes, statusesRes, quickRepliesRes, activityTypesRes, lossReasonsRes] = await Promise.all([
        api.listTags('tag'),
        api.listTags('status'),
        api.listTags('quick_reply'),
        api.listTags('activity_type'),
        api.listTags('loss_reason'),
      ])
      tags.value = tagsRes.data
      statuses.value = statusesRes.data
      quickReplies.value = quickRepliesRes.data
      activityTypes.value = activityTypesRes.data
      lossReasons.value = lossReasonsRes.data
    } finally {
      loading.value = false
    }
  }

  async function createTag(name: string, type: string) {
    await api.createTag({ name, type })
    await fetchTags()
  }

  async function updateTag(
    id: string,
    data: { name?: string; color?: string; group_name?: string; sort_order?: number; behavior?: string },
  ) {
    await api.updateTag(id, data)
    await fetchTags()
  }

  async function deleteTag(id: string) {
    await api.deleteTag(id)
    await fetchTags()
  }

  return { tags, statuses, quickReplies, activityTypes, lossReasons, loading, fetchTags, createTag, updateTag, deleteTag }
})