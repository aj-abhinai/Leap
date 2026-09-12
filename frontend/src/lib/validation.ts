import { z } from 'zod'

export const PASSWORD_POLICY_HINT =
  'Password must be 10-72 characters and include an uppercase letter, a lowercase letter, a digit, and a special character'

// Mirrors the server-side policy in internal/auth/password.go. Length is
// measured in bytes (like Go) so multibyte characters are not undercounted.
export function isStrongPassword(password: string): boolean {
  const bytes = new TextEncoder().encode(password).length
  if (bytes < 10 || bytes > 72) return false
  return (
    /[A-Z]/.test(password) &&
    /[a-z]/.test(password) &&
    /\d/.test(password) &&
    /[^A-Za-z0-9]/.test(password)
  )
}

export const profileSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  phone: z.string().optional(),
})

// One address value, reused to validate every row of the multi-valued email
// repeater (the contact schema only sees the primary).
export const emailSchema = z.email('Invalid email')

// Date of birth is date-only (YYYY-MM-DD): a real calendar date, never in the
// future. An empty string clears it on update; the approximate integer age
// stays as the fallback.
function localToday(): string {
  const d = new Date()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

const dateOfBirthSchema = z
  .string()
  .regex(/^\d{4}-\d{2}-\d{2}$/, 'Use YYYY-MM-DD')
  .refine((v) => {
    const [y, m, d] = v.split('-').map(Number)
    const dt = new Date(y, m - 1, d)
    return dt.getFullYear() === y && dt.getMonth() === m - 1 && dt.getDate() === d
  }, 'Enter a real date')
  .refine((v) => v <= localToday(), 'Date of birth cannot be in the future')

export const contactSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  nickname: z.string().optional(),
  email: z.email('Invalid email').optional().or(z.literal('')),
  phone: z.string().optional(),
  location: z.string().optional(),
  age: z.number().int().positive().optional(),
  date_of_birth: dateOfBirthSchema.optional().or(z.literal('')),
}).refine((d) => d.phone || d.email, {
  message: 'A phone or email is required',
  path: ['phone'],
})
