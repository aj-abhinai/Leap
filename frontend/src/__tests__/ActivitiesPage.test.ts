import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ActivitiesPage from '@/views/ActivitiesPage.vue'
import { useActivitiesStore } from '@/stores/activities'
import { useSettingsStore } from '@/stores/settings'
import { useRemindersStore } from '@/stores/reminders'

describe('ActivitiesPage', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('resets to page one when switching views', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useActivitiesStore()
    store.fetchItems = vi.fn().mockResolvedValue(undefined)
    const settings = useSettingsStore()
    settings.fetchTags = vi.fn().mockResolvedValue(undefined)

    const wrapper = mount(ActivitiesPage, { global: { plugins: [pinia] } })
    await flushPromises()

    store.page = 3
    const overdue = wrapper.findAll('button').find((b) => b.text() === 'Overdue')
    expect(overdue).toBeTruthy()
    await overdue!.trigger('click')
    await flushPromises()

    expect(store.page).toBe(1)
    expect(store.fetchItems).toHaveBeenCalled()
  })

  it('clears the selection when changing pages', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useActivitiesStore()
    store.fetchItems = vi.fn().mockResolvedValue(undefined)
    const settings = useSettingsStore()
    settings.fetchTags = vi.fn().mockResolvedValue(undefined)

    store.total = 100
    store.perPage = 50
    store.items = [
      {
        id: 'a1',
        lead_id: 'l1',
        contact_id: 'c1',
        lead_display_name: 'Alice',
        stage_id: 's1',
        type: 'Call',
        description: '',
        is_done: false,
        is_cancelled: false,
        is_reminded: false,
        created_at: '2026-08-01T00:00:00Z',
      },
    ]

    const wrapper = mount(ActivitiesPage, { global: { plugins: [pinia] } })
    await flushPromises()

    await wrapper.find('tbody input[type="checkbox"]').trigger('change')
    // The mass-action bar appears only while something is selected.
    expect(wrapper.text()).toContain('1 selected')
    expect(wrapper.findAll('button').some((b) => b.text().includes('Delete'))).toBe(true)

    const next = wrapper.findAll('button').find((b) => b.text() === 'Next')
    expect(next).toBeTruthy()
    await next!.trigger('click')
    await flushPromises()

    expect(store.page).toBe(2)
    expect(wrapper.text()).toContain('0 selected')
    expect(wrapper.findAll('button').some((b) => b.text().includes('Delete'))).toBe(false)
  })

  it('routes an all-day snooze to a future day-grid target', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useActivitiesStore()
    store.fetchItems = vi.fn().mockResolvedValue(undefined)
    store.fetchRecent = vi.fn().mockResolvedValue(undefined)
    const settings = useSettingsStore()
    settings.fetchTags = vi.fn().mockResolvedValue(undefined)
    const reminders = useRemindersStore()
    const snoozeSpy = vi.fn().mockResolvedValue(undefined)
    reminders.snoozeReminder = snoozeSpy

    const wrapper = mount(ActivitiesPage, { global: { plugins: [pinia] } })
    await flushPromises()

    const nudge = new Date(2020, 0, 1, 9, 0, 0)
    const item = {
      id: 'a1',
      lead_id: 'l1',
      type: 'Call',
      scheduled_at: new Date(2020, 0, 1).toISOString(),
      scheduled_end_at: new Date(new Date(2020, 0, 2).getTime() - 1).toISOString(),
      remind_at: nudge.toISOString(),
    }
    const vm = wrapper.vm as unknown as {
      snoozeOptions: (i: unknown) => { label: string }[]
      doSnooze: (i: unknown, m: number) => Promise<void>
    }
    expect(vm.snoozeOptions(item).map((p) => p.label)).toEqual(['Tomorrow', 'In 2 days', 'Next week'])

    await vm.doSnooze(item, 24 * 60)
    expect(snoozeSpy).toHaveBeenCalledTimes(1)
    const target = new Date(snoozeSpy.mock.calls[0][2] as string)
    expect(target.getTime()).toBeGreaterThan(Date.now())
    expect(target.getHours()).toBe(nudge.getHours())
  })
})