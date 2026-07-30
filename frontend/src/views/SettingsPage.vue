<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/composables/useApi'
import { useActivityStore, type ActivityEntry } from '@/stores/activity'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Checkbox } from '@/components/ui/checkbox'
import { Separator } from '@/components/ui/separator'
import { Plus, Loader2, Trash2 } from '@lucide/vue'
import { Badge } from '@/components/ui/badge'

const activity = useActivityStore()

interface Role {
  id: string
  name: string
  description: string
  permissions?: { id: string; name: string }[]
}

interface Permission {
  id: string
  name: string
  description: string
}

interface User {
  id: string
  name: string
  email: string
  roles?: Role[]
  created_at: string
}

interface Pipeline {
  id: string
  name: string
  description?: string
  stages?: { id: string; name: string; order: number }[]
}

const roles = ref<Role[]>([])
const allPermissions = ref<Permission[]>([])
const users = ref<User[]>([])
const pipelines = ref<Pipeline[]>([])

const newRoleName = ref('')
const newRoleDesc = ref('')
const newRoleError = ref('')

const newUserName = ref('')
const newUserEmail = ref('')
const newUserPassword = ref('')
const newUserError = ref('')

const newPipelineName = ref('')
const newPipelineDesc = ref('')
const newPipelineError = ref('')

const activityPage = ref(1)

onMounted(() => {
  loadRoles()
  loadPermissions()
  loadUsers()
  loadPipelines()
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

async function loadUsers() {
  try {
    const res = await apiClient.get('/api/users')
    users.value = res.data
  } catch {}
}

async function loadPipelines() {
  try {
    const res = await apiClient.get('/api/pipelines')
    pipelines.value = res.data
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

async function createPipeline() {
  newPipelineError.value = ''
  if (!newPipelineName.value) {
    newPipelineError.value = 'Name is required'
    return
  }
  try {
    await apiClient.post('/api/pipelines', { name: newPipelineName.value, description: newPipelineDesc.value })
    newPipelineName.value = ''
    newPipelineDesc.value = ''
    loadPipelines()
  } catch (e: any) {
    newPipelineError.value = e.message
  }
}

async function deletePipeline(pipelineId: string) {
  try {
    await apiClient.delete(`/api/pipelines/${pipelineId}`)
    loadPipelines()
  } catch {}
}

function loadActivity() {
  activity.fetchActivity(activityPage.value, 20)
}

function roleHasPermission(role: any, permId: string): boolean {
  return role.permissions?.some((p: any) => p.id === permId) || false
}
</script>

<template>
  <LayoutShell>
    <div class="flex flex-1 flex-col gap-4 p-4 pt-0">
      <Tabs defaultValue="profile" class="w-full">
        <TabsList>
          <TabsTrigger value="profile">Profile</TabsTrigger>
          <TabsTrigger value="users">Users</TabsTrigger>
          <TabsTrigger value="roles">Roles</TabsTrigger>
          <TabsTrigger value="pipelines">Pipelines</TabsTrigger>
          <TabsTrigger value="activity">Activity Log</TabsTrigger>
        </TabsList>

        <TabsContent value="profile" class="mt-4">
          <Card>
            <CardHeader>
              <CardTitle>Profile Settings</CardTitle>
            </CardHeader>
            <CardContent>
              <p class="text-sm text-muted-foreground">
                Your account settings. User management is available on other tabs.
              </p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="users" class="mt-4 space-y-4">
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
        </TabsContent>

        <TabsContent value="roles" class="mt-4 space-y-4">
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
        </TabsContent>

        <TabsContent value="pipelines" class="mt-4 space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Create Pipeline</CardTitle>
            </CardHeader>
            <CardContent>
              <div class="flex gap-2">
                <Input v-model="newPipelineName" placeholder="Pipeline name" class="flex-1" />
                <Input v-model="newPipelineDesc" placeholder="Description" class="flex-1" />
                <Button @click="createPipeline">
                  <Plus class="mr-2 size-4" /> Add
                </Button>
              </div>
              <div v-if="newPipelineError" class="mt-2 text-sm text-destructive">{{ newPipelineError }}</div>
            </CardContent>
          </Card>
          <Card v-for="p in pipelines" :key="p.id">
            <CardHeader class="flex flex-row items-center justify-between">
              <CardTitle class="text-base">{{ p.name }}</CardTitle>
              <Button variant="ghost" size="sm" @click="deletePipeline(p.id)">Delete</Button>
            </CardHeader>
            <CardContent>
              <div class="text-sm text-muted-foreground mb-2">{{ p.description }}</div>
              <div class="flex flex-wrap gap-1">
                <Badge v-for="s in p.stages" :key="s.id" variant="outline">
                  {{ s.name }}
                </Badge>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="activity" class="mt-4">
          <Card>
            <CardHeader>
              <CardTitle>Activity Log</CardTitle>
            </CardHeader>
            <CardContent>
              <Button variant="outline" size="sm" @click="loadActivity()" class="mb-4">
                Refresh
              </Button>
              <div v-if="activity.entries.length === 0" class="text-sm text-muted-foreground">
                No activity logged yet
              </div>
              <Table v-else>
                <TableHeader>
                  <TableRow>
                    <TableHead>Description</TableHead>
                    <TableHead>Action</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Date</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-for="e in activity.entries" :key="e.id">
                    <TableCell>{{ e.description }}</TableCell>
                    <TableCell>{{ e.action }}</TableCell>
                    <TableCell>{{ e.resource_type }}</TableCell>
                    <TableCell class="text-xs">
                      {{ new Date(e.created_at).toLocaleString() }}
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  </LayoutShell>
</template>
