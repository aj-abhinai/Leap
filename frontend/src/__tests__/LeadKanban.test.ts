import { describe, it, expect, vi, afterEach, beforeAll, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick, type Component } from 'vue'
import LeadKanban from '@/components/leads/LeadKanban.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import draggable from 'vuedraggable'
import { useUsersStore } from '@/stores/users'
import { useRBACStore } from '@/stores/rbac'
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

function mountKanban(lead: Lead = makeLead({}), draggableStub: boolean | Component = true) {
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
      stubs: { draggable: draggableStub, ConfirmDialog: true },
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

  it('asks before closing an open lead from the card menu', async () => {
    const lead = makeLead({})
    // The default draggable stub swallows the item slot; render it so the
    // card (and its menu) exist in the DOM.
    const slotDraggable = {
      props: ['list'],
      template: '<div><div v-for="item in list" :key="item.id"><slot name="item" :element="item" /></div></div>',
    }
    const wrapper = mountKanban(lead, slotDraggable)
    const rbac = useRBACStore()
    rbac.permissions = ['lead:write']
    await flushPromises()

    const trigger = wrapper.get('button[aria-label="Lead actions"]')
    await trigger.trigger('click')
    await nextTick()
    await flushPromises()

    const items = Array.from(document.querySelectorAll('[role="menuitem"]'))
    const closeItem = items.find((el) => el.textContent?.includes('Move to Closed Lost'))
    expect(closeItem).toBeTruthy()

    closeItem!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()

    const dialog = wrapper.findComponent(ConfirmDialog)
    expect(dialog.props('open')).toBe(true)
    expect(dialog.props('title')).toContain('Closed Lost')
    expect(wrapper.emitted('moveStage')).toBeUndefined()
    wrapper.unmount()
  })
})

describe('LeadKanban collapsed rails', () => {
  // The item slot must render for card-visibility assertions; the default
  // draggable stub swallows it.
  const slotDraggable = {
    props: ['list'],
    template: '<div><div v-for="item in list" :key="item.id"><slot name="item" :element="item" /></div></div>',
  }

  function columnWidthPx(wrapper: ReturnType<typeof mountKanban>, index: number): string {
    return (wrapper.findAll('.card-in')[index].element as HTMLElement).style.width
  }

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

  // Collapse prefs persist to localStorage per pipeline; each test starts
  // from an empty board state.
  beforeEach(() => localStorage.clear())
  afterEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })

  it('collapses a column into a 44px rail with its name and count', async () => {
    const wrapper = mountKanban()
    await flushPromises()

    await wrapper.get('button[aria-label="Collapse Open"]').trigger('click')
    await nextTick()

    const col = wrapper.findAll('.card-in')[0]
    expect(columnWidthPx(wrapper, 0)).toBe('44px')
    expect(col.find('button[aria-label="Expand Open"]').exists()).toBe(true)
    expect(col.get('.rail-name').text()).toBe('Open')
    expect(col.text()).toContain('1')
  })

  it('hides cards, selection and add controls, and the resize handle while collapsed', async () => {
    const wrapper = mountKanban(makeLead({}), slotDraggable)
    const rbac = useRBACStore()
    rbac.permissions = ['lead:write']
    await flushPromises()

    expect(wrapper.text()).toContain('Alice')
    expect(wrapper.find('button[aria-label="Select all in Open"]').exists()).toBe(true)
    expect(wrapper.find('button[aria-label="Add lead to Open"]').exists()).toBe(true)
    expect(wrapper.findAll('.cursor-col-resize')).toHaveLength(4)

    await wrapper.get('button[aria-label="Collapse Open"]').trigger('click')
    await nextTick()

    expect(wrapper.text()).not.toContain('Alice')
    expect(wrapper.find('button[aria-label="Select all in Open"]').exists()).toBe(false)
    expect(wrapper.find('button[aria-label="Add lead to Open"]').exists()).toBe(false)
    expect(wrapper.findAll('.cursor-col-resize')).toHaveLength(3)
    // No draggable is mounted in the collapsed column, so it accepts no drops.
    expect(wrapper.findAllComponents(draggable)).toHaveLength(3)
  })

  it('expands the rail back to its previous width and cards', async () => {
    const wrapper = mountKanban(makeLead({}), slotDraggable)
    await flushPromises()

    await wrapper.get('button[aria-label="Collapse Open"]').trigger('click')
    await nextTick()
    expect(columnWidthPx(wrapper, 0)).toBe('44px')

    await wrapper.get('button[aria-label="Expand Open"]').trigger('click')
    await nextTick()

    expect(columnWidthPx(wrapper, 0)).toBe('288px')
    expect(wrapper.text()).toContain('Alice')
  })

  it('remembers the collapsed set under the per-pipeline preference key', async () => {
    const wrapper = mountKanban()
    await flushPromises()

    await wrapper.get('button[aria-label="Collapse Open"]').trigger('click')
    await nextTick()

    expect(JSON.parse(localStorage.getItem('crm:kanban:collapsed:p1') ?? '{}')).toEqual({
      'stage-open': true,
    })
  })
})
