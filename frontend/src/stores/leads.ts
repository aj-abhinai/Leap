import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

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
}

export const useLeadsStore = defineStore('leads', () => {
  const leads = ref<Lead[]>([])
  const total = ref(0)
  const loading = ref(false)

  async function fetchLeads(pipelineId = '', stageId = '', page = 1, perPage = 50) {
    loading.value = true
    try {
      const params = new URLSearchParams()
      params.set('page', String(page))
      params.set('per_page', String(perPage))
      if (pipelineId) params.set('pipeline_id', pipelineId)
      if (stageId) params.set('stage_id', stageId)
      const res = await apiClient.get(`/api/leads?${params}`)
      leads.value = res.data
      total.value = res.meta?.total || 0
    } finally {
      loading.value = false
    }
  }

  async function fetchTotal() {
    try {
      const params = new URLSearchParams()
      params.set('per_page', '1')
      const res = await apiClient.get(`/api/leads?${params}`)
      total.value = res.meta?.total || 0
    } catch {}
  }

  return { leads, total, loading, fetchLeads, fetchTotal }
})
