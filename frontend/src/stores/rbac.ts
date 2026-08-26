import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getMyPermissions } from '@/api/auth'

export const useRBACStore = defineStore('rbac', () => {
  const permissions = ref<string[]>([])

  async function fetchPermissions() {
    try {
      const res = await getMyPermissions()
      permissions.value = res.data
    } catch {
      permissions.value = []
    }
  }

  function clear() {
    permissions.value = []
  }

  function can(permission: string): boolean {
    return permissions.value.includes('*') || permissions.value.includes(permission)
  }

  return { permissions, fetchPermissions, clear, can }
})