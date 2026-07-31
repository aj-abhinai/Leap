<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/composables/useApi'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Plus } from '@lucide/vue'

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
  try {
    await apiClient.post('/api/roles', { name: newRoleName.value, description: newRoleDesc.value })
    newRoleName.value = ''
    newRoleDesc.value = ''
    loadRoles()
  } catch (e: any) {
    newRoleError.value = e.message
  }
}

async function togglePermission(roleId: string, permissionId: string, hasIt: boolean) {
  try {
    if (hasIt) {
      await apiClient.delete(`/api/roles/${roleId}/permissions/${permissionId}`)
    } else {
      await apiClient.post(`/api/roles/${roleId}/permissions`, { permission_id: permissionId })
    }
    loadRoles()
  } catch {}
}

async function deleteRole(roleId: string) {
  try {
    await apiClient.delete(`/api/roles/${roleId}`)
    loadRoles()
  } catch {}
}

function roleHasPermission(role: any, permId: string): boolean {
  return role.permissions?.some((p: any) => p.id === permId) || false
}
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardHeader>
        <CardTitle>Create Role</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex gap-2">
          <Input v-model="newRoleName" placeholder="Role name" class="flex-1" />
          <Input v-model="newRoleDesc" placeholder="Description" class="flex-1" />
          <Button @click="createRole">
            <Plus class="mr-2 size-4" /> Add
          </Button>
        </div>
        <div v-if="newRoleError" class="mt-2 text-sm text-destructive">{{ newRoleError }}</div>
      </CardContent>
    </Card>
    <Card v-for="role in roles" :key="role.id">
      <CardHeader class="flex flex-row items-center justify-between">
        <CardTitle class="text-base">{{ role.name }}</CardTitle>
        <Button variant="ghost" size="sm" @click="deleteRole(role.id)">Delete</Button>
      </CardHeader>
      <CardContent>
        <div class="text-sm text-muted-foreground mb-2">{{ role.description }}</div>
        <div class="grid grid-cols-2 gap-2">
          <div
            v-for="perm in allPermissions"
            :key="perm.id"
            class="flex items-center gap-2"
          >
            <Checkbox
              :checked="roleHasPermission(role, perm.id)"
              @update:checked="togglePermission(role.id, perm.id, roleHasPermission(role, perm.id))"
            />
            <Label class="text-sm">{{ perm.name }}</Label>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
