import { describe, it, expect } from 'vitest'
import { due, dueLabel } from '@/utils/activity'

// A row with only a creation time has no due boundary: the table must not
// present the record as a due date.
describe('due', () => {
  it('prefers the end, then the start, then the reminder', () => {
    expect(
      due({ scheduled_end_at: 'end', scheduled_at: 'start', remind_at: 'remind', created_at: 'created' }),
    ).toBe('end')
    expect(due({ scheduled_at: 'start', remind_at: 'remind', created_at: 'created' })).toBe('start')
    expect(due({ remind_at: 'remind', created_at: 'created' })).toBe('remind')
  })

  it('has no due time when the row carries only a creation time', () => {
    expect(due({ created_at: '2026-10-01T00:00:00Z' })).toBe('')
  })
})

describe('dueLabel', () => {
  it('is empty when there is no due boundary', () => {
    expect(dueLabel({ created_at: '2026-10-01T00:00:00Z' })).toBe('')
  })

  it('shows an all-day task as its date', () => {
    const start = new Date(2026, 8, 30).toISOString()
    const end = new Date(new Date(2026, 9, 1).getTime() - 1).toISOString()
    expect(dueLabel({ scheduled_at: start, scheduled_end_at: end })).toBe(
      `${new Date(start).toLocaleDateString()} (all day)`,
    )
  })
})
