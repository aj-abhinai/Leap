import { computed, shallowRef } from 'vue'
import { usePipelineStore } from '@/stores/pipeline'
import { fetchBoard, updateLead, type Lead, type BoardStage } from '@/api/leads'
import { toast } from 'vue-sonner'
import { errorMessage } from '@/utils/errors'

// Module-level singleton state so LeadsPage and the app-level lead drawer
// share the same pipeline selection and board: a stage move made from the
// drawer refreshes the kanban without extra wiring.
const selectedPipelineId = shallowRef('')
const search = shallowRef('')
const outcomeFilter = shallowRef<'open' | 'won' | 'lost' | ''>('')
// '__all__' = no assignee filter; 'none' = unassigned; otherwise a user id.
const assigneeFilter = shallowRef('__all__')
// Date range (RFC3339 or '') narrows the board window by created_at.
const fromDate = shallowRef('')
const toDate = shallowRef('')

export function useLeadPipeline() {
  const pipelineStore = usePipelineStore()

  // Stage id → (capped window leads + true count) from the board endpoint.
  const boardStages = shallowRef<BoardStage[]>([])
  const loading = shallowRef(false)

  const selectedPipeline = computed(() =>
    pipelineStore.pipelines.find((p) => p.id === selectedPipelineId.value)
  )

  const kanbanColumns = computed(() => {
    if (!selectedPipeline.value?.stages) return []
    const byStage = new Map(boardStages.value.map((s) => [s.stage_id, s]))
    return selectedPipeline.value.stages.map((stage) => {
      const col = byStage.get(stage.id)
      return {
        ...stage,
        count: col?.count ?? 0,
        leads: col?.leads ?? [],
      }
    })
  })

  async function loadLeads() {
    if (!selectedPipelineId.value) return
    loading.value = true
    try {
      // The date inputs are YYYY-MM-DD; the board filter expects RFC3339.
      const from = fromDate.value ? `${fromDate.value}T00:00:00Z` : undefined
      const to = toDate.value ? `${toDate.value}T23:59:59Z` : undefined
      const res = await fetchBoard({
        pipelineId: selectedPipelineId.value,
        q: search.value.trim() || undefined,
        outcome: outcomeFilter.value || undefined,
        assignedTo: assigneeFilter.value === '__all__' ? undefined : assigneeFilter.value || undefined,
        from,
        to,
      })
      boardStages.value = res.data?.stages ?? []
    } catch {
      toast.error('Failed to load leads')
    } finally {
      loading.value = false
    }
  }

  // moveStage moves an open lead; a closed lead dragged to an open stage
  // spawns a new cycle server-side and returns the new row, so the response
  // lead replaces the old one in the board.
  async function moveStage(leadId: string, newStageId: string, previousStageId?: string) {
    try {
      const res = await updateLead(leadId, { stage_id: newStageId })
      const moved = res.data
      // If a new cycle was spawned, the old card is gone and the new one
      // appears in the target column after the reload.
      if (moved?.id && moved.id !== leadId) {
        toast.success('New lead cycle started')
      } else {
        toast.success('Lead moved', {
          action: previousStageId
            ? {
                label: 'Undo',
                onClick: async () => {
                  await updateLead(leadId, { stage_id: previousStageId })
                  await loadLeads()
                },
              }
            : undefined,
          duration: 5000,
        })
      }
      loadLeads()
    } catch (e) {
      toast.error(errorMessage(e, 'Failed to move lead'))
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
        chunk.map((id) => updateLead(id, { stage_id: newStageId })),
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
    selectedPipelineId,
    selectedPipeline,
    kanbanColumns,
    loading,
    search,
    outcomeFilter,
    assigneeFilter,
    fromDate,
    toDate,
    loadLeads,
    moveStage,
    bulkMoveStage,
  }
}
