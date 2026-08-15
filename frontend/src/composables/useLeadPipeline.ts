import { computed, shallowRef } from 'vue'
import { usePipelineStore } from '@/stores/pipeline'
import { useLeadsStore } from '@/stores/leads'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'

export function useLeadPipeline() {
  const pipelineStore = usePipelineStore()
  const leadsStore = useLeadsStore()
  const selectedPipelineId = shallowRef('')

  const selectedPipeline = computed(() =>
    pipelineStore.pipelines.find((p) => p.id === selectedPipelineId.value)
  )

  const kanbanColumns = computed(() => {
    if (!selectedPipeline.value?.stages) return []
    return selectedPipeline.value.stages.map((stage) => ({
      ...stage,
      leads: leadsStore.leads.filter((l) => l.stage_id === stage.id),
    }))
  })

  async function loadLeads() {
    if (!selectedPipelineId.value) return
    try {
      await leadsStore.fetchLeads(selectedPipelineId.value, '', 1, 200)
    } catch {
      toast.error('Failed to load leads')
    }
  }

  async function moveStage(leadId: string, newStageId: string, previousStageId?: string) {
    try {
      await apiClient.patch(`/api/leads/${leadId}`, { stage_id: newStageId })
      toast.success('Lead moved', {
        action: previousStageId
          ? {
              label: 'Undo',
              onClick: async () => {
                await apiClient.patch(`/api/leads/${leadId}`, { stage_id: previousStageId })
                await loadLeads()
              },
            }
          : undefined,
        duration: 5000,
      })
      loadLeads()
    } catch (e: any) {
      toast.error(e.message || 'Failed to move lead')
      loadLeads()
    }
  }

  async function bulkMoveStage(leadIds: string[], newStageId: string) {
    // Bounded concurrency: fire at most BATCH at a time so a full-column
    // select (up to 200 leads) doesn't hammer the API sequentially while
    // still completing quickly. Results are counted, not surfaced per lead.
    const BATCH = 6
    let moved = 0
    for (let i = 0; i < leadIds.length; i += BATCH) {
      const chunk = leadIds.slice(i, i + BATCH)
      const results = await Promise.allSettled(
        chunk.map((id) => apiClient.patch(`/api/leads/${id}`, { stage_id: newStageId })),
      )
      moved += results.filter((r) => r.status === 'fulfilled').length
    }
    const failed = leadIds.length - moved
    if (failed > 0) {
      toast.error(`Moved ${moved} of ${leadIds.length} leads (${failed} failed)`)
    } else if (moved > 0) {
      toast.success(`Moved ${moved} leads`)
    }
    loadLeads()
  }

  return {
    pipelineStore,
    leadsStore,
    selectedPipelineId,
    selectedPipeline,
    kanbanColumns,
    loadLeads,
    moveStage,
    bulkMoveStage,
  }
}
