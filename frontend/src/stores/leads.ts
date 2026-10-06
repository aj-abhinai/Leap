import { ref } from 'vue'
import { defineStore } from 'pinia'
import { totalOf } from '@/composables/usePagination'
import * as api from '@/api/leads'

export type { Lead, LeadListQuery } from '@/api/leads'

// useLeadsStore owns only the total lead count for the dashboard; the kanban
// reads the board store and list screens own their own fetch state.
export const useLeadsStore = defineStore('leads', () => {
  const total = ref(0)

  // fetchTotal reads the count for the dashboard card. A failure leaves the
  // previous value in place.
  async function fetchTotal() {
    try {
      const res = await api.listLeads({ page: 1, perPage: 1 })
      total.value = totalOf(res)
    } catch {}
  }

  return { total, fetchTotal }
})
