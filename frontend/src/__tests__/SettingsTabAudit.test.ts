import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsTabAudit from '@/components/settings/SettingsTabAudit.vue'
import { apiClient } from '@/composables/useApi'

vi.mock('@/composables/useApi', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

// The real ui/select renders its items through a portal that only mounts when
// the dropdown is open, which jsdom cannot do; stub the module so options
// render inline and the model can be driven from the tests. The Select stub
// keeps reka's contract: an update:modelValue emit on the root.
vi.mock('@/components/ui/select', () => ({
  Select: {
    name: 'Select',
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template: '<div><slot /></div>',
  },
  SelectTrigger: { name: 'SelectTrigger', template: '<button type="button"><slot /></button>' },
  SelectValue: { name: 'SelectValue', template: '<span><slot /></span>' },
  SelectContent: { name: 'SelectContent', template: '<div><slot /></div>' },
  SelectItem: { name: 'SelectItem', template: '<div><slot /></div>' },
}))

// The section row and the content both read permissions, so each test sets
// the holder it exercises.
const { perms } = vi.hoisted(() => ({ perms: { current: [] as string[] } }))

vi.mock('@/stores/rbac', () => ({
  useRBACStore: () => ({
    can: (permission: string) => perms.current.includes('*') || perms.current.includes(permission),
  }),
}))

const getMock = vi.mocked(apiClient.get)

function mountTab() {
  return mount(SettingsTabAudit, { global: { plugins: [createPinia()] } })
}

function switchSection(wrapper: ReturnType<typeof mountTab>, value: string) {
  wrapper.findComponent({ name: 'Tabs' }).vm.$emit('update:model-value', value)
  return flushPromises()
}

describe('SettingsTabAudit user filter', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    perms.current = ['settings:manage']
    getMock.mockReset()
    getMock.mockImplementation(async (url: string) => {
      if (url.startsWith('/api/users')) {
        return { data: [{ id: 'u1', name: 'Alice' }, { id: 'u2', name: 'Bob' }] }
      }
      if (url.startsWith('/api/activity')) {
        return { data: [], meta: { page: 1, per_page: 20, total: 0 } }
      }
      return { data: [] }
    })
  })

  it('loads the users for the actor filter and applies the user_id param', async () => {
    const wrapper = mountTab()
    await flushPromises()

    // The actor dropdown is fed from the user list.
    expect(wrapper.html()).toContain('Alice')
    expect(wrapper.html()).toContain('Bob')

    // Choosing a user refetches the trail with the user_id filter.
    getMock.mockClear()
    const actor = wrapper.findAllComponents({ name: 'Select' })[0]
    await actor.vm.$emit('update:model-value', 'u2')
    await flushPromises()

    const activityCalls = getMock.mock.calls
      .map((c) => String(c[0]))
      .filter((u) => u.includes('/api/activity'))
    expect(activityCalls.some((u) => u.includes('user_id=u2'))).toBe(true)
    expect(activityCalls.some((u) => u.includes('user_id=u1'))).toBe(false)
  })

  it('requests no user filter by default', async () => {
    const wrapper = mountTab()
    await flushPromises()

    const activityCalls = getMock.mock.calls
      .map((c) => String(c[0]))
      .filter((u) => u.includes('/api/activity'))
    expect(activityCalls.length).toBeGreaterThan(0)
    expect(activityCalls.some((u) => u.includes('user_id='))).toBe(false)
  })
})

describe('SettingsTabAudit section permissions', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getMock.mockReset()
    getMock.mockImplementation(async () => ({ data: [], meta: { page: 1, per_page: 20, total: 0 } }))
  })

  it('opens the trail for a settings:manage holder and offers no Export section', async () => {
    perms.current = ['settings:manage']
    const wrapper = mountTab()
    await flushPromises()

    expect(wrapper.text()).toContain('Audit Log')
    expect(wrapper.text()).not.toContain('Export CSV')
    expect(wrapper.findComponent({ name: 'Select' }).exists()).toBe(true)
  })

  it('opens Export for a data:export holder and fetches nothing for the trail', async () => {
    perms.current = ['data:export']
    const wrapper = mountTab()
    await flushPromises()

    expect(wrapper.text()).toContain('Export CSV')
    expect(wrapper.text()).not.toContain('Audit Log')

    const called = getMock.mock.calls.map((c) => String(c[0]))
    expect(called.some((u) => u.includes('/api/activity'))).toBe(false)
    expect(called.some((u) => u.includes('/api/users'))).toBe(false)
  })

  it('opens the trail first when the holder has both permissions', async () => {
    perms.current = ['settings:manage', 'data:export']
    const wrapper = mountTab()
    await flushPromises()

    expect(wrapper.text()).toContain('Audit Log')
    expect(wrapper.text()).not.toContain('Export CSV')
    expect(wrapper.findComponent({ name: 'Select' }).exists()).toBe(true)

    await switchSection(wrapper, 'export')

    expect(wrapper.text()).toContain('Export CSV')
    // The trail's actor filter is gone with its section.
    expect(wrapper.text()).not.toContain('All users')
  })
})
