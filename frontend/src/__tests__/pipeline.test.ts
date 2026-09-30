import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePipelineStore } from '@/stores/pipeline'
import { listPipelines, type Pipeline } from '@/api/pipelines'

vi.mock('@/api/pipelines', () => ({ listPipelines: vi.fn() }))

const pipeline: Pipeline = { id: 'p1', name: 'Default' }

describe('pipeline store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(listPipelines).mockReset()
  })

  it('fetches the catalog once and reuses it for the session', async () => {
    vi.mocked(listPipelines).mockResolvedValue({ data: [pipeline] })
    const store = usePipelineStore()

    await store.fetchPipelines()
    await store.fetchPipelines()

    expect(listPipelines).toHaveBeenCalledTimes(1)
    expect(store.pipelines).toEqual([pipeline])
  })

  it('force re-fetches after a mutation', async () => {
    vi.mocked(listPipelines).mockResolvedValue({ data: [pipeline] })
    const store = usePipelineStore()

    await store.fetchPipelines()
    await store.fetchPipelines(true)

    expect(listPipelines).toHaveBeenCalledTimes(2)
  })

  it('always re-fetches while the catalog is empty', async () => {
    vi.mocked(listPipelines).mockResolvedValue({ data: [] })
    const store = usePipelineStore()

    await store.fetchPipelines()
    await store.fetchPipelines()

    expect(listPipelines).toHaveBeenCalledTimes(2)
  })

  it('does not cache a failed fetch', async () => {
    vi.mocked(listPipelines).mockRejectedValueOnce(new Error('network down'))
    const store = usePipelineStore()

    await expect(store.fetchPipelines()).rejects.toThrow('network down')

    vi.mocked(listPipelines).mockResolvedValueOnce({ data: [pipeline] })
    await store.fetchPipelines()

    expect(listPipelines).toHaveBeenCalledTimes(2)
    expect(store.pipelines).toEqual([pipeline])
  })
})
