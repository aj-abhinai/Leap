import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

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
  const contacts = ref<Contact[]>([])
  const total = ref(0)
  const loading = ref(false)

  async function fetchContacts(page = 1, perPage = 20, search = '') {
    loading.value = true
    try {
      const params = new URLSearchParams()
      params.set('page', String(page))
      params.set('per_page', String(perPage))
      if (search) params.set('q', search)
      const res = await apiClient.get(`/api/contacts?${params}`)
      contacts.value = res.data
      total.value = res.meta?.total || 0
    } finally {
      loading.value = false
    }
  }

  async function fetchContact(id: string): Promise<Contact | null> {
    try {
      const res = await apiClient.get(`/api/contacts/${id}`)
      return res.data
    } catch {
      return null
    }
  }

  async function fetchTotal() {
    try {
      const params = new URLSearchParams()
      params.set('per_page', '1')
      const res = await apiClient.get(`/api/contacts?${params}`)
      total.value = res.meta?.total || 0
    } catch {}
  }

  return { contacts, total, loading, fetchContacts, fetchContact, fetchTotal }
})
