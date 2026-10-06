import { describe, it, expect } from 'vitest'
import { contactSchema, profileSchema } from '@/lib/validation'

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

// The profile schema matches the contact rule: only a value with no digits
// is refused; an explicit empty string clears the phone.
describe('profileSchema phone', () => {
  it('rejects a phone with no digits', () => {
    expect(profileSchema.safeParse({ name: 'Alice', phone: 'abc' }).success).toBe(false)
  })

  it('accepts an empty string and an absent phone', () => {
    expect(profileSchema.safeParse({ name: 'Alice', phone: '' }).success).toBe(true)
    expect(profileSchema.safeParse({ name: 'Alice' }).success).toBe(true)
  })

  it('accepts a formatted phone with digits', () => {
    expect(profileSchema.safeParse({ name: 'Alice', phone: '98765 43210' }).success).toBe(true)
  })
})
