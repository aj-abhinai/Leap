import { defineStore } from 'pinia'
import { apiClient } from '@/composables/useApi'
import { usePagination, totalOf } from '@/composables/usePagination'

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

export const useContactsStore = defineStore('contacts', () => {
  const { items: contacts, total, loading, fetch: fetchPage, setTotal } = usePagination<Contact>()

  async function fetchContacts(page = 1, perPage = 20, search = '') {
    await fetchPage(async (p, pp) => {
      const params = new URLSearchParams()
      params.set('page', String(p))
      params.set('per_page', String(pp))
      if (search) params.set('q', search)
      return apiClient.get(`/api/contacts?${params}`)
    }, page, perPage)
  }

  async function fetchContact(id: string): Promise<Contact> {
    const res = await apiClient.get(`/api/contacts/${id}`)
    return res.data
  }

  async function fetchTotal() {
    try {
      const params = new URLSearchParams()
      params.set('per_page', '1')
      const res = await apiClient.get(`/api/contacts?${params}`)
      setTotal(totalOf(res))
    } catch {}
  }

  return { contacts, total, loading, fetchContacts, fetchContact, fetchTotal }
})
