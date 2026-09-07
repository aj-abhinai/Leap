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

vi.mock('@/stores/rbac', () => ({
  useRBACStore: () => ({
    can: (permission: string) => permission === 'settings:manage',
  }),
}))

const getMock = vi.mocked(apiClient.get)

describe('SettingsTabAudit user filter', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
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
    const wrapper = mount(SettingsTabAudit, { global: { plugins: [createPinia()] } })
    await flushPromises()

    // The actor dropdown is fed from the user list.
    expect(wrapper.html()).toContain('Alice')
    expect(wrapper.html()).toContain('Bob')

    // Choosing a user refetches the trail with the user_id filter.
    getMock.mockClear()
    const select = wrapper.find('select')
    await select.setValue('u2')
    await flushPromises()

    const activityCalls = getMock.mock.calls
      .map((c) => String(c[0]))
      .filter((u) => u.includes('/api/activity'))
    expect(activityCalls.some((u) => u.includes('user_id=u2'))).toBe(true)
    expect(activityCalls.some((u) => u.includes('user_id=u1'))).toBe(false)
  })

  it('requests no user filter by default', async () => {
    const wrapper = mount(SettingsTabAudit, { global: { plugins: [createPinia()] } })
    await flushPromises()

    const activityCalls = getMock.mock.calls
      .map((c) => String(c[0]))
      .filter((u) => u.includes('/api/activity'))
    expect(activityCalls.length).toBeGreaterThan(0)
    expect(activityCalls.some((u) => u.includes('user_id='))).toBe(false)
  })
})