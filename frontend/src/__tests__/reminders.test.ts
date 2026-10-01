import { describe, it, expect } from 'vitest'
import { formatReminderTime } from '@/utils/reminders'

// formatReminderTime resolves the schedule window first: an all-day day, then a
// range, then a point; the reminder-only fallback covers reply-only entries.
describe('formatReminderTime', () => {
  const start = new Date(2026, 8, 30, 13, 0).toISOString()
  const end = new Date(2026, 8, 30, 17, 0).toISOString()

  it('shows a range with both ends', () => {
    expect(formatReminderTime({ type: 'Call', scheduled_at: start, scheduled_end_at: end })).toBe(
      `Scheduled for ${new Date(start).toLocaleString()} – ${new Date(end).toLocaleString()}`,
    )
  })

  it('shows an all-day task as its date', () => {
    const dayStart = new Date(2026, 8, 30).toISOString()
    const dayEnd = new Date(new Date(2026, 9, 1).getTime() - 1).toISOString()
    expect(
      formatReminderTime({ type: 'Call', scheduled_at: dayStart, scheduled_end_at: dayEnd }),
    ).toBe(`Scheduled for ${new Date(dayStart).toLocaleDateString()} (all day)`)
  })

  it('shows a point task as its start', () => {
    expect(formatReminderTime({ type: 'Call', scheduled_at: start })).toBe(
      `Scheduled for ${new Date(start).toLocaleString()}`,
    )
  })

  it('falls back to the reminder for reply-only entries', () => {
    expect(formatReminderTime({ type: 'Call', remind_at: start })).toBe(
      `Reminder at ${new Date(start).toLocaleString()}`,
    )
  })

  it('is empty without a schedule or reminder', () => {
    expect(formatReminderTime({ type: 'Call' })).toBe('')
  })
})
