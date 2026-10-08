import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/tags'
import * as settingsApi from '@/api/settings'

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

  // nudgeLeadMinutes is the org's reminder lead time in minutes; null until
  // the first successful read. Consumers treat null as "use the built-in
  // default".
  const nudgeLeadMinutes = ref<number | null>(null)
  const nudgeLoaded = ref(false)

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

  // fetchNudgeLead reads the org reminder lead once per session. A failure is
  // silent and keeps null: the module default mirrors the server default, so
  // the app stays usable without the value.
  async function fetchNudgeLead() {
    if (nudgeLoaded.value) return
    try {
      const res = await settingsApi.getNudgeLeadMinutes()
      nudgeLeadMinutes.value = res.data.minutes
      nudgeLoaded.value = true
    } catch {}
  }

  // applyNudgeLead records a value saved by this session so the shared task
  // env picks it up without a refetch.
  function applyNudgeLead(minutes: number) {
    nudgeLeadMinutes.value = minutes
    nudgeLoaded.value = true
  }

  return {
    tags,
    statuses,
    quickReplies,
    activityTypes,
    lossReasons,
    loading,
    loadError,
    fetchTags,
    createTag,
    updateTag,
    deleteTag,
    nudgeLeadMinutes,
    fetchNudgeLead,
    applyNudgeLead,
  }
})