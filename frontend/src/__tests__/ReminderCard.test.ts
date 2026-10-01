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

  it('offers day presets for an all-day reminder and minute presets for a timed one', () => {
    const dayStart = new Date(2026, 8, 30).toISOString()
    const dayEnd = new Date(new Date(2026, 9, 1).getTime() - 1).toISOString()
    const allDay = mount(ReminderCard, {
      props: {
        reminder: makeReminder({
          scheduled_at: dayStart,
          scheduled_end_at: dayEnd,
          remind_at: new Date(2026, 8, 30, 9, 0).toISOString(),
        }),
      },
    })
    const allDayOptions = (allDay.vm as unknown as { snoozeOptions: { label: string }[] }).snoozeOptions
    expect(allDayOptions.map((p) => p.label)).toEqual(['Tomorrow', 'In 2 days', 'Next week'])

    const timed = mount(ReminderCard, {
      props: { reminder: makeReminder({ remind_at: new Date().toISOString() }) },
    })
    const timedOptions = (timed.vm as unknown as { snoozeOptions: { label: string }[] }).snoozeOptions
    expect(timedOptions.map((p) => p.label)).toEqual(['15 minutes', '1 hour', '3 hours', 'Tomorrow'])
  })
})
