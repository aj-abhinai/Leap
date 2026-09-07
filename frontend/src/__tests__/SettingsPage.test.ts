import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsPage from '@/views/SettingsPage.vue'
import { apiClient } from '@/composables/useApi'

vi.mock('@/composables/useApi', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  },
}))

let currentPerms: string[] = []
vi.mock('@/stores/rbac', () => ({
  useRBACStore: () => ({
    can: (permission: string) => currentPerms.includes('*') || currentPerms.includes(permission),
    fetchPermissions: async () => {},
  }),
}))

const getMock = vi.mocked(apiClient.get)

function mountPage() {
  return mount(SettingsPage, { global: { plugins: [createPinia()] } })
}

describe('SettingsPage visibility', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    currentPerms = ['*']
    getMock.mockReset()
    getMock.mockImplementation(async (url: string) => {
      if (url.startsWith('/api/tags')) return { data: [] }
      if (url.startsWith('/api/pipelines')) return { data: [] }
      if (url.startsWith('/api/programs')) return { data: [] }
      if (url.startsWith('/api/users')) return { data: [] }
      if (url.startsWith('/api/roles')) return { data: [] }
      if (url.startsWith('/api/permissions')) return { data: [] }
      if (url.startsWith('/api/activity')) return { data: [], meta: { page: 1, per_page: 20, total: 0 } }
      return { data: [] }
    })
  })

  it('shows every tab to a settings:manage holder with mutation controls', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const html = wrapper.html()
    for (const label of ['Contacts', 'Sales', 'Team', 'General', 'Audit log']) {
      expect(html).toContain(`>${label}</span>`)
    }
    // An admin lands on the Contacts tab and sees its mutation controls.
    expect(html).toContain('Tag name')
    expect(wrapper.text()).toContain('Add')
  })

  it('shows domain tabs read-only to a contact/lead reader without settings:manage', async () => {
    currentPerms = ['contact:read', 'lead:read']
    const wrapper = mountPage()
    await flushPromises()

    const html = wrapper.html()
    expect(html).toContain('>Contacts</span>')
    expect(html).toContain('>Sales</span>')
    expect(html).not.toContain('>Team</span>')
    expect(html).not.toContain('>General</span>')
    expect(html).not.toContain('>Audit log</span>')

    // Read-only: the vocabulary lists render without the add controls (the
    // add input is hidden), and the create cards are gone.
    expect(html).not.toContain('placeholder="Tag name"')
    expect(html).not.toContain('Add Pipeline')
    expect(html).not.toContain('Add Program')
    expect(wrapper.text()).not.toContain('Add')

    // Activating the Sales tab mounts the programs card, which fetches the
    // public lead:read list — never the settings:manage one. (The reka
    // trigger's click is not fired by jsdom in controlled mode, so the
    // model-value update is emitted directly.)
    getMock.mockClear()
    wrapper.findComponent({ name: 'Tabs' }).vm.$emit('update:model-value', 'sales')
    await flushPromises()
    const called = getMock.mock.calls.map((c) => String(c[0]))
    expect(called.some((u) => u === '/api/programs')).toBe(true)
    expect(called.some((u) => u.includes('/api/programs/manage'))).toBe(false)
  })

  it('shows the audit log tab to a data:export holder', async () => {
    currentPerms = ['data:export']
    const wrapper = mountPage()
    await flushPromises()

    const html = wrapper.html()
    expect(html).toContain('>Audit log</span>')
    expect(html).not.toContain('>Contacts</span>')
    expect(html).not.toContain('>Team</span>')
    // The export card is what the tab shows them.
    expect(html).toContain('Export CSV')
  })

  it('explains itself when no tab is visible', async () => {
    currentPerms = []
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.html()).toContain('Settings are not available for your role')
    expect(wrapper.html()).not.toContain('TabsList')
  })
})