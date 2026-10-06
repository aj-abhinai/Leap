import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/tags', () => ({
  listTags: vi.fn(),
}))

import { listTags, type Tag } from '@/api/tags'
import { useSettingsStore } from '@/stores/settings'

const listTagsMock = vi.mocked(listTags)

function tag(type: string): Tag {
  return { id: type, name: type, type, sort_order: 0, behavior: 'log', created_at: '2026-01-01T00:00:00Z' }
}

describe('settings store fetchTags', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    listTagsMock.mockReset()
  })

  // One rejected list must not blank the other four; loadError names it.
  it('settles each list independently', async () => {
    listTagsMock.mockImplementation((type: string) =>
      type === 'status'
        ? Promise.reject(new Error('boom'))
        : Promise.resolve({ data: [tag(type)] }),
    )
    const store = useSettingsStore()

    await store.fetchTags()

    expect(store.tags).toHaveLength(1)
    expect(store.quickReplies).toHaveLength(1)
    expect(store.activityTypes).toHaveLength(1)
    expect(store.lossReasons).toHaveLength(1)
    expect(store.statuses).toEqual([])
    expect(store.loadError).toContain('statuses')
    expect(store.loading).toBe(false)
  })

  it('clears loadError after a fully successful fetch', async () => {
    listTagsMock.mockImplementation((type: string) => Promise.resolve({ data: [tag(type)] }))
    const store = useSettingsStore()

    await store.fetchTags()

    expect(store.statuses).toHaveLength(1)
    expect(store.loadError).toBe('')
  })
})