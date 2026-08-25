import { describe, it, expect } from 'vitest'
import { toLocalDateInput, toLocalTimeInput, timeAgo } from '@/utils/time'

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
})
