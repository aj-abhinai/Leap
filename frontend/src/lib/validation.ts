import { z } from 'zod'

export const profileSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  phone: z.string().optional(),
})

export const contactSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  nickname: z.string().optional(),
  email: z.string().email('Invalid email').optional().or(z.literal('')),
  phone: z.string().optional(),
  location: z.string().optional(),
  age: z.number().int().positive().optional(),
}).refine((d) => d.phone || d.email, {
  message: 'A phone or email is required',
  path: ['phone'],
})
