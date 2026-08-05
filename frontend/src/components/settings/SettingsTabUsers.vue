<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { apiClient } from '@/composables/useApi'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Plus, ShieldCheck, Trash2, User } from '@lucide/vue'

interface Role {
  id: string
  name: string
}

interface User {
  id: string
  name: string
  email: string
  role?: Role | null
  protected?: boolean
  created_at: string
}

const users = shallowRef<User[]>([])
const roles = shallowRef<Role[]>([])
const auth = useAuthStore()
const newUserName = shallowRef('')
const newUserEmail = shallowRef('')
const newUserPassword = shallowRef('')
const newUserError = shallowRef('')
const creatingUser = shallowRef(false)

onMounted(() => {
  loadUsers()
  loadRoles()
})

async function loadUsers() {
  try {
    const res = await apiClient.get('/api/users')
    users.value = res.data
  } catch (e: any) {
    toast.error(e.message || 'Failed to load users')
  }
}

async function loadRoles() {
  try {
    const res = await apiClient.get('/api/roles')
    roles.value = res.data
  } catch (e: any) {
    toast.error(e.message || 'Failed to load roles')
  }
}

async function createUser() {
  newUserError.value = ''
  if (!newUserName.value || !newUserEmail.value || !newUserPassword.value) {
    newUserError.value = 'All fields are required'
    return
  }
  creatingUser.value = true
  try {
    await apiClient.post('/api/users', {
      name: newUserName.value,
      email: newUserEmail.value,
      password: newUserPassword.value,
    })
    toast.success('User created')
    newUserName.value = ''
    newUserEmail.value = ''
    newUserPassword.value = ''
    loadUsers()
  } catch (e: any) {
    newUserError.value = e.message || 'Failed to create user'
  } finally {
    creatingUser.value = false
  }
}

async function deleteUser(userId: string) {
  try {
    await apiClient.delete(`/api/users/${userId}`)
    toast.success('User deleted')
    loadUsers()
  } catch (e: any) {
    toast.error(e.message || 'Failed to delete user')
  }
}

async function setRole(userId: string, roleId: string) {
  try {
    await apiClient.put(`/api/users/${userId}/role`, { role_id: roleId })
    toast.success('Role updated')
    loadUsers()
  } catch (e: any) {
    toast.error(e.message || 'Failed to update role')
  }
}

function isProtectedUser(u: User): boolean {
  return !!u.protected || auth.user?.id === u.id
}
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardHeader>
        <CardTitle class="text-base">Create User</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex flex-wrap gap-2">
          <Input v-model="newUserName" placeholder="Name" class="min-w-32 flex-1" />
          <Input v-model="newUserEmail" placeholder="Email" type="email" class="min-w-32 flex-1" />
          <Input v-model="newUserPassword" placeholder="Password" type="password" class="min-w-32 flex-1" />
          <Button @click="createUser" :disabled="creatingUser">
            <Plus class="mr-2 size-4" /> Add User
          </Button>
        </div>
        <div v-if="newUserError" class="mt-2 text-sm text-destructive">{{ newUserError }}</div>
      </CardContent>
    </Card>
    <Card>
      <CardHeader>
        <CardTitle class="text-base">Users ({{ users.length }})</CardTitle>
      </CardHeader>
      <CardContent>
        <div v-if="users.length === 0" class="flex flex-col items-center justify-center py-10 text-center">
          <User class="size-10 text-muted-foreground/40 mb-3" />
          <p class="text-sm text-muted-foreground">No users found</p>
        </div>
        <Table v-else>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Role</TableHead>
              <TableHead class="w-16">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="u in users" :key="u.id">
              <TableCell class="font-medium">{{ u.name }}</TableCell>
              <TableCell class="text-muted-foreground">{{ u.email }}</TableCell>
              <TableCell>
                <div class="flex items-center gap-2">
                  <Badge v-if="u.role" variant="secondary" class="text-xs">
                    {{ u.role.name }}
                    <ShieldCheck v-if="u.role.name === 'superadmin'" class="ml-1 size-3" />
                  </Badge>
                  <span v-else class="text-xs text-muted-foreground">–</span>
                </div>
              </TableCell>
              <TableCell>
                <div class="flex items-center gap-1.5">
                  <select
                    class="h-8 w-40 rounded-md border bg-background px-2 text-sm"
                    :value="u.role?.id ?? ''"
                    :disabled="isProtectedUser(u)"
                    @change="setRole(u.id, ($event.target as HTMLSelectElement).value)"
                  >
                    <option value="">No role</option>
                    <option v-for="r in roles" :key="r.id" :value="r.id">{{ r.name }}</option>
                  </select>
                  <ShieldCheck
                    v-if="u.protected"
                    class="size-3.5 text-muted-foreground"
                    title="Protected"
                  />
                </div>
              </TableCell>
              <TableCell>
                <div v-if="isProtectedUser(u)" class="flex items-center gap-1.5">
                  <ShieldCheck class="size-3.5 text-muted-foreground" />
                  <span class="text-xs text-muted-foreground">Protected</span>
                </div>
                <Button v-else variant="ghost" size="icon-sm" :aria-label="`Delete ${u.name}`" @click="deleteUser(u.id)">
                  <Trash2 class="size-3.5" />
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  </div>
</template>
