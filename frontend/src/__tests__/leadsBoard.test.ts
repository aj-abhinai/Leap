import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import { usePipelineStore } from '@/stores/pipeline'
import { useLeadsBoardStore } from '@/stores/leadsBoard'
import type { ApiResponse } from '@/composables/useApi'
import type { Board, Lead } from '@/api/leads'

vi.mock('@/api/leads', () => ({
  fetchBoard: vi.fn(),
  updateLead: vi.fn(),
}))

const { toast } = vi.hoisted(() => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('vue-sonner', () => ({ toast }))

import { fetchBoard, updateLead } from '@/api/leads'

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

function boardWith(lead: Lead, stageId = 's1'): ApiResponse<Board> {
  return { data: { stages: [{ stage_id: stageId, count: 1, leads: [lead] }] } }
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

// setupBoard seeds the pipeline catalog and selects the first pipeline; a
// fresh Pinia gives every test a clean board, filters, and request guard.
// Pinia unwraps refs on the store proxy, so reads and writes go direct.
function setupBoard() {
  setActivePinia(createPinia())
  const pipelineStore = usePipelineStore()
  pipelineStore.pipelines = [
    {
      id: 'p1',
      name: 'Sales',
      stages: [{ id: 's1', pipeline_id: 'p1', name: 'New', order: 0, is_closing: false, outcome: 'open' }],
    },
    {
      id: 'p2',
      name: 'Renewals',
      stages: [{ id: 's2', pipeline_id: 'p2', name: 'New', order: 0, is_closing: false, outcome: 'open' }],
    },
  ]
  const board = useLeadsBoardStore()
  board.selectedPipelineId = 'p1'
  return board
}

describe('useLeadsBoardStore loading', () => {
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
    const board = setupBoard()

    const load1 = board.load()
    const load2 = board.load()

    second.resolve(boardWith(makeLead('lead-new')))
    await load2
    first.resolve(boardWith(makeLead('lead-old')))
    await load1

    expect(board.kanbanColumns[0].leads.map((l) => l.id)).toEqual(['lead-new'])
    expect(board.loading).toBe(false)
  })

  it('ignores a stale failure after a newer load won', async () => {
    const first = deferred<ApiResponse<Board>>()
    const second = deferred<ApiResponse<Board>>()
    fetchBoardMock.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const board = setupBoard()

    const load1 = board.load()
    const load2 = board.load()

    second.resolve(boardWith(makeLead('lead-new')))
    await load2
    first.reject(new Error('stale failure'))
    await load1

    expect(toast.error).not.toHaveBeenCalled()
    expect(board.kanbanColumns[0].leads.map((l) => l.id)).toEqual(['lead-new'])
  })

  it('clears the old board and bumps revision when the pipeline changes', async () => {
    const first = deferred<ApiResponse<Board>>()
    fetchBoardMock.mockReturnValueOnce(first.promise)
    const board = setupBoard()

    const load1 = board.load()
    board.selectedPipelineId = 'p2'
    const revisionAfterSwitch = board.boardRevision

    fetchBoardMock.mockResolvedValueOnce(boardWith(makeLead('renewal-lead'), 's2'))
    const load2 = board.load()
    await load2
    first.resolve(boardWith(makeLead('sales-lead')))
    await load1

    expect(board.kanbanColumns[0].leads.map((l) => l.id)).toEqual(['renewal-lead'])
    expect(board.boardRevision).toBe(revisionAfterSwitch + 1)
  })

  // The page and the app-level drawer must read one board: a reload from the
  // drawer is the reload the page renders.
  it('shares one board between the page and the drawer', async () => {
    fetchBoardMock.mockResolvedValue(boardWith(makeLead('lead-1')))
    const page = setupBoard()
    await page.load()

    const drawer = useLeadsBoardStore()
    expect(drawer).toBe(page)
    expect(drawer.kanbanColumns[0].leads.map((l) => l.id)).toEqual(['lead-1'])

    fetchBoardMock.mockResolvedValueOnce(boardWith(makeLead('lead-2')))
    await drawer.load()
    expect(page.kanbanColumns[0].leads.map((l) => l.id)).toEqual(['lead-2'])
  })
})

describe('useLeadsBoardStore moveStage toasts', () => {
  beforeEach(() => {
    updateLeadMock.mockReset()
    fetchBoardMock.mockReset().mockResolvedValue(boardWith(makeLead('lead-1')))
    toast.error.mockReset()
    toast.success.mockReset()
  })

  it('offers Undo for an open-to-open move', async () => {
    updateLeadMock.mockResolvedValue({ data: { lead: makeLead('lead-1'), spawned: false } })
    const board = setupBoard()

    await board.moveStage('lead-1', 's2', 's1')

    expect(toast.success).toHaveBeenCalledWith(
      'Lead moved',
      expect.objectContaining({ action: expect.objectContaining({ label: 'Undo' }) }),
    )
  })

  it('toasts "Lead closed" with no Undo when the move closes the lead', async () => {
    updateLeadMock.mockResolvedValue({ data: { lead: makeLead('lead-1', 'lost'), spawned: false } })
    const board = setupBoard()

    await board.moveStage('lead-1', 's2', 's1')

    expect(toast.success).toHaveBeenCalledWith('Lead closed')
    expect(toast.success).not.toHaveBeenCalledWith('Lead moved', expect.anything())
  })

  it('toasts "New lead cycle started" when the move spawns a cycle', async () => {
    updateLeadMock.mockResolvedValue({ data: { lead: makeLead('lead-1'), spawned: true } })
    const board = setupBoard()

    await board.moveStage('lead-1', 's2', 's1')

    expect(toast.success).toHaveBeenCalledWith('New lead cycle started')
  })

  it('Undo moves the lead back silently and reloads the board', async () => {
    updateLeadMock.mockResolvedValue({ data: { lead: makeLead('lead-1'), spawned: false } })
    const board = setupBoard()

    await board.moveStage('lead-1', 's2', 's1')
    await flushPromises()
    const opts = toast.success.mock.calls[0][1] as { action: { onClick: () => Promise<void> } }
    fetchBoardMock.mockClear()
    toast.success.mockClear()

    await opts.action.onClick()
    await flushPromises()

    expect(updateLeadMock).toHaveBeenLastCalledWith('lead-1', { stage_id: 's1' })
    expect(toast.success).not.toHaveBeenCalled()
    expect(fetchBoardMock).toHaveBeenCalledTimes(1)
  })
})

describe('useLeadsBoardStore bulkMoveStage', () => {
  beforeEach(() => {
    updateLeadMock.mockReset()
    fetchBoardMock.mockReset().mockResolvedValue(boardWith(makeLead('lead-1')))
    toast.error.mockReset()
    toast.success.mockReset()
  })

  it('moves in batches of 6 and reloads the board once', async () => {
    let inFlight = 0
    let maxInFlight = 0
    updateLeadMock.mockImplementation(() => {
      inFlight++
      maxInFlight = Math.max(maxInFlight, inFlight)
      return Promise.resolve({ data: { lead: makeLead('lead-1'), spawned: false } }).finally(() => {
        inFlight--
      })
    })
    const board = setupBoard()
    const ids = Array.from({ length: 7 }, (_, i) => `lead-${i}`)

    await board.bulkMoveStage(ids, 's2')
    await flushPromises()

    expect(updateLeadMock).toHaveBeenCalledTimes(7)
    expect(maxInFlight).toBeLessThanOrEqual(6)
    expect(fetchBoardMock).toHaveBeenCalledTimes(1)
  })

  it('reports partial failures with the counts', async () => {
    updateLeadMock
      .mockResolvedValueOnce({ data: { lead: makeLead('lead-1'), spawned: false } })
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce({ data: { lead: makeLead('lead-3'), spawned: false } })
    const board = setupBoard()

    await board.bulkMoveStage(['lead-1', 'lead-2', 'lead-3'], 's2')
    await flushPromises()

    expect(toast.error).toHaveBeenCalledWith('Moved 2 of 3 leads (1 failed)')
    expect(fetchBoardMock).toHaveBeenCalledTimes(1)
  })
})

describe('useLeadsBoardStore date window', () => {
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
    const board = setupBoard()
    board.setDatePreset('today')
    await board.load()

    const params = fetchBoardMock.mock.calls[0][0]
    expect(params.from).toBe(new Date(2026, 8, 30).toISOString())
    expect(params.to).toBe(new Date(new Date(2026, 9, 1).getTime() - 1).toISOString())
  })

  it('sends a custom range from the start day to the end day', async () => {
    const board = setupBoard()
    board.setDateRange('2026-09-01', '2026-09-03')
    await board.load()

    const params = fetchBoardMock.mock.calls[0][0]
    expect(params.from).toBe(new Date(2026, 8, 1).toISOString())
    expect(params.to).toBe(new Date(new Date(2026, 8, 4).getTime() - 1).toISOString())
  })

  it('sends no window for all dates', async () => {
    const board = setupBoard()
    board.setDatePreset('today')
    board.setDatePreset('all')
    await board.load()

    const params = fetchBoardMock.mock.calls[0][0]
    expect(params.from).toBeUndefined()
    expect(params.to).toBeUndefined()
  })

  it('clears the window with the other filters', () => {
    const board = setupBoard()
    board.setDateRange('2026-09-01', '2026-09-03')
    expect(board.activeFilterCount).toBe(1)

    board.clearFilters()

    expect(board.datePreset).toBe('all')
    expect(board.fromDate).toBe('')
    expect(board.toDate).toBe('')
    expect(board.activeFilterCount).toBe(0)
  })
})
