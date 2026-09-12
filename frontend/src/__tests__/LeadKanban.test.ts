import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import LeadKanban from '@/components/leads/LeadKanban.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import draggable from 'vuedraggable'
import { useUsersStore } from '@/stores/users'
import type { Lead } from '@/stores/leads'
import type { Stage } from '@/stores/pipeline'

const openStage: Stage = {
  id: 'stage-open',
  pipeline_id: 'p1',
  name: 'Open',
  order: 0,
  is_closing: false,
  outcome: 'open',
}
const openStageB: Stage = {
  id: 'stage-open-b',
  pipeline_id: 'p1',
  name: 'Follow-up',
  order: 1,
  is_closing: false,
  outcome: 'open',
}
const lostStage: Stage = {
  id: 'stage-lost',
  pipeline_id: 'p1',
  name: 'Closed Lost',
  order: 2,
  is_closing: true,
  outcome: 'lost',
}
const wonStage: Stage = {
  id: 'stage-won',
  pipeline_id: 'p1',
  name: 'Converted',
  order: 3,
  is_closing: true,
  outcome: 'won',
}

function makeLead(overrides: Partial<Lead>): Lead {
  return {
    id: 'lead-1',
    stage_id: 'stage-open',
    stage_outcome: 'open',
    display_name: 'Alice',
    ...overrides,
  } as Lead
}

function mountKanban(lead: Lead = makeLead({})) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const users = useUsersStore()
  users.fetchOptions = vi.fn().mockResolvedValue(undefined)
  return mount(LeadKanban, {
    props: {
      pipelineId: 'p1',
      stages: [openStage, openStageB, lostStage, wonStage],
      columns: [
        { ...openStage, leads: [lead], count: 1 },
        { ...openStageB, leads: [], count: 0 },
        { ...lostStage, leads: [], count: 0 },
        { ...wonStage, leads: [], count: 0 },
      ],
    },
    global: {
      plugins: [pinia],
      stubs: { draggable: true, ConfirmDialog: true },
    },
  })
}

async function dropInto(wrapper: ReturnType<typeof mountKanban>, columnIndex: number, lead: Lead) {
  const drags = wrapper.findAllComponents(draggable)
  await drags[columnIndex].vm.$emit('change', { added: { element: lead } })
  await nextTick()
}

describe('LeadKanban close confirmation', () => {
  // jsdom has no ResizeObserver; the board observes its scroller on mount.
  beforeAll(() => {
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    )
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('asks before closing an open lead and moves it on confirm', async () => {
    const lead = makeLead({})
    const wrapper = mountKanban(lead)
    await flushPromises()

    await dropInto(wrapper, 2, lead)

    const dialog = wrapper.findComponent(ConfirmDialog)
    expect(dialog.props('open')).toBe(true)
    expect(dialog.props('title')).toContain('Closed Lost')
    expect(wrapper.emitted('moveStage')).toBeUndefined()

    dialog.vm.$emit('confirm')
    await nextTick()

    expect(wrapper.emitted('moveStage')).toEqual([['lead-1', 'stage-lost', 'stage-open']])
    expect(dialog.props('open')).toBe(false)

    // The dialog then reports itself closed; a confirmed move must not reload.
    dialog.vm.$emit('update:open', false)
    await nextTick()
    expect(wrapper.emitted('reload')).toBeUndefined()
  })

  it('reloads the board without moving when the close confirmation is cancelled', async () => {
    const lead = makeLead({})
    const wrapper = mountKanban(lead)
    await flushPromises()

    await dropInto(wrapper, 2, lead)
    expect(wrapper.findComponent(ConfirmDialog).props('open')).toBe(true)

    wrapper.findComponent(ConfirmDialog).vm.$emit('cancel')
    await nextTick()

    expect(wrapper.emitted('moveStage')).toBeUndefined()
    expect(wrapper.emitted('reload')).toHaveLength(1)
    expect(wrapper.findComponent(ConfirmDialog).props('open')).toBe(false)

    // A cancel click reaches both the dialog's own close and the wrapper's
    // cancel; the board must reload once.
    wrapper.findComponent(ConfirmDialog).vm.$emit('update:open', false)
    await nextTick()
    expect(wrapper.emitted('reload')).toHaveLength(1)
  })

  it('reloads once when the dialog is dismissed without a button', async () => {
    const lead = makeLead({})
    const wrapper = mountKanban(lead)
    await flushPromises()

    await dropInto(wrapper, 2, lead)
    const dialog = wrapper.findComponent(ConfirmDialog)
    expect(dialog.props('open')).toBe(true)

    dialog.vm.$emit('update:open', false)
    await nextTick()

    expect(wrapper.emitted('moveStage')).toBeUndefined()
    expect(wrapper.emitted('reload')).toHaveLength(1)
    expect(dialog.props('open')).toBe(false)
  })

  it('moves an open lead into another open stage without confirmation', async () => {
    const lead = makeLead({})
    const wrapper = mountKanban(lead)
    await flushPromises()

    await dropInto(wrapper, 1, lead)

    expect(wrapper.findComponent(ConfirmDialog).props('open')).toBe(false)
    expect(wrapper.emitted('moveStage')).toEqual([['lead-1', 'stage-open-b', 'stage-open']])
  })

  it('spawns a cycle from a closed lead without confirmation', async () => {
    const lead = makeLead({ stage_id: 'stage-lost', stage_outcome: 'lost' })
    const wrapper = mountKanban(lead)
    await flushPromises()

    await dropInto(wrapper, 0, lead)

    expect(wrapper.findComponent(ConfirmDialog).props('open')).toBe(false)
    expect(wrapper.emitted('moveStage')).toEqual([['lead-1', 'stage-open', 'stage-lost']])
  })

  it('moves a closed lead between closing stages without confirmation', async () => {
    const lead = makeLead({ stage_id: 'stage-lost', stage_outcome: 'lost' })
    const wrapper = mountKanban(lead)
    await flushPromises()

    await dropInto(wrapper, 3, lead)

    expect(wrapper.findComponent(ConfirmDialog).props('open')).toBe(false)
    expect(wrapper.emitted('moveStage')).toEqual([['lead-1', 'stage-won', 'stage-lost']])
  })
})
