import { defineStore } from 'pinia'
import { usePagination } from '@/composables/usePagination'
import * as api from '@/api/activity'

export type { ActivityEntry } from '@/api/activity'

export const useActivityStore = defineStore('activity', () => {
  const { items: entries, total, loading, fetch: fetchPage } = usePagination<api.ActivityEntry>()

  async function fetchActivity(page = 1, perPage = 20, filters: { action?: string; resourceType?: string } = {}) {
    await fetchPage((p, pp) => api.listAuditLog({ page: p, perPage: pp, filters }), page, perPage)
  }

  return { entries, total, loading, fetchActivity }
})