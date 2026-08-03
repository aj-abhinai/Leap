import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsTabUsers from '@/components/settings/SettingsTabUsers.vue'
import { apiClient } from '@/composables/useApi'

vi.mock('@/composables/useApi', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 'u-self' },
  }),
}))

const getMock = vi.mocked(apiClient.get)
const postMock = vi.mocked(apiClient.post)
const deleteMock = vi.mocked(apiClient.delete)

describe('SettingsTabUsers', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getMock.mockReset()
    postMock.mockReset()
    deleteMock.mockReset()

    getMock.mockImplementation(async (url: string) => {
      if (url === '/api/users') {
        return {
          data: [
            { id: 'u1', name: 'Alice', email: 'alice@example.com', roles: [{ id: 'r1', name: 'editor' }] },
            { id: 'u-self', name: 'Me', email: 'me@example.com', roles: [] },
          ],
        }
      }
      if (url === '/api/roles') {
        return {
          data: [
            { id: 'r1', name: 'editor' },
            { id: 'r2', name: 'manager' },
          ],
        }
      }
      return { data: [] }
    })
    postMock.mockResolvedValue({ data: { message: 'ok' } })
    deleteMock.mockResolvedValue({ data: { message: 'ok' } })
  })

  it('lists users with their role badges', async () => {
    const wrapper = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    expect(wrapper.html()).toContain('alice@example.com')
    expect(wrapper.html()).toContain('editor')
  })

  it('assigns a role through the users roles endpoint', async () => {
    const wrapper = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    const select = wrapper.find('select')
    await select.setValue('r2')
    await flushPromises()

    expect(postMock).toHaveBeenCalledWith('/api/users/u1/roles', { role_id: 'r2' })
  })

  it('removes a non-superadmin role from a user', async () => {
    const wrapper = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    const removeButton = wrapper.find('button[title="Remove role editor"]')
    expect(removeButton.exists()).toBe(true)
    await removeButton.trigger('click')
    await flushPromises()

    expect(deleteMock).toHaveBeenCalledWith('/api/users/u1/roles/r1')
  })
})
