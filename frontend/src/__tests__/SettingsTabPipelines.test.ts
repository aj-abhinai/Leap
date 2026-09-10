import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsTabPipelines from '@/components/settings/SettingsTabPipelines.vue'
import { usePipelineStore } from '@/stores/pipeline'
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

vi.mock('vue-sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const getMock = vi.mocked(apiClient.get)
const patchMock = vi.mocked(apiClient.patch)

describe('SettingsTabPipelines edit', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getMock.mockReset()
    patchMock.mockReset()
    getMock.mockImplementation(async (url: string) => {
      if (url.startsWith('/api/pipelines')) {
        return {
          data: [
            {
              id: 'p1',
              name: 'Default',
              description: 'Old desc',
              stages: [
                { id: 's1', pipeline_id: 'p1', name: 'Open', order: 0, is_closing: false, outcome: 'open' },
              ],
            },
          ],
        }
      }
      return { data: [] }
    })
  })

  it('saves the pipeline name and description edit', async () => {
    const wrapper = mount(SettingsTabPipelines, { global: { plugins: [createPinia()] } })
    await flushPromises()

    await wrapper.find('button[title="Edit pipeline"]').trigger('click')
    await flushPromises()

    const inputs = wrapper.findAll('input')
    const nameInput = inputs.find((i) => (i.element as HTMLInputElement).value === 'Default')
    const descInput = inputs.find((i) => (i.element as HTMLInputElement).value === 'Old desc')
    await nameInput!.setValue('Renamed Pipeline')
    await descInput!.setValue('New desc')
    const saveButton = wrapper.findAll('button').find((b) => b.text().includes('Save'))
    await saveButton!.trigger('click')
    await flushPromises()

    expect(patchMock).toHaveBeenCalledWith('/api/pipelines/p1', {
      name: 'Renamed Pipeline',
      description: 'New desc',
    })
  })

  it('mutations refresh the shared pipeline store', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const wrapper = mount(SettingsTabPipelines, { global: { plugins: [pinia] } })
    await flushPromises()

    const store = usePipelineStore()
    expect(store.pipelines).toHaveLength(1)
    const fetchSpy = vi.spyOn(store, 'fetchPipelines')

    patchMock.mockResolvedValueOnce({ data: {} } as never)
    await wrapper.find('button[title="Edit pipeline"]').trigger('click')
    await flushPromises()
    const saveButton = wrapper.findAll('button').find((b) => b.text().includes('Save'))
    await saveButton!.trigger('click')
    await flushPromises()

    expect(fetchSpy).toHaveBeenCalled()
  })
})