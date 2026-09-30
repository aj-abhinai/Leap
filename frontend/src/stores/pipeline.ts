import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/pipelines'

export type { Stage, Pipeline } from '@/api/pipelines'

export const usePipelineStore = defineStore('pipeline', () => {
  const pipelines = ref<api.Pipeline[]>([])
  const loading = ref(false)

  // fetchPipelines loads the catalog once and reuses it for the session;
  // force re-fetches after a pipeline or stage mutation. An empty catalog
  // always re-fetches so a workspace whose first pipeline was just created
  // picks it up on the next visit.
  async function fetchPipelines(force = false) {
    if (pipelines.value.length > 0 && !force) return
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