import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { SidebarProvider } from '@/components/ui/sidebar'
import AppSidebar from '@/components/layout/AppSidebar.vue'

let currentPerms: string[] = []
vi.mock('@/stores/rbac', () => ({
  useRBACStore: () => ({
    can: (permission: string) => currentPerms.includes('*') || currentPerms.includes(permission),
    fetchPermissions: async () => {},
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: 'u1', name: 'Test' }, logout: vi.fn() }),
}))

vi.mock('@/router', () => ({ default: { push: vi.fn() } }))

vi.mock('@/api/reminders', () => ({
  listReminders: async () => ({ data: [] }),
}))

vi.mock('@/composables/useApi', () => ({
  apiClient: { get: vi.fn(), post: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))

function mountSidebar() {
  return mount(SidebarProvider, {
    slots: { default: AppSidebar },
    global: { plugins: [createPinia()] },
  })
}

describe('AppSidebar settings nav visibility', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    currentPerms = ['*']
  })

  it('shows the Settings item to a user with any visible tab', async () => {
    currentPerms = ['contact:read']
    const wrapper = mountSidebar()
    await flushPromises()

    expect(wrapper.text()).toContain('Settings')
    expect(wrapper.text()).toContain('Tasks')
  })

  it('shows the Settings item to a data:export-only user', async () => {
    currentPerms = ['data:export']
    const wrapper = mountSidebar()
    await flushPromises()

    expect(wrapper.text()).toContain('Settings')
  })

  it('hides the Settings item for a user with no visible tab', async () => {
    currentPerms = []
    const wrapper = mountSidebar()
    await flushPromises()

    expect(wrapper.text()).not.toContain('Settings')
  })

  // The click writes the sidebar_state cookie that SidebarProvider reads for
  // defaultOpen; clear it so later mounts still start expanded.
  it('collapses the sidebar from the in-sidebar control', async () => {
    const wrapper = mountSidebar()
    await flushPromises()

    expect(wrapper.find('[data-slot="sidebar"]').attributes('data-state')).toBe('expanded')
    await wrapper.find('button[aria-label="Collapse sidebar"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-slot="sidebar"]').attributes('data-state')).toBe('collapsed')

    document.cookie = 'sidebar_state=; path=/; max-age=0'
  })
})