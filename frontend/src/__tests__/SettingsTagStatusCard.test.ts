import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsTagStatusCard from '@/components/settings/SettingsTagStatusCard.vue'

const updateTag = vi.fn()
const deleteTag = vi.fn()
vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    tags: [{ id: 't1', name: 'Student', color: '#ff0000' }],
    statuses: [],
    quickReplies: [],
    activityTypes: [],
    lossReasons: [],
    loading: false,
    fetchTags: vi.fn(),
    createTag: vi.fn(),
    updateTag,
    deleteTag,
  }),
}))

vi.mock('vue-sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

describe('SettingsTagStatusCard color editing', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    updateTag.mockReset()
    deleteTag.mockReset()
  })

  it('clears a tag color through the clear button', async () => {
    const wrapper = mount(SettingsTagStatusCard, {
      props: { kind: 'tag', title: 'Tags', placeholder: 'Tag name' },
      global: { plugins: [createPinia()] },
    })
    await flushPromises()

    expect(wrapper.find('input[type="color"]').exists()).toBe(true)
    await wrapper.find('button[aria-label="Clear color for Student"]').trigger('click')
    await flushPromises()

    expect(updateTag).toHaveBeenCalledWith('t1', { color: '' })
  })

  it('hides the color control entirely in readonly mode', async () => {
    const wrapper = mount(SettingsTagStatusCard, {
      props: { kind: 'tag', title: 'Tags', placeholder: 'Tag name', readonly: true },
      global: { plugins: [createPinia()] },
    })
    await flushPromises()

    expect(wrapper.find('input[type="color"]').exists()).toBe(false)
    expect(wrapper.find('button[aria-label="Clear color for Student"]').exists()).toBe(false)
  })
})