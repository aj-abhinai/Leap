import { computed, shallowRef } from 'vue'
import { usePipelineStore } from '@/stores/pipeline'
import { useLeadsStore } from '@/stores/leads'
import { updateLead } from '@/api/leads'
import { toast } from 'vue-sonner'
import { errorMessage } from '@/utils/errors'

// Module-level singleton state so LeadsPage and the app-level lead drawer
// share the same pipeline selection and lead store: a stage move made from
// the drawer refreshes the kanban without extra wiring.
const selectedPipelineId = shallowRef('')
const search = shallowRef('')
const outcomeFilter = shallowRef<'open' | 'won' | 'lost' | ''>('')
// '__all__' = no assignee filter; 'none' = unassigned; otherwise a user id.
const assigneeFilter = shallowRef('__all__')
// Remembers the last query that hit the fetch-all cap so the warning toast
// appears once per distinct query, not on every loadLeads call.
let lastCappedQuery = ''

export function useLeadPipeline() {
  const pipelineStore = usePipelineStore()
  const leadsStore = useLeadsStore()

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
      await leadsStore.fetchAllLeads({
        pipelineId: selectedPipelineId.value,
        q: search.value.trim() || undefined,
        outcome: outcomeFilter.value || undefined,
        assignedTo: assigneeFilter.value === '__all__' ? undefined : assigneeFilter.value || undefined,
      })
      // Warn once per distinct capped query so repeated calls (debounced
      // keystrokes, filter toggles) don't spam the toast.
      const sig = `${selectedPipelineId.value}|${search.value}|${outcomeFilter.value}|${assigneeFilter.value}`
      if (leadsStore.capped && lastCappedQuery !== sig) {
        lastCappedQuery = sig
        toast.warning('Showing first 2000 leads — narrow the search or filters')
      }
    } catch {
      toast.error('Failed to load leads')
    }
  }

  async function moveStage(leadId: string, newStageId: string, previousStageId?: string) {
    try {
      await updateLead(leadId, { stage_id: newStageId })
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
    leadsStore,
    selectedPipelineId,
    selectedPipeline,
    kanbanColumns,
    search,
    outcomeFilter,
    assigneeFilter,
    loadLeads,
    moveStage,
    bulkMoveStage,
  }
}
