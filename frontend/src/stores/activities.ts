import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/activities'
import * as leadsApi from '@/api/leads'

export type { ActivityListItem, ActivityListFilters } from '@/api/activities'

export const useActivitiesStore = defineStore('activities', () => {
  const items = ref<api.ActivityListItem[]>([])
  const total = ref(0)
  const page = ref(1)
  const perPage = ref(50)
  const loading = ref(false)
  const recent = ref<api.ActivityListItem[]>([])

  // fetchSeq discards out-of-order responses: only the latest request's
  // result may land, so fast filter changes never show stale data.
  let fetchSeq = 0

  async function fetchItems(f: api.ActivityListFilters) {
    const seq = ++fetchSeq
    loading.value = true
    try {
      const res = await api.listActivities({ page: page.value, perPage: perPage.value, filters: f })
      if (seq !== fetchSeq) return
      items.value = res.data
      total.value = res.meta?.total ?? 0
    } finally {
      if (seq === fetchSeq) loading.value = false
    }
  }

  async function fetchRecent(n = 10) {
    try {
      const res = await api.listActivities({
        page: 1,
        perPage: n,
        filters: { sort: 'due_at', order: 'desc' },
      })
      recent.value = res.data
    } catch {
      recent.value = []
    }
  }

  async function deleteItem(leadID: string, activityID: string) {
    await leadsApi.deleteLeadActivity(leadID, activityID)
  }

  async function markDone(leadID: string, activityID: string) {
    await leadsApi.updateLeadActivity(leadID, activityID, { is_done: true })
  }

  return { items, total, page, perPage, loading, recent, fetchItems, fetchRecent, deleteItem, markDone }
})