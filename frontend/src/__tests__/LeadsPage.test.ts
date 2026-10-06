import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import LeadsPage from '@/views/LeadsPage.vue'
import { usePipelineStore, type Pipeline, type Stage } from '@/stores/pipeline'
import { useLeadsBoardStore } from '@/stores/leadsBoard'
import { useRBACStore } from '@/stores/rbac'
import { useUsersStore } from '@/stores/users'
import type { Lead } from '@/stores/leads'

vi.mock('@/api/leads', () => ({ fetchBoard: vi.fn() }))
vi.mock('@/api/contacts', () => ({ getContact: vi.fn() }))

const { toast } = vi.hoisted(() => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('vue-sonner', () => ({ toast }))

import { fetchBoard } from '@/api/leads'

const fetchBoardMock = vi.mocked(fetchBoard)

const stage: Stage = {
  id: 's1',
  pipeline_id: 'p1',
  name: 'New',
  order: 0,
  is_closing: false,
  outcome: 'open',
}

// mountPage wires the stores LeadsPage reads, mounts the view, and settles the
// initial board load. LeadKanban and LeadForm are stubbed so these tests
// exercise the page's own header, filter, and empty-state logic.
async function mountPage(pipelines: Pipeline[]) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const pipelineStore = usePipelineStore()
  pipelineStore.fetchPipelines = vi.fn().mockResolvedValue(undefined)
  pipelineStore.pipelines = pipelines
  const users = useUsersStore()
  users.fetchOptions = vi.fn().mockResolvedValue(undefined)
  const rbac = useRBACStore()
  rbac.permissions = ['lead:write']
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/settings', component: { template: '<div />' } },
    ],
  })
  await router.push('/')
  await router.isReady()
  const wrapper = mount(LeadsPage, {
    global: {
      plugins: [pinia, router],
      stubs: { LeadKanban: true, LeadForm: true },
    },
  })
  await flushPromises()
  return wrapper
}

describe('LeadsPage board feedback', () => {
  beforeEach(() => {
    fetchBoardMock.mockReset()
    fetchBoardMock.mockResolvedValue({ data: { stages: [] } })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('offers Add Lead only when the pipeline has a stage to create in', async () => {
    const noStages = await mountPage([{ id: 'p1', name: 'Sales', stages: [] }])
    expect(noStages.findAll('button').some((b) => b.text() === 'Add Lead')).toBe(false)
    noStages.unmount()

    const withStage = await mountPage([{ id: 'p1', name: 'Sales', stages: [stage] }])
    expect(withStage.findAll('button').some((b) => b.text() === 'Add Lead')).toBe(true)
    withStage.unmount()
  })

  it('shows the filtered-zero state and clears every filter in one click', async () => {
    fetchBoardMock.mockResolvedValue({
      data: { stages: [{ stage_id: 's1', count: 2, leads: [] }] },
    })
    const wrapper = await mountPage([{ id: 'p1', name: 'Sales', stages: [stage] }])
    const board = useLeadsBoardStore()

    board.search = 'zzz'
    await flushPromises()

    expect(wrapper.text()).toContain('No leads match your filters')
    const clear = wrapper.findAll('button').find((b) => b.text() === 'Clear filters')
    expect(clear).toBeTruthy()

    await clear!.trigger('click')
    expect(board.search).toBe('')
    expect(wrapper.text()).not.toContain('No leads match your filters')
  })

  it('reports the true stage count when the board caps the cards', async () => {
    const lead = {
      id: 'l1',
      stage_id: 's1',
      stage_outcome: 'open',
      display_name: 'Ann',
    } as Lead
    fetchBoardMock.mockResolvedValue({
      data: { stages: [{ stage_id: 's1', count: 5, leads: [lead] }] },
    })
    const wrapper = await mountPage([{ id: 'p1', name: 'Sales', stages: [stage] }])

    expect(wrapper.text()).toContain('Showing 1 of 5 leads')
  })
})
