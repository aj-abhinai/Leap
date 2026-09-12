import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getMyPermissions } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

export const useRBACStore = defineStore('rbac', () => {
  const permissions = ref<string[]>([])
  // loaded marks a completed fetch for the current session; clear() resets it
  // on logout so the next login fetches again.
  const loaded = ref(false)
  // inflight coalesces concurrent mounts into one request, and generation
  // discards a response that lands after the session was cleared.
  let inflight: Promise<void> | null = null
  let generation = 0

  // fetchPermissions loads the signed-in user's permissions once. It is a
  // no-op while logged out (boot runs before any session exists) and shares
  // one request across concurrent callers.
  async function fetchPermissions() {
    const auth = useAuthStore()
    if (!auth.isAuthenticated || loaded.value) return
    if (inflight) return inflight
    const requested = generation
    const request = (async () => {
      try {
        const res = await getMyPermissions()
        // A response from a previous session must not repopulate state.
        if (requested !== generation) return
        permissions.value = res.data
        loaded.value = true
      } catch {
        if (requested === generation) permissions.value = []
      }
    })()
    inflight = request
    // Detach the in-flight pointer once this request settles; a clear() may
    // already have replaced it.
    void request.then(() => {
      if (inflight === request) inflight = null
    })
    return request
  }

  function clear() {
    generation++
    inflight = null
    permissions.value = []
    loaded.value = false
  }

  function can(permission: string): boolean {
    return permissions.value.includes('*') || permissions.value.includes(permission)
  }

  return { permissions, fetchPermissions, clear, can }
})
