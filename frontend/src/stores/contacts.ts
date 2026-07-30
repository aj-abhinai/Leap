import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

export interface Contact {
  id: string
  name: string
  email?: string
  phone?: string
  location?: string
  age?: number
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

  return { contacts, total, loading, fetchContacts }
})
