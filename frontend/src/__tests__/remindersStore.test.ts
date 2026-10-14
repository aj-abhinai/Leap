import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/reminders', () => ({
  listReminders: vi.fn(),
  dismissReminder: vi.fn(),
  snoozeReminder: vi.fn(),
}))

import { listReminders, type Reminder } from '@/api/reminders'
import { useRemindersStore } from '@/stores/reminders'

const listMock = vi.mocked(listReminders)

function reminder(id: string): Reminder {
  return {
    id,
    lead_id: 'l1',
    stage_id: 's1',
    type: 'Call',
    description: '',
    is_done: false,
    is_cancelled: false,
    is_reminded: false,
    created_at: '2026-01-01T00:00:00Z',
  }
}

describe('reminders store fetchReminders', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    listMock.mockReset()
  })

  // The bell poller and the page mount call this fire-and-forget, so a
  // failed refresh must settle: the last known list stays and the failure
  // is reported through loadError instead of a rejection.
  it('settles on failure: keeps the list and sets loadError', async () => {
    const store = useRemindersStore()
    listMock.mockResolvedValueOnce({ data: [reminder('r1')] })
    await store.fetchReminders()

    listMock.mockRejectedValueOnce(new Error('boom'))
    await expect(store.fetchReminders()).resolves.toBeUndefined()

    expect(store.reminders.map((r) => r.id)).toEqual(['r1'])
    expect(store.loadError).toBe('Failed to refresh reminders')
    expect(store.loading).toBe(false)
  })

  it('clears loadError after a successful fetch and updates the list', async () => {
    const store = useRemindersStore()
    listMock.mockResolvedValue({ data: [reminder('r2')] })

    await store.fetchReminders()

    expect(store.reminders.map((r) => r.id)).toEqual(['r2'])
    expect(store.loadError).toBe('')
    expect(store.loading).toBe(false)
  })
})
