import { defineStore } from 'pinia'
import { ref } from 'vue'
import { usePagination, totalOf } from '@/composables/usePagination'
import * as api from '@/api/leads'

export type { Lead, LeadListQuery } from '@/api/leads'

export const useLeadsStore = defineStore('leads', () => {
  const { items: leads, total, loading, setTotal } = usePagination<api.Lead>()
  // capped is true when fetchAllLeads stopped early at the safety cap instead
  // of loading the full result set.
  const capped = ref(false)

  // fetchAllLeads loads every matching lead across pages (200/page) so the
  // kanban never silently drops leads beyond the first page. It stops at a
  // safety cap (MAX_PAGES) and flags `capped` when the result set is larger.
  const PAGE_SIZE = 200
  const MAX_PAGES = 10
  // fetchSeq discards stale responses: overlapping calls (debounced search,
  // filter changes, pipeline switches) must not let an older request overwrite
  // a newer one.
  let fetchSeq = 0
  async function fetchAllLeads(f: api.LeadListQuery = {}) {
    const seq = ++fetchSeq
    const all: api.Lead[] = []
    let page = 1
    let serverTotal = 0
    capped.value = false
    loading.value = true
    try {
      for (;;) {
        const res = await api.listLeads({ page, perPage: PAGE_SIZE, query: f })
        if (seq !== fetchSeq) return
        const items = res.data ?? []
        serverTotal = res.meta?.total ?? 0
        all.push(...items)
        if (items.length < PAGE_SIZE || all.length >= serverTotal) break
        if (page >= MAX_PAGES) {
          capped.value = true
          break
        }
        page++
      }
      if (seq !== fetchSeq) return
      leads.value = all
      total.value = serverTotal
    } finally {
      if (seq === fetchSeq) loading.value = false
    }
  }

  async function fetchTotal() {
    try {
      const res = await api.listLeads({ page: 1, perPage: 1 })
      setTotal(totalOf(res))
    } catch {}
  }

  return { leads, total, loading, capped, fetchAllLeads, fetchTotal }
})