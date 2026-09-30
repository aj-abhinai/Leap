import { describe, it, expect } from 'vitest'
import { contactSchema } from '@/lib/validation'

// The contact schema gates the form's first values; the per-row phone check
// in ContactForm covers the remaining repeater rows.
describe('contactSchema phone', () => {
  const base = { name: 'Alice', email: 'alice@example.com' }

  it('rejects a phone with no digits', () => {
    expect(contactSchema.safeParse({ ...base, phone: 'abc' }).success).toBe(false)
  })

  it('accepts a formatted phone with digits', () => {
    expect(contactSchema.safeParse({ ...base, phone: '98765 43210' }).success).toBe(true)
  })

  it('accepts an absent phone', () => {
    expect(contactSchema.safeParse({ ...base }).success).toBe(true)
  })
})
