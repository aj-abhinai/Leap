import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import RemindersPage from '@/views/RemindersPage.vue'
import { apiClient } from '@/composables/useApi'
import type { Reminder } from '@/api/reminders'

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

function makeReminder(overrides: Partial<Reminder> = {}): Reminder {
  return {
    id: 'r1',
    lead_id: 'l1',
    contact_id: 'c1',
    lead_display_name: 'Alice',
    stage_id: 's1',
    stage_name: 'New',
    user_id: 'creator-1',
    user_name: 'Creator',
    type: 'Call',
    description: '',
    scheduled_at: new Date(Date.now() + 60 * 60_000).toISOString(),
    is_done: false,
    is_cancelled: false,
    is_reminded: false,
    created_at: new Date().toISOString(),
    ...overrides,
  }
}

describe('RemindersPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getMock.mockReset()
  })

  // /api/reminders already returns the current user's recipient queue, so the
  // page must not re-filter by task creator: a task assigned to the user but
  // created by someone else has to stay visible.
  it('renders the server recipient queue without a creator filter', async () => {
    getMock.mockResolvedValue({ data: [makeReminder({ user_id: 'someone-else' })] })

    const wrapper = mount(RemindersPage, {
      global: { plugins: [createPinia()], stubs: { ReminderCard: true } },
    })
    await flushPromises()

    expect(wrapper.text()).not.toContain('My reminders only')
    expect(wrapper.findComponent({ name: 'ReminderCard' }).exists()).toBe(true)
  })

  it('snoozes an all-day reminder to a future day of its own grid', async () => {
    const postMock = vi.mocked(apiClient.post)
    postMock.mockReset()
    postMock.mockResolvedValue({ data: null })
    getMock.mockResolvedValue({ data: [makeReminder()] })

    const wrapper = mount(RemindersPage, {
      global: { plugins: [createPinia()], stubs: { ReminderCard: true } },
    })
    await flushPromises()

    const nudge = new Date(2020, 0, 1, 9, 0, 0)
    const allDay = makeReminder({
      scheduled_at: new Date(2020, 0, 1).toISOString(),
      scheduled_end_at: new Date(new Date(2020, 0, 2).getTime() - 1).toISOString(),
      remind_at: nudge.toISOString(),
    })
    const vm = wrapper.vm as unknown as { snooze: (r: unknown, m: number) => Promise<void> }
    await vm.snooze(allDay, 24 * 60)

    expect(postMock).toHaveBeenCalledTimes(1)
    const body = postMock.mock.calls[0][1] as { remind_at: string }
    const target = new Date(body.remind_at)
    expect(target.getTime()).toBeGreaterThan(Date.now())
    expect(target.getHours()).toBe(nudge.getHours())
  })
})
