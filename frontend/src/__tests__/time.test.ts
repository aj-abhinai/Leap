import { describe, it, expect } from 'vitest'
import { toLocalDateInput, toLocalTimeInput, timeAgo } from '@/utils/time'

// These helpers render a stored instant into local wall-clock components for
// <input type="date"> / <input type="time">, matching the local-time semantics
// of mergeDateTime (local wall-clock in, UTC instant out). Tests compare
// against the environment's own local getters so they are zone-agnostic.
describe('toLocalDateInput / toLocalTimeInput', () => {
  it('renders local wall-clock components', () => {
    const instant = '2026-08-06T09:30:00Z'
    const d = new Date(instant)
    const expectDate = [
      d.getFullYear(),
      String(d.getMonth() + 1).padStart(2, '0'),
      String(d.getDate()).padStart(2, '0'),
    ].join('-')
    const expectTime = [
      String(d.getHours()).padStart(2, '0'),
      String(d.getMinutes()).padStart(2, '0'),
    ].join(':')
    expect(toLocalDateInput(instant)).toBe(expectDate)
    expect(toLocalTimeInput(instant)).toBe(expectTime)
  })

  it('round-trips through mergeDateTime semantics (local -> UTC instant)', () => {
    const stored = '2026-08-06T09:30:00Z'
    const date = toLocalDateInput(stored)
    const time = toLocalTimeInput(stored)
    const back = new Date(`${date}T${time}`).toISOString()
    // new Date(local wall-clock) -> same instant in the environment's zone.
    expect(back).toBe(new Date(stored).toISOString())
  })

  it('pads single-digit components', () => {
    const instant = '2026-01-05T06:07:00Z'
    const d = new Date(instant)
    expect(toLocalDateInput(instant)).toBe(
      `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`,
    )
    expect(toLocalTimeInput(instant)).toBe(
      `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`,
    )
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
})
