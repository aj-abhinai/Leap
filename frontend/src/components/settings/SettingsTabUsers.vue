<script setup lang="ts">
import { onMounted, shallowRef, computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useRBACStore } from '@/stores/rbac'
import {
  listUsers,
  createUser as apiCreateUser,
  updateUser as apiUpdateUser,
  resetUserPassword as apiResetUserPassword,
  deleteUser as apiDeleteUser,
  reactivateUser as apiReactivateUser,
  setUserRole as apiSetRole,
  type User,
} from '@/api/users'
import { listRoles, type Role } from '@/api/roles'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'
import { Plus, ShieldCheck, Pencil, KeyRound, UserX, UserCheck, User as UserIcon } from '@lucide/vue'
import { PASSWORD_POLICY_HINT, isStrongPassword } from '@/lib/validation'
import { errorMessage } from '@/utils/errors'

const users = shallowRef<User[]>([])
const roles = shallowRef<Role[]>([])
const auth = useAuthStore()
const rbac = useRBACStore()
const newUserName = shallowRef('')
const newUserEmail = shallowRef('')
const newUserPassword = shallowRef('')
const newUserRoleId = shallowRef('')
const newUserError = shallowRef('')
const creatingUser = shallowRef(false)

// Edit state
const editingUser = shallowRef<User | null>(null)
const editName = shallowRef('')
const editEmail = shallowRef('')
const editPhone = shallowRef('')
const editError = shallowRef('')
const savingEdit = shallowRef(false)

// Reset-password state
const resettingUser = shallowRef<User | null>(null)
const resetPassword = shallowRef('')
const resetError = shallowRef('')
const resetting = shallowRef(false)

// Deactivate state
const deactivatingUser = shallowRef<User | null>(null)

// Only wildcard holders may assign the superadmin role; hide the option for
// everyone else so settings:manage users never hit a confusing 403.
const selectableRoles = computed(() => {
  if (rbac.can('*')) return roles.value
  return roles.value.filter((r) => r.name !== 'superadmin')
})

// roleOptionsFor returns the options for a user's role dropdown, always
// including the user's current role so the select never misrepresents a
// stored superadmin role as "No role" for viewers who cannot assign it.
function roleOptionsFor(u: User): Role[] {
  const current = u.role
  if (current && !selectableRoles.value.some((r) => r.id === current.id)) {
    return [current, ...selectableRoles.value]
  }
  return selectableRoles.value
}

onMounted(() => {
  loadUsers()
  loadRoles()
})

async function loadUsers() {
  try {
    const res = await listUsers()
    users.value = res.data
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to load users'))
  }
}

async function loadRoles() {
  try {
    const res = await listRoles()
    roles.value = res.data
    preselectSalesRole()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to load roles'))
  }
}

// The new-user form preselects the Sales role instead of carrying a per-role
// default flag; fall back to no role when Sales is missing or not assignable.
function preselectSalesRole() {
  if (!newUserRoleId.value) {
    const sales = selectableRoles.value.find((r) => r.name === 'Sales')
    if (sales) newUserRoleId.value = sales.id
  }
}

async function createUser() {
  newUserError.value = ''
  if (!newUserName.value || !newUserEmail.value || !newUserPassword.value) {
    newUserError.value = 'All fields are required'
    return
  }
  if (!isStrongPassword(newUserPassword.value)) {
    newUserError.value = PASSWORD_POLICY_HINT
    return
  }
  creatingUser.value = true
  try {
    await apiCreateUser({
      name: newUserName.value,
      email: newUserEmail.value,
      password: newUserPassword.value,
      role_id: newUserRoleId.value || undefined,
    })
    toast.success('User created')
    newUserName.value = ''
    newUserEmail.value = ''
    newUserPassword.value = ''
    newUserRoleId.value = ''
    preselectSalesRole()
    loadUsers()
  } catch (e) {
    newUserError.value = errorMessage(e, 'Failed to create user')
  } finally {
    creatingUser.value = false
  }
}

function openEdit(u: User) {
  editingUser.value = u
  editName.value = u.name
  editEmail.value = u.email
  editPhone.value = u.phone ?? ''
  editError.value = ''
}

async function saveEdit() {
  const u = editingUser.value
  if (!u) return
  editError.value = ''
  if (!editName.value.trim() || !editEmail.value.trim()) {
    editError.value = 'Name and email are required'
    return
  }
  savingEdit.value = true
  try {
    await apiUpdateUser(u.id, {
      name: editName.value.trim(),
      email: editEmail.value.trim(),
      phone: editPhone.value.trim() || undefined,
    })
    toast.success('User updated')
    editingUser.value = null
    loadUsers()
  } catch (e) {
    editError.value = errorMessage(e, 'Failed to update user')
  } finally {
    savingEdit.value = false
  }
}

function openResetPassword(u: User) {
  resettingUser.value = u
  resetPassword.value = ''
  resetError.value = ''
}

async function saveResetPassword() {
  const u = resettingUser.value
  if (!u) return
  resetError.value = ''
  if (!isStrongPassword(resetPassword.value)) {
    resetError.value = PASSWORD_POLICY_HINT
    return
  }
  resetting.value = true
  try {
    await apiResetUserPassword(u.id, resetPassword.value)
    toast.success('Password reset — the user must change it at next login')
    resettingUser.value = null
  } catch (e) {
    resetError.value = errorMessage(e, 'Failed to reset password')
  } finally {
    resetting.value = false
  }
}

function requestDeactivate(u: User) {
  deactivatingUser.value = u
}

async function deactivateUser() {
  const u = deactivatingUser.value
  if (!u) return
  try {
    await apiDeleteUser(u.id)
    toast.success('User deactivated')
    loadUsers()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to deactivate user'))
  } finally {
    deactivatingUser.value = null
  }
}

async function reactivateUser(u: User) {
  try {
    await apiReactivateUser(u.id)
    toast.success('User reactivated')
    loadUsers()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to reactivate user'))
  }
}

async function setRole(userId: string, roleId: string) {
  try {
    await apiSetRole(userId, roleId)
    toast.success('Role updated')
    loadUsers()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to update role'))
  }
}

function isProtectedUser(u: User): boolean {
  return !!u.protected || auth.user?.id === u.id
}

// Precomputed per render so the row template calls the check once instead of
// twice, and the select handler stays typed.
const protectedIds = computed(() => new Set(users.value.filter(isProtectedUser).map((u) => u.id)))

function onRoleChange(u: User, event: Event) {
  setRole(u.id, (event.target as HTMLSelectElement).value)
}

const deactivatingName = computed(() => deactivatingUser.value?.name ?? '')
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
          <select
            v-model="newUserRoleId"
            class="h-10 w-36 rounded-md border bg-background px-2 text-sm"
            :aria-label="`Role for ${newUserName || 'new user'}`"
          >
            <option value="">No role</option>
            <option v-for="r in selectableRoles" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select>
          <Input v-model="newUserPassword" placeholder="Password (10+ chars, strong)" type="password" class="min-w-32 flex-1" />
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
          <UserIcon class="size-10 text-muted-foreground/40 mb-3" />
          <p class="text-sm text-muted-foreground">No users found</p>
        </div>
        <Table v-else>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Role</TableHead>
              <TableHead class="w-40">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="u in users" :key="u.id" :class="{ 'opacity-50': u.active === false }">
              <TableCell class="font-medium">
                <div class="flex items-center gap-2">
                  {{ u.name }}
                  <Badge v-if="u.active === false" variant="secondary" class="text-xs">Deactivated</Badge>
                </div>
              </TableCell>
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
                    :aria-label="`Role for ${u.name}`"
                    :disabled="u.active === false || protectedIds.has(u.id)"
                    @change="onRoleChange(u, $event)"
                  >
                    <option value="">No role</option>
                    <option v-for="r in roleOptionsFor(u)" :key="r.id" :value="r.id">{{ r.name }}</option>
                  </select>
                  <ShieldCheck
                    v-if="u.protected"
                    class="size-3.5 text-muted-foreground"
                    title="Protected"
                  />
                </div>
              </TableCell>
              <TableCell>
                <div v-if="u.active === false" class="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    title="Reactivate user"
                    :aria-label="`Reactivate ${u.name}`"
                    @click="reactivateUser(u)"
                  >
                    <UserCheck class="size-3.5" />
                  </Button>
                </div>
                <div v-else class="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    title="Edit user"
                    :aria-label="`Edit ${u.name}`"
                    @click="openEdit(u)"
                  >
                    <Pencil class="size-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    title="Reset password"
                    :aria-label="`Reset password for ${u.name}`"
                    @click="openResetPassword(u)"
                  >
                    <KeyRound class="size-3.5" />
                  </Button>
                  <Button
                    v-if="!protectedIds.has(u.id)"
                    variant="ghost"
                    size="icon-sm"
                    title="Deactivate user"
                    :aria-label="`Deactivate ${u.name}`"
                    @click="requestDeactivate(u)"
                  >
                    <UserX class="size-3.5" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>

    <Dialog :open="!!editingUser" @update:open="(v) => { if (!v) editingUser = null }">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Edit user</DialogTitle>
          <DialogDescription>Update the identity details of {{ editingUser?.name }}.</DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-2">
          <div class="space-y-2">
            <Label for="edit-name">Name</Label>
            <Input id="edit-name" v-model="editName" />
          </div>
          <div class="space-y-2">
            <Label for="edit-email">Email</Label>
            <Input id="edit-email" v-model="editEmail" type="email" />
          </div>
          <div class="space-y-2">
            <Label for="edit-phone">Phone</Label>
            <Input id="edit-phone" v-model="editPhone" placeholder="98765 43210 or +971 50 123 4567" />
          </div>
          <p v-if="editError" class="text-sm text-destructive">{{ editError }}</p>
        </div>
        <DialogFooter>
          <Button variant="ghost" @click="editingUser = null">Cancel</Button>
          <Button :disabled="savingEdit" @click="saveEdit">
            {{ savingEdit ? 'Saving…' : 'Save changes' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog :open="!!resettingUser" @update:open="(v) => { if (!v) resettingUser = null }">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Reset password</DialogTitle>
          <DialogDescription>
            Set a temporary password for {{ resettingUser?.name }}. They must choose their own at next login.
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-2">
          <div class="space-y-2">
            <Label for="reset-password">Temporary password</Label>
            <Input id="reset-password" v-model="resetPassword" type="password" :placeholder="PASSWORD_POLICY_HINT" />
          </div>
          <p v-if="resetError" class="text-sm text-destructive">{{ resetError }}</p>
        </div>
        <DialogFooter>
          <Button variant="ghost" @click="resettingUser = null">Cancel</Button>
          <Button :disabled="resetting" @click="saveResetPassword">
            {{ resetting ? 'Resetting…' : 'Reset password' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <ConfirmDialog
      :open="!!deactivatingUser"
      title="Deactivate user"
      :description="`Deactivate ${deactivatingName}? They cannot log in anymore, their work and history stay, and the account can be reactivated.`"
      confirm-text="Deactivate"
      destructive
      @update:open="(v) => { if (!v) deactivatingUser = null }"
      @confirm="deactivateUser"
    />
  </div>
</template>