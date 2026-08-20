import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'
import { usePagination, totalOf } from '@/composables/usePagination'

export interface Lead {
  id: string
  nickname?: string
  display_name: string
  contact_id: string
  contact_name?: string
  contact_phone?: string
  contact_email?: string
  pipeline_id: string
  stage_id: string
  stage_name?: string
  outcome?: string
  lost_reason?: string
  program_id?: string
  program_name?: string
  value?: number
  notes?: string
  assigned_to?: string
  created_at: string
  updated_at: string
  next_task_type?: string
  next_task_at?: string
  last_touch_type?: string
  last_touch_at?: string
}

export interface LeadListQuery {
  pipelineId?: string
  stageId?: string
  q?: string
  outcome?: 'open' | 'won' | 'lost'
  assignedTo?: string
}

export const useLeadsStore = defineStore('leads', () => {
  const { items: leads, total, loading, setTotal } = usePagination<Lead>()
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
  async function fetchAllLeads(f: LeadListQuery = {}) {
    const seq = ++fetchSeq
    const all: Lead[] = []
    let page = 1
    let serverTotal = 0
    capped.value = false
    loading.value = true
    try {
      for (;;) {
        const params = new URLSearchParams()
        params.set('page', String(page))
        params.set('per_page', String(PAGE_SIZE))
        if (f.pipelineId) params.set('pipeline_id', f.pipelineId)
        if (f.stageId) params.set('stage_id', f.stageId)
        if (f.q) params.set('q', f.q)
        if (f.outcome) params.set('outcome', f.outcome)
        if (f.assignedTo) params.set('assigned_to', f.assignedTo)
        const res = await apiClient.get<Lead[]>(`/api/leads?${params}`)
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
      const params = new URLSearchParams()
      params.set('per_page', '1')
      const res = await apiClient.get(`/api/leads?${params}`)
      setTotal(totalOf(res))
    } catch {}
  }

  return { leads, total, loading, capped, fetchAllLeads, fetchTotal }
})
