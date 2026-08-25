import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

export interface ActivityListItem {
  id: string
  lead_id: string
  contact_id: string
  lead_display_name: string
  stage_id: string
  stage_name?: string
  user_id?: string
  user_name?: string
  type: string
  description: string
  quick_reply_id?: string
  quick_reply_name?: string
  scheduled_at?: string
  remind_at?: string
  responded_at?: string
  occurred_at?: string
  is_done: boolean
  is_cancelled: boolean
  is_reminded: boolean
  created_at: string
}

export interface ActivityListFilters {
  status?: string
  overdue?: string
  mine?: string
  type?: string
  q?: string
  from?: string
  to?: string
  sort?: string
  order?: string
}

export const useActivitiesStore = defineStore('activities', () => {
  const items = ref<ActivityListItem[]>([])
  const total = ref(0)
  const page = ref(1)
  const perPage = ref(50)
  const loading = ref(false)
  const recent = ref<ActivityListItem[]>([])

  function buildQuery(f: ActivityListFilters, pageNum: number, per: number): string {
    const params = new URLSearchParams()
    if (f.status) params.set('status', f.status)
    if (f.overdue) params.set('overdue', f.overdue)
    if (f.mine) params.set('mine', f.mine)
    if (f.type) params.set('type', f.type)
    if (f.q) params.set('q', f.q)
    if (f.from) params.set('from', f.from)
    if (f.to) params.set('to', f.to)
    if (f.sort) params.set('sort', f.sort)
    if (f.order) params.set('order', f.order)
    params.set('page', String(pageNum))
    params.set('per_page', String(per))
    return params.toString()
  }

  // fetchSeq discards out-of-order responses: only the latest request's
  // result may land, so fast filter changes never show stale data.
  let fetchSeq = 0

  async function fetchItems(f: ActivityListFilters) {
    const seq = ++fetchSeq
    loading.value = true
    try {
      const res = await apiClient.get(`/api/activities?${buildQuery(f, page.value, perPage.value)}`)
      if (seq !== fetchSeq) return
      items.value = res.data
      total.value = res.meta?.total ?? 0
    } finally {
      if (seq === fetchSeq) loading.value = false
    }
  }

  async function fetchRecent(n = 10) {
    try {
      const q = buildQuery({ sort: 'due_at', order: 'desc' }, 1, n)
      const res = await apiClient.get(`/api/activities?${q}`)
      recent.value = res.data
    } catch {
      recent.value = []
    }
  }

  async function deleteItem(leadID: string, activityID: string) {
    await apiClient.delete(`/api/leads/${leadID}/activities/${activityID}`)
  }

  async function markDone(leadID: string, activityID: string) {
    await apiClient.patch(`/api/leads/${leadID}/activities/${activityID}`, { is_done: true })
  }

  async function snooze(leadID: string, activityID: string, remindAt: string) {
    await apiClient.post(`/api/leads/${leadID}/reminders/${activityID}/snooze`, { remind_at: remindAt })
  }

  return { items, total, page, perPage, loading, recent, fetchItems, fetchRecent, deleteItem, markDone, snooze }
})