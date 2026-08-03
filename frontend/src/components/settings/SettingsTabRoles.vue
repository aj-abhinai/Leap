<script setup lang="ts">
import { onMounted, shallowRef, computed } from 'vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Checkbox } from '@/components/ui/checkbox'
import { Plus, Shield, ShieldCheck, Trash2, Pencil } from '@lucide/vue'

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

const roles = shallowRef<Role[]>([])
const allPermissions = shallowRef<Permission[]>([])
const loading = shallowRef(false)

// Modal state
const modalOpen = shallowRef(false)
const editingRole = shallowRef<Role | null>(null)
const formName = shallowRef('')
const formDesc = shallowRef('')
const formPermissions = shallowRef<Set<string>>(new Set())
const formError = shallowRef('')
const saving = shallowRef(false)

const isEditing = computed(() => !!editingRole.value)

onMounted(() => {
  loadRoles()
  loadPermissions()
})

async function loadRoles() {
  loading.value = true
  try {
    const res = await apiClient.get('/api/roles')
    roles.value = res.data
  } catch (e: any) {
    toast.error(e.message || 'Failed to load roles')
  } finally {
    loading.value = false
  }
}

async function loadPermissions() {
  try {
    const res = await apiClient.get('/api/permissions')
    allPermissions.value = res.data
  } catch (e: any) {
    toast.error(e.message || 'Failed to load permissions')
  }
}

function openCreate() {
  editingRole.value = null
  formName.value = ''
  formDesc.value = ''
  formPermissions.value = new Set()
  formError.value = ''
  modalOpen.value = true
}

function openEdit(role: Role) {
  editingRole.value = role
  formName.value = role.name
  formDesc.value = role.description
  formPermissions.value = new Set((role.permissions ?? []).map((p) => p.id))
  formError.value = ''
  modalOpen.value = true
}

function isSuperadminRole(role: Role): boolean {
  return role.name === 'superadmin'
}

function permissionIsLocked(role: Role, perm: Permission): boolean {
  return isSuperadminRole(role) && perm.name === '*'
}

function togglePermission(permId: string) {
  const next = new Set(formPermissions.value)
  if (next.has(permId)) {
    next.delete(permId)
  } else {
    next.add(permId)
  }
  formPermissions.value = next
}

async function saveRole() {
  formError.value = ''
  if (!formName.value.trim()) {
    formError.value = 'Name is required'
    return
  }
  saving.value = true
  try {
    if (isEditing.value && editingRole.value) {
      const id = editingRole.value.id
      await apiClient.patch(`/api/roles/${id}`, {
        name: formName.value.trim(),
        description: formDesc.value.trim(),
      })
      // Sync the permission diff
      const current = new Set((editingRole.value.permissions ?? []).map((p) => p.id))
      for (const permId of current) {
        if (!formPermissions.value.has(permId)) {
          await apiClient.delete(`/api/roles/${id}/permissions/${permId}`)
        }
      }
      for (const permId of formPermissions.value) {
        if (!current.has(permId)) {
          await apiClient.post(`/api/roles/${id}/permissions`, { permission_id: permId })
        }
      }
      toast.success('Role updated')
    } else {
      const res = await apiClient.post('/api/roles', {
        name: formName.value.trim(),
        description: formDesc.value.trim(),
      })
      const role = res.data as Role
      for (const permId of formPermissions.value) {
        await apiClient.post(`/api/roles/${role.id}/permissions`, { permission_id: permId })
      }
      toast.success('Role created')
    }
    modalOpen.value = false
    loadRoles()
  } catch (e: any) {
    formError.value = e.message || 'Failed to save role'
  } finally {
    saving.value = false
  }
}

async function deleteRole(role: Role) {
  if (!window.confirm(`Delete role "${role.name}"?`)) return
  try {
    await apiClient.delete(`/api/roles/${role.id}`)
    toast.success('Role deleted')
    loadRoles()
  } catch (e: any) {
    toast.error(e.message || 'Failed to delete role')
  }
}

function permissionLabel(perm: Permission): string {
  return perm.name === '*' ? 'All permissions' : perm.name
}

function groupPermissions(perms: Permission[]): { group: string; perms: Permission[] }[] {
  const groups = new Map<string, Permission[]>()
  for (const p of perms) {
    const group = p.name.includes(':') ? p.name.split(':')[0] : 'system'
    if (!groups.has(group)) groups.set(group, [])
    groups.get(group)!.push(p)
  }
  return [...groups.entries()].map(([group, perms]) => ({ group, perms }))
}

const permissionGroups = computed(() => groupPermissions(allPermissions.value))
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-base font-medium">Roles ({{ roles.length }})</h3>
      <Button @click="openCreate">
        <Plus class="mr-2 size-4" /> New Role
      </Button>
    </div>

    <div v-if="roles.length === 0" class="flex flex-col items-center justify-center py-12 text-center">
      <Shield class="size-10 text-muted-foreground/40 mb-3" />
      <p class="text-sm text-muted-foreground">No roles defined</p>
    </div>
    <Table v-else>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Description</TableHead>
          <TableHead>Permissions</TableHead>
          <TableHead class="w-24 text-right">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="role in roles" :key="role.id" class="group">
          <TableCell>
            <div class="flex items-center gap-2">
              <span class="font-medium">{{ role.name }}</span>
              <Badge v-if="isSuperadminRole(role)" variant="secondary" class="text-xs">
                <ShieldCheck class="mr-1 size-3" /> Protected
              </Badge>
            </div>
          </TableCell>
          <TableCell class="text-muted-foreground">{{ role.description || '—' }}</TableCell>
          <TableCell>
            <span class="text-sm text-muted-foreground">{{ role.permissions?.length ?? 0 }} permissions</span>
          </TableCell>
          <TableCell class="text-right">
            <div class="flex items-center justify-end gap-1">
              <Button variant="ghost" size="icon-sm" title="Edit role" @click="openEdit(role)">
                <Pencil class="size-3.5" />
              </Button>
              <Button
                v-if="!isSuperadminRole(role)"
                variant="ghost"
                size="icon-sm"
                title="Delete role"
                @click="deleteRole(role)"
              >
                <Trash2 class="size-3.5" />
              </Button>
            </div>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>

    <Dialog v-model:open="modalOpen">
      <DialogContent class="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{{ isEditing ? `Edit role: ${editingRole?.name}` : 'New role' }}</DialogTitle>
          <DialogDescription>
            {{ isEditing ? 'Update the role name and its permissions.' : 'Create a role and assign its permissions.' }}
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4">
          <div class="grid grid-cols-2 gap-3">
            <div class="space-y-2">
              <Label for="role-name">Name</Label>
              <Input id="role-name" v-model="formName" placeholder="e.g. Sales Manager" />
            </div>
            <div class="space-y-2">
              <Label for="role-desc">Description</Label>
              <Input id="role-desc" v-model="formDesc" placeholder="Optional" />
            </div>
          </div>

          <div>
            <div class="mb-2 flex items-center justify-between">
              <Label class="text-sm font-medium">Permissions</Label>
              <Button
                v-if="!isEditing || !editingRole || !isSuperadminRole(editingRole)"
                variant="ghost"
                size="sm"
                @click="
                  formPermissions.size === allPermissions.length
                    ? (formPermissions = new Set())
                    : (formPermissions = new Set(allPermissions.map((p) => p.id)))
                "
              >
                {{ formPermissions.size === allPermissions.length ? 'Clear all' : 'Select all' }}
              </Button>
            </div>
            <div class="max-h-80 overflow-y-auto rounded-md border p-3">
              <div v-for="group in permissionGroups" :key="group.group" class="mb-3 last:mb-0">
                <p class="mb-1.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  {{ group.group }}
                </p>
                <div class="grid grid-cols-2 gap-x-3 gap-y-1">
                  <div
                    v-for="perm in group.perms"
                    :key="perm.id"
                    class="flex items-center gap-2 rounded px-1 py-0.5 hover:bg-muted/50"
                  >
                    <Checkbox
                      :checked="formPermissions.has(perm.id)"
                      :disabled="isEditing && !!editingRole && permissionIsLocked(editingRole, perm)"
                      :title="
                        isEditing && editingRole && permissionIsLocked(editingRole, perm)
                          ? 'The wildcard permission cannot be removed from the superadmin role'
                          : perm.description
                      "
                      @update:checked="togglePermission(perm.id)"
                    />
                    <Label class="cursor-pointer text-sm" :title="perm.description">
                      {{ permissionLabel(perm) }}
                    </Label>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-if="formError" class="text-sm text-destructive">{{ formError }}</div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="modalOpen = false">Cancel</Button>
          <Button @click="saveRole" :disabled="saving">
            {{ saving ? 'Saving…' : isEditing ? 'Save changes' : 'Create role' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
