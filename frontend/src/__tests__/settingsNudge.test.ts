import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSettingsStore } from '@/stores/settings'

// The org nudge read is mocked so the store's session behavior is pinned
// without transport.
vi.mock('@/api/settings', () => ({
  getNudgeLeadMinutes: vi.fn(),
}))

import { getNudgeLeadMinutes } from '@/api/settings'

const getNudgeMock = vi.mocked(getNudgeLeadMinutes)

describe('settings store nudge lead', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getNudgeMock.mockReset()
  })

  it('stores the org value after the first fetch', async () => {
    getNudgeMock.mockResolvedValue({ data: { minutes: 15 } })
    const store = useSettingsStore()

    await store.fetchNudgeLead()

    expect(store.nudgeLeadMinutes).toBe(15)
  })

  it('fetches once per session', async () => {
    getNudgeMock.mockResolvedValue({ data: { minutes: 15 } })
    const store = useSettingsStore()

    await store.fetchNudgeLead()
    await store.fetchNudgeLead()

    expect(getNudgeMock).toHaveBeenCalledTimes(1)
  })

  it('keeps null when the read fails', async () => {
    getNudgeMock.mockRejectedValue(new Error('offline'))
    const store = useSettingsStore()

    await store.fetchNudgeLead()

    expect(store.nudgeLeadMinutes).toBeNull()
  })

  it('keeps a saved value over a later fetch', async () => {
    getNudgeMock.mockResolvedValue({ data: { minutes: 15 } })
    const store = useSettingsStore()

    store.applyNudgeLead(20)
    await store.fetchNudgeLead()

    expect(store.nudgeLeadMinutes).toBe(20)
    expect(getNudgeMock).not.toHaveBeenCalled()
  })
})
