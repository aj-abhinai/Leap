import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsTagStatusCard from '@/components/settings/SettingsTagStatusCard.vue'
import { toast } from 'vue-sonner'

const updateTag = vi.fn()
const deleteTag = vi.fn()
vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    tags: [{ id: 't1', name: 'Student', color: '#ff0000' }],
    statuses: [],
    quickReplies: [
      {
        id: 'qr1',
        name: 'No Reply',
        type: 'quick_reply',
        group_name: 'Not Connected',
        sort_order: 1,
        behavior: 'log',
      },
    ],
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

// The real dialog and select render through reka-ui portals, which jsdom
// cannot open. The stubs render the same content inline so the edit fields
// and the save path are directly assertable.
vi.mock('@/components/ui/dialog', () => ({
  Dialog: { props: ['open'], template: '<div v-if="open"><slot /></div>' },
  DialogContent: { template: '<div><slot /></div>' },
  DialogHeader: { template: '<div><slot /></div>' },
  DialogTitle: { template: '<h2><slot /></h2>' },
  DialogDescription: { template: '<p><slot /></p>' },
  DialogFooter: { template: '<div><slot /></div>' },
}))

vi.mock('@/components/ui/select', () => ({
  Select: { template: '<div><slot /></div>' },
  SelectTrigger: { template: '<button type="button"><slot /></button>' },
  SelectValue: { template: '<span><slot /></span>' },
  SelectContent: { template: '<div><slot /></div>' },
  SelectItem: { template: '<div><slot /></div>' },
}))

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

describe('SettingsTagStatusCard quick reply editing', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    updateTag.mockReset()
    deleteTag.mockReset()
    vi.mocked(toast.success).mockClear()
  })

  async function openEditDialog() {
    const wrapper = mount(SettingsTagStatusCard, {
      props: { kind: 'quick_reply', title: 'Quick Replies', placeholder: 'e.g. No Reply, Busy' },
      global: { plugins: [createPinia()] },
    })
    await flushPromises()
    await wrapper.find('button[aria-label="Edit No Reply"]').trigger('click')
    await flushPromises()
    return wrapper
  }

  function inputByValue(wrapper: VueWrapper<any>, value: string) {
    return wrapper.findAll('input').find((i) => (i.element as HTMLInputElement).value === value)
  }

  function saveButton(wrapper: VueWrapper<any>) {
    return wrapper.findAll('button').find((b) => b.text().includes('Save'))
  }

  it('renames a quick reply and reports the right entity', async () => {
    const wrapper = await openEditDialog()

    const nameInput = inputByValue(wrapper, 'No Reply')
    expect(nameInput).toBeTruthy()
    await nameInput!.setValue('Busy')

    await saveButton(wrapper)!.trigger('click')
    await flushPromises()

    expect(updateTag).toHaveBeenCalledWith('qr1', {
      name: 'Busy',
      group_name: 'Not Connected',
      sort_order: 1,
      behavior: 'log',
    })
    expect(toast.success).toHaveBeenCalledWith('Quick reply updated')
  })

  it('refuses a blank name instead of sending the update', async () => {
    const wrapper = await openEditDialog()

    await inputByValue(wrapper, 'No Reply')!.setValue('')
    await saveButton(wrapper)!.trigger('click')
    await flushPromises()

    expect(updateTag).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Name is required')
  })

  it('refuses a cleared sort order instead of sending the update', async () => {
    const wrapper = await openEditDialog()

    const sortInput = inputByValue(wrapper, '1')
    expect(sortInput).toBeTruthy()
    await sortInput!.setValue('')

    await saveButton(wrapper)!.trigger('click')
    await flushPromises()

    expect(updateTag).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Sort order must be a whole number')
  })

  it('refuses a decimal sort order instead of sending the update', async () => {
    const wrapper = await openEditDialog()

    const sortInput = inputByValue(wrapper, '1')
    expect(sortInput).toBeTruthy()
    await sortInput!.setValue('1.5')

    await saveButton(wrapper)!.trigger('click')
    await flushPromises()

    expect(updateTag).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Sort order must be a whole number')
  })

  it('refuses a sort order too large for the API integer decode', async () => {
    const wrapper = await openEditDialog()

    const sortInput = inputByValue(wrapper, '1')
    expect(sortInput).toBeTruthy()
    await sortInput!.setValue('9007199254740993')

    await saveButton(wrapper)!.trigger('click')
    await flushPromises()

    expect(updateTag).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Sort order must be a whole number')
  })
})
