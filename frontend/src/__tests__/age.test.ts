import { describe, it, expect } from 'vitest'
import { computeAge, displayAge } from '@/utils/age'

// buildDob returns the YYYY-MM-DD date that is exactly `years` ago today (or
// `adjustDays` away from that birthday), keeping the assertions independent of
// the calendar day the test runs on. The day is clamped to the target month's
// length so a Feb 29 run lands on Feb 28 of a non-leap year instead of rolling
// into March.
function buildDob(years: number, adjustDays = 0): string {
  const now = new Date()
  const year = now.getFullYear() - years
  const month = now.getMonth()
  const daysInMonth = new Date(year, month + 1, 0).getDate()
  const d = new Date(year, month, Math.min(now.getDate(), daysInMonth))
  d.setDate(d.getDate() + adjustDays)
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

describe('computeAge', () => {
  it('returns null for missing or malformed values', () => {
    expect(computeAge(undefined)).toBeNull()
    expect(computeAge(null)).toBeNull()
    expect(computeAge('')).toBeNull()
    expect(computeAge('not-a-date')).toBeNull()
    expect(computeAge('2000-13-01')).toBeNull()
  })

  it('returns null for dates that do not exist', () => {
    expect(computeAge('2023-02-31')).toBeNull()
    expect(computeAge('2023-04-31')).toBeNull()
  })

  it('computes the completed age from the birth date', () => {
    expect(computeAge(buildDob(30))).toBe(30)
    expect(computeAge(buildDob(30, 1))).toBe(29)
  })
})

describe('displayAge', () => {
  it('prefers the computed age when the birth date is usable', () => {
    expect(displayAge(buildDob(30), 99)).toBe(30)
  })

  it('falls back to the approximate age when no usable birth date exists', () => {
    expect(displayAge(undefined, 42)).toBe(42)
    expect(displayAge('2023-02-31', 42)).toBe(42)
  })

  it('returns null when neither value is usable', () => {
    expect(displayAge(undefined, undefined)).toBeNull()
  })
})
