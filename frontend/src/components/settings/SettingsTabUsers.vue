<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/composables/useApi'
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
import { Plus, Trash2 } from '@lucide/vue'

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
  try {
    await apiClient.post('/api/users', {
      name: newUserName.value,
      email: newUserEmail.value,
      password: newUserPassword.value,
    })
    newUserName.value = ''
    newUserEmail.value = ''
    newUserPassword.value = ''
    loadUsers()
  } catch (e: any) {
    newUserError.value = e.message
  }
}

async function deleteUser(userId: string) {
  try {
    await apiClient.delete(`/api/users/${userId}`)
    loadUsers()
  } catch {}
}
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardHeader>
        <CardTitle>Create User</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex gap-2">
          <Input v-model="newUserName" placeholder="Name" class="flex-1" />
          <Input v-model="newUserEmail" placeholder="Email" type="email" class="flex-1" />
          <Input v-model="newUserPassword" placeholder="Password" type="password" class="flex-1" />
          <Button @click="createUser">
            <Plus class="mr-2 size-4" /> Add
          </Button>
        </div>
        <div v-if="newUserError" class="mt-2 text-sm text-destructive">{{ newUserError }}</div>
      </CardContent>
    </Card>
    <Card>
      <CardHeader>
        <CardTitle>Users</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Roles</TableHead>
              <TableHead class="w-20">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="u in users" :key="u.id">
              <TableCell class="font-medium">{{ u.name }}</TableCell>
              <TableCell>{{ u.email }}</TableCell>
              <TableCell>
                <Badge v-for="r in u.roles" :key="r.id" variant="secondary" class="mr-1">
                  {{ r.name }}
                </Badge>
              </TableCell>
              <TableCell>
                <Button variant="ghost" size="icon" @click="deleteUser(u.id)">
                  <Trash2 class="size-4 text-destructive" />
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  </div>
</template>
