import { describe, it, expect } from 'vitest'
import { allDayRange, dateWindow, dayWindow, isAllDayRange, presetWindow, toLocalDateInput, toLocalTimeInput, timeAgo } from '@/utils/time'

// A fixed local instant: Wednesday 30 September 2026, 12:00. Expectations are
// built from the same local-date primitives, so they hold in any time zone.
const now = new Date(2026, 8, 30, 12, 0, 0)
const endOfToday = new Date(new Date(2026, 9, 1).getTime() - 1).toISOString()

// These helpers render a stored instant into local wall-clock components for
// <input type="date"> / <input type="time">, matching the local-time semantics
// of mergeDateTime (local wall-clock in, UTC instant out). Wall-clock output
// depends on the environment's zone, so the output shape is asserted and the
// local semantics are verified by round-tripping (local wall-clock back to an
// instant); both stay zone-agnostic without re-encoding the getters.
describe('toLocalDateInput / toLocalTimeInput', () => {
  it('formats as zero-padded YYYY-MM-DD and HH:mm', () => {
    expect(toLocalDateInput('2026-01-05T06:07:00Z')).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(toLocalTimeInput('2026-01-05T06:07:00Z')).toMatch(/^\d{2}:\d{2}$/)
  })

  it('round-trips through mergeDateTime semantics (local -> UTC instant)', () => {
    const stored = '2026-08-06T09:30:00Z'
    const date = toLocalDateInput(stored)
    const time = toLocalTimeInput(stored)
    const back = new Date(`${date}T${time}`).toISOString()
    expect(back).toBe(new Date(stored).toISOString())
  })
})

describe('timeAgo', () => {
  it('returns empty for invalid input', () => {
    expect(timeAgo('')).toBe('')
    expect(timeAgo('not-a-date')).toBe('')
  })

  it('formats recent times', () => {
    const now = Date.now()
    expect(timeAgo(new Date(now - 30_000).toISOString())).toBe('just now')
    expect(timeAgo(new Date(now - 5 * 60_000).toISOString())).toBe('5m ago')
    expect(timeAgo(new Date(now - 2 * 60 * 60_000).toISOString())).toBe('2h ago')
    expect(timeAgo(new Date(now - 3 * 24 * 60 * 60_000).toISOString())).toBe('3d ago')
  })

  it('formats future times', () => {
    const now = Date.now()
    expect(timeAgo(new Date(now + 30_000).toISOString())).toBe('just now')
    expect(timeAgo(new Date(now + 5 * 60_000).toISOString())).toBe('in 5m')
    expect(timeAgo(new Date(now + 90 * 60_000).toISOString())).toBe('in 1h')
    expect(timeAgo(new Date(now + 60 * 60_000).toISOString())).toBe('in 1h')
    expect(timeAgo(new Date(now + 24 * 60 * 60_000).toISOString())).toBe('in 1d')
    expect(timeAgo(new Date(now + 3 * 24 * 60 * 60_000).toISOString())).toBe('in 3d')
  })
})

describe('allDayRange', () => {
  it('spans the local day with a 09:00 reminder', () => {
    expect(allDayRange('2026-09-30')).toEqual({
      start: new Date(2026, 8, 30).toISOString(),
      end: new Date(new Date(2026, 9, 1).getTime() - 1).toISOString(),
      remind: new Date(2026, 8, 30, 9, 0, 0).toISOString(),
    })
  })

  it('rejects malformed and impossible days', () => {
    expect(allDayRange('2026-9-1')).toBeNull()
    expect(allDayRange('2026-02-31')).toBeNull()
  })
})

describe('isAllDayRange', () => {
  it('matches a helper-built day', () => {
    const day = allDayRange('2026-09-30')!
    expect(isAllDayRange(day.start, day.end)).toBe(true)
  })

  it('rejects other schedules and missing inputs', () => {
    const start = new Date(2026, 8, 30, 9, 0).toISOString()
    const end = new Date(2026, 8, 30, 10, 0).toISOString()
    expect(isAllDayRange(start, end)).toBe(false)
    expect(isAllDayRange(undefined, end)).toBe(false)
    expect(isAllDayRange(start, undefined)).toBe(false)
  })
})

describe('presetWindow', () => {
  it('covers the whole local day for today', () => {
    expect(presetWindow('today', now)).toEqual({
      from: new Date(2026, 8, 30).toISOString(),
      to: endOfToday,
    })
  })

  it('covers the previous local day for yesterday', () => {
    expect(presetWindow('yesterday', now)).toEqual({
      from: new Date(2026, 8, 29).toISOString(),
      to: new Date(new Date(2026, 8, 30).getTime() - 1).toISOString(),
    })
  })

  it('starts the week on Sunday', () => {
    // 30 September 2026 is a Wednesday, so its week began Sunday the 27th.
    expect(new Date(2026, 8, 30).getDay()).toBe(3)
    expect(presetWindow('week', now)).toEqual({
      from: new Date(2026, 8, 27).toISOString(),
      to: endOfToday,
    })
  })

  it('starts the month on the 1st and ends today', () => {
    expect(presetWindow('month', now)).toEqual({
      from: new Date(2026, 8, 1).toISOString(),
      to: endOfToday,
    })
  })
})

describe('dayWindow', () => {
  it('spans one local day to its last millisecond', () => {
    expect(dayWindow('2026-09-01')).toEqual({
      from: new Date(2026, 8, 1).toISOString(),
      to: new Date(new Date(2026, 8, 2).getTime() - 1).toISOString(),
    })
  })

  it('rejects malformed and impossible days', () => {
    expect(dayWindow('')).toBeNull()
    expect(dayWindow('2026-9-1')).toBeNull()
    expect(dayWindow('2026-02-31')).toBeNull()
  })
})

describe('dateWindow', () => {
  it('has no window for all dates', () => {
    expect(dateWindow('all', '', '', now)).toBeNull()
  })

  it('resolves a named preset', () => {
    expect(dateWindow('today', '', '', now)).toEqual(presetWindow('today', now))
  })

  it('spans a custom range from the start day to the end day', () => {
    expect(dateWindow('custom', '2026-09-01', '2026-09-03', now)).toEqual({
      from: new Date(2026, 8, 1).toISOString(),
      to: new Date(new Date(2026, 8, 4).getTime() - 1).toISOString(),
    })
  })

  it('has no window while a custom range is incomplete', () => {
    expect(dateWindow('custom', '2026-09-01', '', now)).toBeNull()
  })
})
