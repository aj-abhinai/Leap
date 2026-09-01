import { apiClient, type ApiResponse } from '@/composables/useApi'

// Contact entities and the contact endpoints. Every URL for the contacts and
// notes resources lives here so callers never construct them by hand.

export interface TagRef {
  id: string
  name: string
  color?: string
}

export interface PhoneValue {
  id: string
  value: string
  is_primary: boolean
}

export interface EmailValue {
  id: string
  value: string
  is_primary: boolean
}

export interface Contact {
  id: string
  name: string
  nickname?: string
  email?: string
  phone?: string
  phones?: PhoneValue[]
  emails?: EmailValue[]
  location?: string
  age?: number
  tags?: TagRef[]
  status?: TagRef
  created_at: string
  updated_at: string
}

export interface ContactNote {
  id: string
  contact_id: string
  user_id?: string
  user_name?: string
  note: string
  created_at: string
  updated_at: string
}

export type ContactSaveBody = Record<string, any>

// ResolveMatch is the compact contact result returned by the resolve
// endpoint (lead entry phone lookup): id, name, and primary phone/email.
export interface ResolveMatch {
  id: string
  name: string
  phone?: string
  email?: string
}

// DuplicateMatch is a live contact returned in a 409 when a create collides
// with an existing primary phone or email and the duplicate is unconfirmed.
export interface DuplicateMatch {
  id: string
  name: string
  phone?: string
  email?: string
}

export interface BulkImportResult {
  imported: number
  failed: number
  errors?: { row: number; message: string }[]
}

export function listContacts(params: {
  page: number
  perPage: number
  q?: string
}): Promise<ApiResponse<Contact[]>> {
  const p = new URLSearchParams({ page: String(params.page), per_page: String(params.perPage) })
  if (params.q) p.set('q', params.q)
  return apiClient.get(`/api/contacts?${p}`)
}

export function getContact(id: string): Promise<ApiResponse<Contact>> {
  return apiClient.get(`/api/contacts/${id}`)
}

export function createContact(body: ContactSaveBody): Promise<ApiResponse<Contact & { warnings?: string[] }>> {
  return apiClient.post('/api/contacts', body)
}

export function updateContact(id: string, body: ContactSaveBody): Promise<ApiResponse<Contact>> {
  return apiClient.patch(`/api/contacts/${id}`, body)
}

export function deleteContact(id: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/contacts/${id}`)
}

export function bulkImportContacts(contacts: unknown[]): Promise<ApiResponse<BulkImportResult>> {
  return apiClient.post('/api/contacts/bulk', { contacts })
}

export function resolveContactByPhone(phone: string): Promise<ApiResponse<ResolveMatch[]>> {
  return apiClient.get(`/api/contacts/resolve?phone=${encodeURIComponent(phone)}`)
}

export function listNotes(contactId: string, params?: { page?: number; perPage?: number }): Promise<ApiResponse<ContactNote[]>> {
  const p = new URLSearchParams()
  if (params?.page) p.set('page', String(params.page))
  if (params?.perPage) p.set('per_page', String(params.perPage))
  const qs = p.toString()
  return apiClient.get(`/api/contacts/${contactId}/notes${qs ? `?${qs}` : ''}`)
}

export function addNote(contactId: string, note: string): Promise<ApiResponse<ContactNote>> {
  return apiClient.post(`/api/contacts/${contactId}/notes`, { note })
}

export function deleteNote(contactId: string, noteId: string): Promise<ApiResponse<null>> {
  return apiClient.delete(`/api/contacts/${contactId}/notes/${noteId}`)
}