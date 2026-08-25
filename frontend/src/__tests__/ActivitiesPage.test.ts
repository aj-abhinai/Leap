import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ActivitiesPage from '@/views/ActivitiesPage.vue'
import { useActivitiesStore } from '@/stores/activities'
import { useSettingsStore } from '@/stores/settings'

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
})