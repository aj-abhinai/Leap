import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

export interface UserOption {
  id: string
  name: string
}

// users store powers the lead assignee picker with the minimal id+name list
// from GET /api/users/options (lead:read). Fetched lazily once and cached for
// the session.
export const useUsersStore = defineStore('users', () => {
  const options = ref<UserOption[]>([])
  const loading = ref(false)
  // error reports whether the last fetch failed. The form uses it to tell a
  // transient failure (keep the current assignee) from a successful fetch that
  // no longer lists the assigned user (a deleted user — reset to Unassigned).
  const error = ref(false)

  async function fetchOptions(force = false) {
    if (options.value.length > 0 && !force) return
    loading.value = true
    error.value = false
    try {
      const res = await apiClient.get<UserOption[]>('/api/users/options')
      options.value = res.data ?? []
    } catch {
      // The picker degrades to "Unassigned" only; a failed options fetch must
      // not become an unhandled rejection in onMounted callers.
      error.value = true
      options.value = []
    } finally {
      loading.value = false
    }
  }

  return { options, loading, error, fetchOptions }
})
