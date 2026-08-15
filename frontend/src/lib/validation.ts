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

export const contactSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  nickname: z.string().optional(),
  email: z.email('Invalid email').optional().or(z.literal('')),
  phone: z.string().optional(),
  location: z.string().optional(),
  age: z.number().int().positive().optional(),
}).refine((d) => d.phone || d.email, {
  message: 'A phone or email is required',
  path: ['phone'],
})
