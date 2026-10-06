import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import LeadActivityDrawer from '@/components/leads/LeadActivityDrawer.vue'
import { useLeadDrawerGlobal } from '@/composables/useLeadDrawerGlobal'
import { useLeadsBoardStore } from '@/stores/leadsBoard'
import { getLead, fetchBoard, type Lead } from '@/api/leads'

vi.mock('@/api/leads', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/leads')>()
  return { ...actual, getLead: vi.fn(), fetchBoard: vi.fn() }
})

vi.mock('@/stores/rbac', () => ({
  useRBACStore: () => ({ can: () => true }),
}))

vi.mock('@/stores/pipeline', () => ({
  usePipelineStore: () => ({ pipelines: [], fetchPipelines: vi.fn() }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

function makeLead(overrides: Partial<Lead> = {}): Lead {
  return {
    id: 'l1',
    display_name: 'Alice',
    contact_id: 'c1',
    pipeline_id: 'p1',
    stage_id: 's-open',
    stage_outcome: 'open',
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-01T00:00:00Z',
    ...overrides,
  }
}

describe('LeadActivityDrawer', () => {
  let pinia: Pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.mocked(getLead).mockReset()
    vi.mocked(fetchBoard).mockReset()
    const { drawerOpen, drawerLeadId, drawerLead } = useLeadDrawerGlobal()
    drawerOpen.value = false
    drawerLeadId.value = ''
    drawerLead.value = null
    document.body.innerHTML = ''
  })

  function mountDrawer() {
    return mount(LeadActivityDrawer, {
      global: {
        plugins: [pinia],
        stubs: {
          LeadActivityForm: {
            template: '<button data-testid="activity-form" @click="$emit(\'saved\')" />',
          },
          LeadActivity: true,
          LeadStageHistory: true,
        },
      },
      attachTo: document.body,
    })
  }

  function openDrawerWith(lead: Lead) {
    const { drawerOpen, drawerLeadId } = useLeadDrawerGlobal()
    vi.mocked(getLead).mockResolvedValue({ data: lead })
    drawerLeadId.value = lead.id
    drawerOpen.value = true
  }

  it('hides the Log Task form and shows the closed notice for a closed lead', async () => {
    openDrawerWith(makeLead({ stage_id: 's-lost', stage_outcome: 'lost' }))
    const wrapper = mountDrawer()
    await flushPromises()
    await nextTick()

    expect(document.body.querySelector('[data-testid="activity-form"]')).toBeNull()
    expect(document.body.textContent).toContain('This deal is closed. Tasks are read-only.')
    wrapper.unmount()
  })

  it('renders the Log Task form for an open lead', async () => {
    openDrawerWith(makeLead())
    const wrapper = mountDrawer()
    await flushPromises()
    await nextTick()

    expect(document.body.querySelector('[data-testid="activity-form"]')).not.toBeNull()
    expect(document.body.textContent).not.toContain('This deal is closed. Tasks are read-only.')
    wrapper.unmount()
  })

  // The drawer and the page share one board store; a save must refresh the
  // board the page renders, not a private copy.
  it('refreshes the shared board after a task save', async () => {
    openDrawerWith(makeLead())
    const wrapper = mountDrawer()
    await flushPromises()
    await nextTick()

    const board = useLeadsBoardStore()
    board.selectedPipelineId = 'p1'
    vi.mocked(fetchBoard).mockResolvedValue({ data: { stages: [] } })

    const form = document.body.querySelector<HTMLButtonElement>('[data-testid="activity-form"]')
    expect(form).not.toBeNull()
    form!.click()
    await flushPromises()

    expect(fetchBoard).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
})
