import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { useOrgTasks } from '@/composables/useOrgTasks'
import { useTasks } from '@/tasks/vue'
import { useSettingsStore } from '@/stores/settings'
import type { TaskEnv } from '@/tasks'

// The org nudge read is mocked so the provide/inject wiring is pinned without
// transport.
vi.mock('@/api/settings', () => ({
  getNudgeLeadMinutes: vi.fn(),
}))

import { getNudgeLeadMinutes } from '@/api/settings'

const getNudgeMock = vi.mocked(getNudgeLeadMinutes)

// The probe reads the env through the same useTasks() path every consumer
// uses, so a fallback env (default 5) fails the org-value assertions.
let captured: TaskEnv | null = null
const Probe = defineComponent({
  setup() {
    captured = useTasks().env
    return () => null
  },
})

const Host = defineComponent({
  components: { Probe },
  setup() {
    useOrgTasks()
  },
  template: '<Probe />',
})

function mountHost(pinia: Pinia) {
  mount(Host, { global: { plugins: [pinia] } })
}

describe('useOrgTasks', () => {
  beforeEach(() => {
    captured = null
    getNudgeMock.mockReset()
  })

  it('keeps the module default until the org value arrives', async () => {
    let resolve!: (value: { data: { minutes: number } }) => void
    getNudgeMock.mockReturnValue(
      new Promise<{ data: { minutes: number } }>((r) => {
        resolve = r
      }),
    )
    const pinia = createPinia()
    setActivePinia(pinia)

    mountHost(pinia)

    expect(captured?.defaults.nudgeLeadMinutes).toBe(5)
    resolve({ data: { minutes: 15 } })
    await flushPromises()
    expect(captured?.defaults.nudgeLeadMinutes).toBe(15)
  })

  it('serves the org value to descendants after the fetch', async () => {
    getNudgeMock.mockResolvedValue({ data: { minutes: 15 } })
    const pinia = createPinia()
    setActivePinia(pinia)

    mountHost(pinia)
    await flushPromises()

    expect(captured?.defaults.nudgeLeadMinutes).toBe(15)
  })

  it('follows a value saved in this session', async () => {
    getNudgeMock.mockResolvedValue({ data: { minutes: 15 } })
    const pinia = createPinia()
    setActivePinia(pinia)

    mountHost(pinia)
    await flushPromises()

    useSettingsStore().applyNudgeLead(20)
    await nextTick()

    expect(captured?.defaults.nudgeLeadMinutes).toBe(20)
  })
})
