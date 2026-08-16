import { defineStore } from 'pinia'
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

export const useLeadsStore = defineStore('leads', () => {
  const { items: leads, total, loading, fetch: fetchPage, setTotal } = usePagination<Lead>()

  async function fetchLeads(pipelineId = '', stageId = '', page = 1, perPage = 50) {
    await fetchPage(async (p, pp) => {
      const params = new URLSearchParams()
      params.set('page', String(p))
      params.set('per_page', String(pp))
      if (pipelineId) params.set('pipeline_id', pipelineId)
      if (stageId) params.set('stage_id', stageId)
      return apiClient.get(`/api/leads?${params}`)
    }, page, perPage)
  }

  async function fetchTotal() {
    try {
      const params = new URLSearchParams()
      params.set('per_page', '1')
      const res = await apiClient.get(`/api/leads?${params}`)
      setTotal(totalOf(res))
    } catch {}
  }

  return { leads, total, loading, fetchLeads, fetchTotal }
})
