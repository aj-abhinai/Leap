import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/pipelines'

export type { Stage, Pipeline } from '@/api/pipelines'

export const usePipelineStore = defineStore('pipeline', () => {
  const pipelines = ref<api.Pipeline[]>([])
  const loading = ref(false)

  async function fetchPipelines() {
    loading.value = true
    try {
      const res = await api.listPipelines()
      pipelines.value = res.data
    } finally {
      loading.value = false
    }
  }

  return { pipelines, loading, fetchPipelines }
})