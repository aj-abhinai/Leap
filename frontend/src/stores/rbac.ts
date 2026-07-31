import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'

interface Permission {
  id: string
  name: string
  description: string
  created_at: string
}

export const useRBACStore = defineStore('rbac', () => {
  const permissions = ref<string[]>([])

  async function fetchPermissions() {
    try {
      const res = await apiClient.get<Permission[]>('/api/permissions')
      permissions.value = res.data.map((p) => p.name)
    } catch {
      permissions.value = []
    }
  }

  function can(permission: string): boolean {
    return permissions.value.includes('*') || permissions.value.includes(permission)
  }

  return { permissions, fetchPermissions, can }
})
