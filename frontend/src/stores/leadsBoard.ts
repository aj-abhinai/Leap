import { computed, shallowRef, watch } from 'vue'
import { defineStore } from 'pinia'
import { fetchBoard, updateLead, type BoardStage } from '@/api/leads'
import { usePipelineStore } from '@/stores/pipeline'
import { createRequestGuard } from '@/composables/useRequestGuard'
import { dateWindow, type DatePreset } from '@/utils/time'
import { toast } from 'vue-sonner'
import { errorMessage } from '@/utils/errors'

// useLeadsBoardStore owns the kanban board: the pipeline selection, the
// filters, the loaded stages, and the move verbs. The Leads page and the
// app-level activity drawer read and refresh this one instance, so a task
// saved in the drawer reloads the board the page renders.
//
// Loading is guarded twice: only the newest request may write, and a response
// for a pipeline the user has left is dropped. A pipeline switch clears the
// board first and bumps revision so the kanban remounts and animates in.
const PIPELINE_KEY = 'crm:leads:pipeline'

export const useLeadsBoardStore = defineStore('leadsBoard', () => {
  const pipelineStore = usePipelineStore()

  const selectedPipelineId = shallowRef('')
  const search = shallowRef('')
  const outcomeFilter = shallowRef<'open' | 'won' | 'lost' | ''>('')
  // '__all__' = no assignee filter; 'none' = unassigned; otherwise a user id.
  const assigneeFilter = shallowRef('__all__')
  // Board window: a named local-day preset, an explicit custom range, or no
  // window ('all'). The window narrows the board by created_at.
  const datePreset = shallowRef<DatePreset>('all')
  const fromDate = shallowRef('')
  const toDate = shallowRef('')

  // Remember the last pipeline so the board opens where the user left it.
  watch(selectedPipelineId, (id) => {
    if (id) localStorage.setItem(PIPELINE_KEY, id)
  })

  const guard = createRequestGuard()
  // Stage id → (capped window leads + true count) from the board endpoint.
  const boardStages = shallowRef<BoardStage[]>([])
  const loading = shallowRef(false)
  // Bumped when the board changes pipeline, so the kanban remounts and
  // animates cards in. Filter changes reuse the same board and don't replay.
  const boardRevision = shallowRef(0)
  // Which pipeline the loaded board belongs to; a mismatch means the selector
  // moved and the old board must be dropped for a fresh load.
  const boardPipelineId = shallowRef('')

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

  // setDatePreset switches the board window. Named presets and 'all' drop any
  // custom range; 'custom' keeps the days an explicit range set.
  function setDatePreset(preset: DatePreset) {
    datePreset.value = preset
    if (preset !== 'custom') {
      fromDate.value = ''
      toDate.value = ''
    }
  }

  // setDateRange applies an explicit local-day range picked on the calendar.
  function setDateRange(from: string, to: string) {
    fromDate.value = from
    toDate.value = to
    datePreset.value = 'custom'
  }

  const activeFilterCount = computed(
    () =>
      (search.value.trim() !== '' ? 1 : 0) +
      (outcomeFilter.value !== '' ? 1 : 0) +
      (assigneeFilter.value !== '__all__' ? 1 : 0) +
      (datePreset.value !== 'all' ? 1 : 0),
  )

  function clearFilters() {
    search.value = ''
    outcomeFilter.value = ''
    assigneeFilter.value = '__all__'
    datePreset.value = 'all'
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

  // load fetches the board for the current pipeline and filters. The window is
  // resolved per load, never cached, so a tab left open across midnight
  // re-reads the viewer's current "today" on the next refresh. A failure keeps
  // the current board and surfaces a toast.
  async function load() {
    const pipelineId = selectedPipelineId.value
    if (!pipelineId) return
    const run = guard.start()
    // A pipeline switch means a different board: clear the old one so the
    // skeleton shows instead of stale cards from the previous pipeline.
    if (boardPipelineId.value !== pipelineId) {
      boardStages.value = []
      boardPipelineId.value = pipelineId
      boardRevision.value++
    }
    loading.value = true
    try {
      const bounds = dateWindow(datePreset.value, fromDate.value, toDate.value)
      const res = await fetchBoard({
        pipelineId,
        q: search.value.trim() || undefined,
        outcome: outcomeFilter.value || undefined,
        assignedTo: assigneeFilter.value === '__all__' ? undefined : assigneeFilter.value || undefined,
        from: bounds?.from,
        to: bounds?.to,
      })
      // The user may have switched pipelines or changed a filter while this
      // request was in flight; a stale response must not overwrite the board.
      if (!run.isCurrent() || pipelineId !== selectedPipelineId.value) return
      boardStages.value = res.data?.stages ?? []
    } catch {
      if (run.isCurrent() && pipelineId === selectedPipelineId.value) toast.error('Failed to load leads')
    } finally {
      if (run.isCurrent() && pipelineId === selectedPipelineId.value) loading.value = false
    }
  }

  // moveStage moves a lead between stages and reloads the board. A closed lead
  // dragged to an open stage spawns a new cycle server-side (updateLead
  // reports spawned). announce: false runs the move without toasts (Undo).
  async function moveStage(leadId: string, newStageId: string, previousStageId?: string, opts?: { announce?: boolean }) {
    const announce = opts?.announce ?? true
    try {
      const { data } = await updateLead(leadId, { stage_id: newStageId })
      if (!announce) return
      if (data.spawned) {
        toast.success('New lead cycle started')
      } else if (data.lead.stage_outcome !== 'open') {
        // The move closed the lead. Closing is terminal; a true undo cannot
        // exist, so offer none.
        toast.success('Lead closed')
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
      load()
    }
  }

  // bulkMoveStage moves selected leads in batches of 6 and reports the counts;
  // a failed lead stays where it was and the summary names the failures.
  async function bulkMoveStage(leadIds: string[], newStageId: string) {
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
    load()
  }

  return {
    selectedPipelineId,
    selectedPipeline,
    kanbanColumns,
    loading,
    hasBoard,
    boardRevision,
    search,
    outcomeFilter,
    assigneeFilter,
    datePreset,
    fromDate,
    toDate,
    activeFilterCount,
    clearFilters,
    setDatePreset,
    setDateRange,
    syncPipelineSelection,
    load,
    moveStage,
    bulkMoveStage,
  }
})
