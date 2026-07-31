<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/composables/useApi'
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
import { Plus, Trash2, User } from '@lucide/vue'

interface Role {
  id: string
  name: string
}

interface User {
  id: string
  name: string
  email: string
  roles?: Role[]
  created_at: string
}

const users = ref<User[]>([])
const newUserName = ref('')
const newUserEmail = ref('')
const newUserPassword = ref('')
const newUserError = ref('')
const creatingUser = ref(false)

onMounted(() => loadUsers())

async function loadUsers() {
  try {
    const res = await apiClient.get('/api/users')
    users.value = res.data
  } catch {}
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
              <TableHead>Roles</TableHead>
              <TableHead class="w-16">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="u in users" :key="u.id">
              <TableCell class="font-medium">{{ u.name }}</TableCell>
              <TableCell class="text-muted-foreground">{{ u.email }}</TableCell>
              <TableCell>
                <div class="flex flex-wrap gap-1">
                  <Badge v-for="r in u.roles" :key="r.id" variant="secondary" class="text-xs">
                    {{ r.name }}
                  </Badge>
                  <span v-if="!u.roles?.length" class="text-xs text-muted-foreground">—</span>
                </div>
              </TableCell>
              <TableCell>
                <Button variant="ghost" size="icon-sm" @click="deleteUser(u.id)">
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
