import { computed, shallowRef, watch } from 'vue'
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

// Remember the last pipeline so the board opens where the user left it.
const PIPELINE_KEY = 'crm:leads:pipeline'
watch(selectedPipelineId, (id) => {
  if (id) localStorage.setItem(PIPELINE_KEY, id)
})

// Which pipeline the currently loaded board belongs to; a mismatch means the
// selector moved and the old board must be dropped for a fresh load.
const boardPipelineId = shallowRef('')

export function useLeadPipeline() {
  const pipelineStore = usePipelineStore()

  // Stage id → (capped window leads + true count) from the board endpoint.
  const boardStages = shallowRef<BoardStage[]>([])
  const loading = shallowRef(false)
  // Bumped when the board changes pipeline, so the kanban remounts and
  // animates cards in. Filter changes reuse the same board and don't replay.
  const boardRevision = shallowRef(0)

  const hasBoard = computed(() => boardStages.value.length > 0)

  // Pipeline selector is a view switch, not a filter: prefer the remembered
  // pipeline, fall back to the first one. No-op while no pipelines exist so a
  // late-arriving list (created elsewhere) can still select on arrival.
  function syncPipelineSelection() {
    if (pipelineStore.pipelines.length === 0) return
    if (pipelineStore.pipelines.some((p) => p.id === selectedPipelineId.value)) return
    const saved = localStorage.getItem(PIPELINE_KEY)
    selectedPipelineId.value =
      saved && pipelineStore.pipelines.some((p) => p.id === saved)
        ? saved
        : pipelineStore.pipelines[0].id
  }

  const activeFilterCount = computed(
    () =>
      (search.value.trim() !== '' ? 1 : 0) +
      (outcomeFilter.value !== '' ? 1 : 0) +
      (assigneeFilter.value !== '__all__' ? 1 : 0) +
      (fromDate.value !== '' ? 1 : 0) +
      (toDate.value !== '' ? 1 : 0),
  )

  function clearFilters() {
    search.value = ''
    outcomeFilter.value = ''
    assigneeFilter.value = '__all__'
    fromDate.value = ''
    toDate.value = ''
  }

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
    const pipelineId = selectedPipelineId.value
    if (!pipelineId) return
    // A pipeline switch means a different board: clear the old one so the
    // skeleton shows instead of stale cards from the previous pipeline, and
    // bump the revision so the fresh board animates in. Filter changes keep
    // the current board and dim it until the new data lands.
    if (boardPipelineId.value !== pipelineId) {
      boardStages.value = []
      boardPipelineId.value = pipelineId
      boardRevision.value++
    }
    loading.value = true
    try {
      // The date inputs are YYYY-MM-DD; the board filter expects RFC3339.
      const from = fromDate.value ? `${fromDate.value}T00:00:00Z` : undefined
      const to = toDate.value ? `${toDate.value}T23:59:59Z` : undefined
      const res = await fetchBoard({
        pipelineId,
        q: search.value.trim() || undefined,
        outcome: outcomeFilter.value || undefined,
        assignedTo: assigneeFilter.value === '__all__' ? undefined : assigneeFilter.value || undefined,
        from,
        to,
      })
      // The user may have switched pipelines while this request was in
      // flight; a stale response must not overwrite the current board.
      if (pipelineId !== selectedPipelineId.value) return
      boardStages.value = res.data?.stages ?? []
    } catch {
      if (pipelineId === selectedPipelineId.value) toast.error('Failed to load leads')
    } finally {
      if (pipelineId === selectedPipelineId.value) loading.value = false
    }
  }

  // moveStage moves an open lead; a closed lead dragged to an open stage
  // spawns a new cycle server-side (updateLead reports spawned), so the
  // response lead replaces the old one in the board. announce: false runs the
  // move without success/error toasts (used by Undo, which was silent before
  // the refactor).
  async function moveStage(leadId: string, newStageId: string, previousStageId?: string, opts?: { announce?: boolean }) {
    const announce = opts?.announce ?? true
    try {
      const { data } = await updateLead(leadId, { stage_id: newStageId })
      if (!announce) return
      if (data.spawned) {
        toast.success('New lead cycle started')
      } else {
        toast.success('Lead moved', {
          action: previousStageId
            ? {
                label: 'Undo',
                onClick: () => moveStage(leadId, previousStageId!, undefined, { announce: false }),
              }
            : undefined,
          duration: 5000,
        })
      }
    } catch (e) {
      if (announce) toast.error(errorMessage(e, 'Failed to move lead'))
    } finally {
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
    hasBoard,
    boardRevision,
    search,
    outcomeFilter,
    assigneeFilter,
    fromDate,
    toDate,
    activeFilterCount,
    clearFilters,
    syncPipelineSelection,
    loadLeads,
    moveStage,
    bulkMoveStage,
  }
}
