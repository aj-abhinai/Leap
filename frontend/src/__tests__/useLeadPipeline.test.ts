import { describe, it, expect, vi, beforeEach } from 'vitest'
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
