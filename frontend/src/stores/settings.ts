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

  // loadError names the word lists whose last load failed; empty when the
  // last fetch settled all five.
  const loadError = ref('')

  async function fetchTags() {
    loading.value = true
    loadError.value = ''
    const results = await Promise.allSettled([
      api.listTags('tag'),
      api.listTags('status'),
      api.listTags('quick_reply'),
      api.listTags('activity_type'),
      api.listTags('loss_reason'),
    ])
    const lists = [
      ['tags', results[0], tags],
      ['statuses', results[1], statuses],
      ['quick replies', results[2], quickReplies],
      ['task types', results[3], activityTypes],
      ['loss reasons', results[4], lossReasons],
    ] as const
    const failed: string[] = []
    for (const [label, result, target] of lists) {
      if (result.status === 'fulfilled') target.value = result.value.data
      else failed.push(label)
    }
    if (failed.length > 0) {
      loadError.value = `Failed to load: ${failed.join(', ')}`
    }
    loading.value = false
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

  return { tags, statuses, quickReplies, activityTypes, lossReasons, loading, loadError, fetchTags, createTag, updateTag, deleteTag }
})