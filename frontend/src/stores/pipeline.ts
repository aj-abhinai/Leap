import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

export interface Stage {
  id: string
  pipeline_id: string
  name: string
  order: number
  color?: string
  is_closing: boolean
}

export interface Pipeline {
  id: string
  name: string
  description?: string
  stages?: Stage[]
}

export const usePipelineStore = defineStore('pipeline', () => {
  const pipelines = ref<Pipeline[]>([])
  const loading = ref(false)

  async function fetchPipelines() {
    loading.value = true
    try {
      const res = await apiClient.get('/api/pipelines')
      pipelines.value = res.data
    } finally {
      loading.value = false
    }
  }

  return { pipelines, loading, fetchPipelines }
})
