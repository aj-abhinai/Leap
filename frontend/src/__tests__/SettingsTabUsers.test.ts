import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsTabUsers from '@/components/settings/SettingsTabUsers.vue'
import { apiClient } from '@/composables/useApi'

vi.mock('@/composables/useApi', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
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

function selectByLabel(wrapper: VueWrapper, label: string) {
  const select = wrapper
    .findAllComponents({ name: 'Select' })
    .find((s) => s.find(`[aria-label="${label}"]`).exists())
  if (!select) throw new Error(`select "${label}" not found`)
  return select
}

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 'u-self' },
  }),
}))

vi.mock('@/stores/rbac', () => ({
  useRBACStore: () => ({
    can: (permission: string) => false,
  }),
}))

const getMock = vi.mocked(apiClient.get)
const postMock = vi.mocked(apiClient.post)
const putMock = vi.mocked(apiClient.put)
const deleteMock = vi.mocked(apiClient.delete)

describe('SettingsTabUsers', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getMock.mockReset()
    postMock.mockReset()
    putMock.mockReset()
    deleteMock.mockReset()

    getMock.mockImplementation(async (url: string) => {
      if (url === '/api/users') {
        return {
          data: [
            { id: 'u1', name: 'Alice', email: 'alice@example.com', role: { id: 'r1', name: 'editor' } },
            { id: 'u-self', name: 'Me', email: 'me@example.com', role: null },
          ],
        }
      }
      if (url === '/api/roles') {
        return {
          data: [
            { id: 'r1', name: 'editor' },
            { id: 'r2', name: 'manager' },
            { id: 'r3', name: 'superadmin' },
            { id: 'r4', name: 'Sales' },
          ],
        }
      }
      return { data: [] }
    })
    postMock.mockResolvedValue({ data: { message: 'ok' } })
    putMock.mockResolvedValue({ data: { message: 'ok' } })
    deleteMock.mockResolvedValue({ data: { message: 'ok' } })
  })

  it('lists users with their role badge', async () => {
    const wrapper = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    expect(wrapper.html()).toContain('alice@example.com')
    expect(wrapper.html()).toContain('editor')
  })

  it('sets a role through the single-role endpoint', async () => {
    const wrapper = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    const alice = selectByLabel(wrapper, 'Role for Alice')
    await alice.vm.$emit('update:model-value', 'r2')
    await flushPromises()

    expect(putMock).toHaveBeenCalledWith('/api/users/u1/role', { role_id: 'r2' })
  })

  it('allows clearing a role to no role', async () => {
    const wrapper = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    const alice = selectByLabel(wrapper, 'Role for Alice')
    // "No role" is the sentinel the Select carries; the component maps it
    // back to the API's explicit empty string.
    await alice.vm.$emit('update:model-value', '__none__')
    await flushPromises()

    expect(putMock).toHaveBeenCalledWith('/api/users/u1/role', { role_id: '' })
  })

  it('preselects Sales in the create form and sends it with the user', async () => {
    const wrapper = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    const createSelect = wrapper.findAllComponents({ name: 'Select' })[0]
    // Sales is preselected by default — no interaction needed.
    expect(createSelect.props('modelValue')).toBe('r4')
    const options = createSelect.findAllComponents({ name: 'SelectItem' }).map((o) => o.text())
    expect(options).toContain('Sales')
    expect(options).toContain('No role')

    await wrapper.find('input[placeholder="Name"]').setValue('New Rep')
    await wrapper.find('input[placeholder="Email"]').setValue('rep@example.com')
    await wrapper.find('input[placeholder*="Password"]').setValue('Strong-Pass-123')
    const addUser = wrapper.findAll('button').find((b) => b.text().includes('Add User'))
    expect(addUser).toBeTruthy()
    await addUser!.trigger('click')
    await flushPromises()

    expect(postMock).toHaveBeenCalledWith('/api/users', {
      name: 'New Rep',
      email: 'rep@example.com',
      password: 'Strong-Pass-123',
      role_id: 'r4',
    })

    // A second user starts from the Sales preselect again.
    expect(createSelect.props('modelValue')).toBe('r4')
  })

  it('hides the superadmin option from non-wildcard users', async () => {
    const wrapper = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    const selects = wrapper.findAllComponents({ name: 'Select' })
    expect(selects.length).toBeGreaterThan(0)
    // The first user (editor) and the second (no role) are not superadmins,
    // so neither dropdown may offer the superadmin option. The create form
    // is equally restricted.
    for (const select of selects) {
      const options = select.findAllComponents({ name: 'SelectItem' }).map((o) => o.text())
      expect(options).not.toContain('superadmin')
      expect(options).toContain('editor')
      expect(options).toContain('manager')
    }
  })

  it('deactivates a user and reactivates a deactivated one', async () => {
    const wrapper = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    // Deactivate Alice through the confirm dialog (rendered in the body
    // portal).
    await wrapper.find('button[aria-label="Deactivate Alice"]').trigger('click')
    await flushPromises()
    const confirm = [...document.body.querySelectorAll('button')].find((b) => b.textContent?.includes('Deactivate'))
    expect(confirm).toBeTruthy()
    confirm!.click()
    await flushPromises()

    expect(deleteMock).toHaveBeenCalledWith('/api/users/u1')
    wrapper.unmount()

    // A deactivated user (inactive) shows the reactivate action instead.
    getMock.mockImplementation(async (url: string) => {
      if (url === '/api/users') {
        return {
          data: [
            { id: 'u1', name: 'Alice', email: 'alice@example.com', role: null, active: false },
          ],
        }
      }
      if (url === '/api/roles') {
        return { data: [] }
      }
      return { data: [] }
    })
    const wrapper2 = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    expect(wrapper2.find('button[aria-label="Reactivate Alice"]').exists()).toBe(true)
    expect(wrapper2.find('button[aria-label="Deactivate Alice"]').exists()).toBe(false)
    expect(wrapper2.html()).toContain('Deactivated')
  })
})
