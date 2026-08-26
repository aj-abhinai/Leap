import { defineStore } from 'pinia'
import { usePagination, totalOf } from '@/composables/usePagination'
import * as api from '@/api/contacts'

export type { Contact, TagRef, PhoneValue, EmailValue, ContactNote, ContactSaveBody } from '@/api/contacts'

export const useContactsStore = defineStore('contacts', () => {
  const { items: contacts, total, loading, fetch: fetchPage, setTotal } = usePagination<api.Contact>()

  async function fetchContacts(page = 1, perPage = 20, search = '') {
    await fetchPage((p, pp) => api.listContacts({ page: p, perPage: pp, q: search || undefined }), page, perPage)
  }

  async function fetchContact(id: string): Promise<api.Contact> {
    const res = await api.getContact(id)
    return res.data
  }

  async function fetchTotal() {
    try {
      const res = await api.listContacts({ page: 1, perPage: 1 })
      setTotal(totalOf(res))
    } catch {}
  }

  // Mutations return the saved contact so callers can read create warnings;
  // list refetches are the caller's job.
  async function create(body: api.ContactSaveBody) {
    const res = await api.createContact(body)
    return res.data
  }

  async function update(id: string, body: api.ContactSaveBody) {
    const res = await api.updateContact(id, body)
    return res.data
  }

  async function remove(id: string) {
    await api.deleteContact(id)
  }

  return { contacts, total, loading, fetchContacts, fetchContact, fetchTotal, create, update, remove }
})