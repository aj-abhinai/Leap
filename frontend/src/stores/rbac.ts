import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

export const useRBACStore = defineStore('rbac', () => {
  const permissions = ref<string[]>([])

  async function fetchPermissions() {
    try {
      const res = await apiClient.get('/api/permissions')
      permissions.value = res.data?.map((p: any) => p.name) || []
    } catch {
      permissions.value = []
    }
  }

  function can(permission: string): boolean {
    return permissions.value.includes('*') || permissions.value.includes(permission)
  }

  return { permissions, fetchPermissions, can }
})
