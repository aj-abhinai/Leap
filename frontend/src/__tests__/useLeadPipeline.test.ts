import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePipelineStore } from '@/stores/pipeline'
import type { ApiResponse } from '@/composables/useApi'
import type { Board, Lead } from '@/api/leads'

vi.mock('@/api/leads', () => ({
  fetchBoard: vi.fn(),
  updateLead: vi.fn(),
}))

const { toast } = vi.hoisted(() => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('vue-sonner', () => ({ toast }))

import { fetchBoard, updateLead } from '@/api/leads'
import { useLeadPipeline } from '@/composables/useLeadPipeline'

const fetchBoardMock = vi.mocked(fetchBoard)
const updateLeadMock = vi.mocked(updateLead)

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function boardWith(lead: Lead): ApiResponse<Board> {
  return { data: { stages: [{ stage_id: 's1', count: 1, leads: [lead] }] } }
}

function makeLead(id: string, stageOutcome: 'open' | 'won' | 'lost' = 'open'): Lead {
  return {
    id,
    display_name: id,
    contact_id: 'c1',
    pipeline_id: 'p1',
    stage_id: 's1',
    stage_outcome: stageOutcome,
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-01T00:00:00Z',
  }
}

function setupPipeline() {
  setActivePinia(createPinia())
  const pipelineStore = usePipelineStore()
  pipelineStore.pipelines = [
    {
      id: 'p1',
      name: 'Sales',
      stages: [{ id: 's1', pipeline_id: 'p1', name: 'New', order: 0, is_closing: false, outcome: 'open' }],
    },
  ]
  const pipeline = useLeadPipeline()
  pipeline.selectedPipelineId.value = 'p1'
  // The composable keeps module-level filter state; each test starts clean.
  pipeline.datePreset.value = 'all'
  pipeline.fromDate.value = ''
  pipeline.toDate.value = ''
  return pipeline
}

describe('useLeadPipeline loadLeads sequence guard', () => {
  beforeEach(() => {
    fetchBoardMock.mockReset()
    toast.error.mockReset()
    toast.success.mockReset()
  })

  // Two overlapping loads stand in for two filter changes: the older request
  // must not overwrite the newer response even when it lands last.
  it('keeps the newest board when overlapping loads resolve out of order', async () => {
    const first = deferred<ApiResponse<Board>>()
    const second = deferred<ApiResponse<Board>>()
    fetchBoardMock.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const pipeline = setupPipeline()

    const load1 = pipeline.loadLeads()
    const load2 = pipeline.loadLeads()

    second.resolve(boardWith(makeLead('lead-new')))
    await load2
    first.resolve(boardWith(makeLead('lead-old')))
    await load1

    expect(pipeline.kanbanColumns.value[0].leads.map((l) => l.id)).toEqual(['lead-new'])
    expect(pipeline.loading.value).toBe(false)
  })

  it('ignores a stale failure after a newer load won', async () => {
    const first = deferred<ApiResponse<Board>>()
    const second = deferred<ApiResponse<Board>>()
    fetchBoardMock.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const pipeline = setupPipeline()

    const load1 = pipeline.loadLeads()
    const load2 = pipeline.loadLeads()

    second.resolve(boardWith(makeLead('lead-new')))
    await load2
    first.reject(new Error('stale failure'))
    await load1

    expect(toast.error).not.toHaveBeenCalled()
    expect(pipeline.kanbanColumns.value[0].leads.map((l) => l.id)).toEqual(['lead-new'])
  })

  // The page and the activities drawer each own a board: one instance's load
  // must not cancel the other's in-flight load or strand its loading state.
  it('lets two composable instances load independently', async () => {
    const first = deferred<ApiResponse<Board>>()
    const second = deferred<ApiResponse<Board>>()
    fetchBoardMock.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)

    const page = setupPipeline()
    const drawer = useLeadPipeline()
    drawer.selectedPipelineId.value = 'p1'

    const pageLoad = page.loadLeads()
    const drawerLoad = drawer.loadLeads()

    second.resolve(boardWith(makeLead('drawer-lead')))
    await drawerLoad
    first.resolve(boardWith(makeLead('page-lead')))
    await pageLoad

    expect(page.kanbanColumns.value[0].leads.map((l) => l.id)).toEqual(['page-lead'])
    expect(page.loading.value).toBe(false)
    expect(drawer.kanbanColumns.value[0].leads.map((l) => l.id)).toEqual(['drawer-lead'])
  })
})

describe('useLeadPipeline moveStage toasts', () => {
  beforeEach(() => {
    updateLeadMock.mockReset()
    fetchBoardMock.mockReset().mockResolvedValue(boardWith(makeLead('lead-1')))
    toast.error.mockReset()
    toast.success.mockReset()
  })

  it('offers Undo for an open-to-open move', async () => {
    updateLeadMock.mockResolvedValue({ data: { lead: makeLead('lead-1'), spawned: false } })
    const pipeline = setupPipeline()

    await pipeline.moveStage('lead-1', 's2', 's1')

    expect(toast.success).toHaveBeenCalledWith(
      'Lead moved',
      expect.objectContaining({ action: expect.objectContaining({ label: 'Undo' }) }),
    )
  })

  it('toasts "Lead closed" with no Undo when the move closes the lead', async () => {
    updateLeadMock.mockResolvedValue({ data: { lead: makeLead('lead-1', 'lost'), spawned: false } })
    const pipeline = setupPipeline()

    await pipeline.moveStage('lead-1', 's2', 's1')

    expect(toast.success).toHaveBeenCalledWith('Lead closed')
    expect(toast.success).not.toHaveBeenCalledWith('Lead moved', expect.anything())
  })
})

describe('useLeadPipeline date window', () => {
  beforeEach(() => {
    fetchBoardMock.mockReset().mockResolvedValue(boardWith(makeLead('lead-1')))
    // Wednesday 30 September 2026, noon; window expectations use the same
    // local-date primitives so they hold in any time zone.
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 8, 30, 12, 0, 0))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it("sends the viewer's local day for the today preset", async () => {
    const pipeline = setupPipeline()
    pipeline.setDatePreset('today')
    await pipeline.loadLeads()

    const params = fetchBoardMock.mock.calls[0][0]
    expect(params.from).toBe(new Date(2026, 8, 30).toISOString())
    expect(params.to).toBe(new Date(new Date(2026, 9, 1).getTime() - 1).toISOString())
  })

  it('sends a custom range from the start day to the end day', async () => {
    const pipeline = setupPipeline()
    pipeline.setDateRange('2026-09-01', '2026-09-03')
    await pipeline.loadLeads()

    const params = fetchBoardMock.mock.calls[0][0]
    expect(params.from).toBe(new Date(2026, 8, 1).toISOString())
    expect(params.to).toBe(new Date(new Date(2026, 8, 4).getTime() - 1).toISOString())
  })

  it('sends no window for all dates', async () => {
    const pipeline = setupPipeline()
    pipeline.setDatePreset('today')
    pipeline.setDatePreset('all')
    await pipeline.loadLeads()

    const params = fetchBoardMock.mock.calls[0][0]
    expect(params.from).toBeUndefined()
    expect(params.to).toBeUndefined()
  })

  it('clears the window with the other filters', () => {
    const pipeline = setupPipeline()
    pipeline.setDateRange('2026-09-01', '2026-09-03')
    expect(pipeline.activeFilterCount.value).toBe(1)

    pipeline.clearFilters()

    expect(pipeline.datePreset.value).toBe('all')
    expect(pipeline.fromDate.value).toBe('')
    expect(pipeline.toDate.value).toBe('')
    expect(pipeline.activeFilterCount.value).toBe(0)
  })
})
