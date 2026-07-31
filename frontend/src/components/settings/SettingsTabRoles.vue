<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Plus, Trash2, Shield } from '@lucide/vue'

interface Permission {
  id: string
  name: string
  description: string
}

interface Role {
  id: string
  name: string
  description: string
  permissions?: { id: string; name: string }[]
}

const roles = ref<Role[]>([])
const allPermissions = ref<Permission[]>([])
const newRoleName = ref('')
const newRoleDesc = ref('')
const newRoleError = ref('')
const creatingRole = ref(false)

onMounted(() => {
  loadRoles()
  loadPermissions()
})

async function loadRoles() {
  try {
    const res = await apiClient.get('/api/roles')
    roles.value = res.data
  } catch {}
}

async function loadPermissions() {
  try {
    const res = await apiClient.get('/api/permissions')
    allPermissions.value = res.data
  } catch {}
}

async function createRole() {
  newRoleError.value = ''
  if (!newRoleName.value) {
    newRoleError.value = 'Name is required'
    return
  }
  creatingRole.value = true
  try {
    await apiClient.post('/api/roles', { name: newRoleName.value, description: newRoleDesc.value })
    toast.success('Role created')
    newRoleName.value = ''
    newRoleDesc.value = ''
    loadRoles()
  } catch (e: any) {
    newRoleError.value = e.message || 'Failed to create role'
  } finally {
    creatingRole.value = false
  }
}

async function togglePermission(roleId: string, permissionId: string, hasIt: boolean) {
  try {
    if (hasIt) {
      await apiClient.delete(`/api/roles/${roleId}/permissions/${permissionId}`)
    } else {
      await apiClient.post(`/api/roles/${roleId}/permissions`, { permission_id: permissionId })
    }
    toast.success('Permissions updated')
    loadRoles()
  } catch (e: any) {
    toast.error(e.message || 'Failed to update permissions')
  }
}

async function deleteRole(roleId: string) {
  try {
    await apiClient.delete(`/api/roles/${roleId}`)
    toast.success('Role deleted')
    loadRoles()
  } catch (e: any) {
    toast.error(e.message || 'Failed to delete role')
  }
}

function roleHasPermission(role: any, permId: string): boolean {
  return role.permissions?.some((p: any) => p.id === permId) || false
}
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardHeader>
        <CardTitle class="text-base">Create Role</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex flex-wrap gap-2">
          <Input v-model="newRoleName" placeholder="Role name" class="min-w-40 flex-1" />
          <Input v-model="newRoleDesc" placeholder="Description" class="min-w-40 flex-1" />
          <Button @click="createRole" :disabled="creatingRole">
            <Plus class="mr-2 size-4" /> Add Role
          </Button>
        </div>
        <div v-if="newRoleError" class="mt-2 text-sm text-destructive">{{ newRoleError }}</div>
      </CardContent>
    </Card>
    <div v-if="roles.length === 0" class="flex flex-col items-center justify-center py-12 text-center">
      <Shield class="size-10 text-muted-foreground/40 mb-3" />
      <p class="text-sm text-muted-foreground">No roles defined</p>
    </div>
    <Card v-for="role in roles" :key="role.id">
      <CardHeader class="flex flex-row items-center justify-between pb-2">
        <div>
          <CardTitle class="text-base">{{ role.name }}</CardTitle>
          <p v-if="role.description" class="text-sm text-muted-foreground mt-0.5">{{ role.description }}</p>
        </div>
        <Button variant="ghost" size="sm" @click="deleteRole(role.id)">
          <Trash2 class="mr-1 size-3.5" /> Delete
        </Button>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-2 md:grid-cols-3 gap-2">
          <div
            v-for="perm in allPermissions"
            :key="perm.id"
            class="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50 transition-colors"
          >
            <Checkbox
              :checked="roleHasPermission(role, perm.id)"
              @update:checked="togglePermission(role.id, perm.id, roleHasPermission(role, perm.id))"
            />
            <Label class="text-sm cursor-pointer">{{ perm.name }}</Label>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
