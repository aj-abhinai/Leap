import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ReminderCard from '@/components/leads/ReminderCard.vue'
import type { Reminder } from '@/api/reminders'

function makeReminder(overrides: Partial<Reminder> = {}): Reminder {
  return {
    id: 'r1',
    lead_id: 'l1',
    stage_id: 's1',
    type: 'Call',
    description: '',
    is_done: false,
    is_cancelled: false,
    is_reminded: false,
    created_at: new Date().toISOString(),
    ...overrides,
  }
}

// The reminder card resolves the schedule window first: an all-day day, then a
// range, then a point.
describe('ReminderCard', () => {
  it('shows a range with both ends', () => {
    const start = new Date(2026, 8, 30, 13, 0).toISOString()
    const end = new Date(2026, 8, 30, 17, 0).toISOString()
    const wrapper = mount(ReminderCard, {
      props: { reminder: makeReminder({ scheduled_at: start, scheduled_end_at: end }) },
    })
    expect(wrapper.text()).toContain(
      `Scheduled: ${new Date(start).toLocaleString()} – ${new Date(end).toLocaleString()}`,
    )
  })

  it('shows an all-day task as its date', () => {
    const start = new Date(2026, 8, 30).toISOString()
    const end = new Date(new Date(2026, 9, 1).getTime() - 1).toISOString()
    const wrapper = mount(ReminderCard, {
      props: { reminder: makeReminder({ scheduled_at: start, scheduled_end_at: end }) },
    })
    expect(wrapper.text()).toContain(`Scheduled: ${new Date(start).toLocaleDateString()} (all day)`)
  })
})
