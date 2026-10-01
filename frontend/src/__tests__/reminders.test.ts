import { describe, it, expect } from 'vitest'
import { formatReminderTime, snoozeTarget } from '@/utils/reminders'
import { allDayRange, isAllDayRange } from '@/utils/time'

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

// snoozeTarget resolves the new reminder moment. An all-day task moves by
// whole days from its own nudge, so the window keeps the day shape.
describe('snoozeTarget', () => {
  it('moves a timed task to now + minutes', () => {
    const before = Date.now()
    const got = new Date(snoozeTarget({ type: 'Call' }, 15)).getTime()
    expect(got).toBeGreaterThanOrEqual(before + 15 * 60_000 - 1_000)
    expect(got).toBeLessThanOrEqual(Date.now() + 15 * 60_000 + 1_000)
  })

  it('lands an overdue all-day task on a future day of its own grid', () => {
    // The task day is long past, so its stored nudge is in the past too. The
    // target must still be in the future: the backend rejects a past snooze.
    const day = allDayRange('2020-01-01')!
    const got = new Date(
      snoozeTarget(
        { type: 'Call', scheduled_at: day.start, scheduled_end_at: day.end, remind_at: day.remind },
        24 * 60,
      ),
    )
    const nudge = new Date(day.remind)
    expect(got.getTime()).toBeGreaterThan(Date.now())
    expect(got.getHours()).toBe(nudge.getHours())
    expect(got.getMinutes()).toBe(nudge.getMinutes())

    const expectedDay = new Date()
    expectedDay.setDate(expectedDay.getDate() + 1)
    expect([got.getFullYear(), got.getMonth(), got.getDate()]).toEqual([
      expectedDay.getFullYear(),
      expectedDay.getMonth(),
      expectedDay.getDate(),
    ])

    // The backend shifts the window by the same delta; whole days keep the
    // shape.
    const delta = got.getTime() - nudge.getTime()
    const shiftedStart = new Date(new Date(day.start).getTime() + delta).toISOString()
    const shiftedEnd = new Date(new Date(day.end).getTime() + delta).toISOString()
    expect(isAllDayRange(shiftedStart, shiftedEnd)).toBe(true)
  })

  it('falls back to now + minutes for an all-day shape without a nudge', () => {
    const day = allDayRange('2026-09-30')!
    const before = Date.now()
    const got = new Date(
      snoozeTarget({ type: 'Call', scheduled_at: day.start, scheduled_end_at: day.end }, 15),
    ).getTime()
    expect(got).toBeGreaterThanOrEqual(before + 15 * 60_000 - 1_000)
  })

  it('maps each day preset to its own day count', () => {
    const day = allDayRange('2020-01-01')!
    const base = { type: 'Call', scheduled_at: day.start, scheduled_end_at: day.end, remind_at: day.remind }
    const nudge = new Date(day.remind)
    const expected = (days: number) => {
      const d = new Date()
      d.setDate(d.getDate() + days)
      d.setHours(nudge.getHours(), nudge.getMinutes(), nudge.getSeconds(), nudge.getMilliseconds())
      return d.toISOString()
    }
    expect(snoozeTarget(base, 2 * 24 * 60)).toBe(expected(2))
    expect(snoozeTarget(base, 7 * 24 * 60)).toBe(expected(7))
  })
})
