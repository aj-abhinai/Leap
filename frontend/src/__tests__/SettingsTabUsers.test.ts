import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
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

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 'u-self' },
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

    const select = wrapper.find('select')
    await select.setValue('r2')
    await flushPromises()

    expect(putMock).toHaveBeenCalledWith('/api/users/u1/role', { role_id: 'r2' })
  })

  it('allows clearing a role to no role', async () => {
    const wrapper = mount(SettingsTabUsers, { global: { plugins: [createPinia()] } })
    await flushPromises()

    const select = wrapper.find('select')
    await select.setValue('')
    await flushPromises()

    expect(putMock).toHaveBeenCalledWith('/api/users/u1/role', { role_id: '' })
  })
})
