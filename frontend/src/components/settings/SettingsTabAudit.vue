<script setup lang="ts">
import { shallowRef, computed, onMounted } from 'vue'
import { useActivityStore } from '@/stores/activity'
import { useRBACStore } from '@/stores/rbac'
import { listUsers, type User } from '@/api/users'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge, type BadgeVariants } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import SettingsTabExport from '@/components/settings/SettingsTabExport.vue'
import SettingsSectionMenu from '@/components/settings/SettingsSectionMenu.vue'
import { RefreshCw, ScrollText } from '@lucide/vue'
import { formatDateTime } from '@/utils/time'

// Audit log tab: the mutation trail and the data-export card. The section row
// filters by permission — the trail needs settings:manage, the card needs
// data:export — so each user opens the first section they hold.
const activity = useActivityStore()
const rbac = useRBACStore()
const activityPage = shallowRef(1)
const activityPerPage = 20
// Reka Select cannot carry an empty string as an item value, so the "All …"
// choice travels as this sentinel and maps back to '' for the store call.
const ALL = '__all__'
const activityAction = shallowRef('')
const activityResourceType = shallowRef('')
const activityUserId = shallowRef('')
const users = shallowRef<User[]>([])

const activityTotalPages = computed(() => Math.ceil(activity.total / activityPerPage) || 1)

// The section row is permission-filtered: the trail stays settings:manage and
// the export card stays data:export; the first accessible section opens on
// each visit (ADR 015), so a data:export-only user lands on Export.
const sections = computed(() => {
  const list: { value: string; label: string }[] = []
  if (rbac.can('settings:manage')) list.push({ value: 'audit', label: 'Audit Log' })
  if (rbac.can('data:export')) list.push({ value: 'export', label: 'Export' })
  return list
})

onMounted(() => {
  if (rbac.can('settings:manage')) {
    loadUsers()
    loadActivity()
  }
})

// The actor filter lists the live users, so a manager can trace one person's
// mutations; the backend user_id filter is a settings:manage read.
async function loadUsers() {
  try {
    const res = await listUsers()
    users.value = res.data
  } catch {}
}

function loadActivity() {
  activity.fetchActivity(activityPage.value, activityPerPage, {
    action: activityAction.value,
    resourceType: activityResourceType.value,
    userId: activityUserId.value,
  })
}

function applyActivityFilters() {
  activityPage.value = 1
  loadActivity()
}

// Filter setters: sentinel ↔ '' mapping plus the page reset that the old
// select @change handlers performed.
function setActivityUser(v: unknown) {
  activityUserId.value = v === ALL ? '' : String(v)
  applyActivityFilters()
}

function setActivityAction(v: unknown) {
  activityAction.value = v === ALL ? '' : String(v)
  applyActivityFilters()
}

function setActivityResourceType(v: unknown) {
  activityResourceType.value = v === ALL ? '' : String(v)
  applyActivityFilters()
}

function activityPrevPage() {
  if (activityPage.value <= 1) return
  activityPage.value--
  loadActivity()
}

function activityNextPage() {
  if (activityPage.value >= activityTotalPages.value) return
  activityPage.value++
  loadActivity()
}

function resourceBadgeVariant(type: string): BadgeVariants['variant'] {
  const map: Record<string, BadgeVariants['variant']> = {
    contact: 'default',
    lead: 'secondary',
    user: 'outline',
    role: 'outline',
    pipeline: 'outline',
    program: 'outline',
    tag: 'outline',
    settings: 'outline',
  }
  return map[type] ?? 'outline'
}
</script>

<template>
  <SettingsSectionMenu :sections="sections">
    <template #audit>
      <Card>
        <CardHeader class="flex flex-row items-center justify-between">
          <CardTitle>Audit Log</CardTitle>
          <Button variant="outline" size="sm" @click="loadActivity()">
            <RefreshCw class="mr-2 size-3.5" /> Refresh
          </Button>
        </CardHeader>
        <CardContent>
          <div class="mb-4 flex flex-wrap gap-2">
            <Select
              :model-value="activityUserId || ALL"
              @update:model-value="setActivityUser"
            >
              <SelectTrigger size="sm" class="w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem :value="ALL">All users</SelectItem>
                <SelectItem v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</SelectItem>
              </SelectContent>
            </Select>
            <Select
              :model-value="activityAction || ALL"
              @update:model-value="setActivityAction"
            >
              <SelectTrigger size="sm" class="w-44">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem :value="ALL">All actions</SelectItem>
                <SelectItem value="create">Create</SelectItem>
                <SelectItem value="update">Update</SelectItem>
                <SelectItem value="delete">Delete</SelectItem>
                <SelectItem value="import">Import</SelectItem>
                <SelectItem value="login">Login</SelectItem>
                <SelectItem value="logout">Logout</SelectItem>
                <SelectItem value="password_change">Password change</SelectItem>
                <SelectItem value="profile_update">Profile update</SelectItem>
                <SelectItem value="reset_password">Reset password</SelectItem>
                <SelectItem value="reactivate">Reactivate</SelectItem>
              </SelectContent>
            </Select>
            <Select
              :model-value="activityResourceType || ALL"
              @update:model-value="setActivityResourceType"
            >
              <SelectTrigger size="sm" class="w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem :value="ALL">All types</SelectItem>
                <SelectItem value="contact">Contact</SelectItem>
                <SelectItem value="lead">Lead</SelectItem>
                <SelectItem value="user">User</SelectItem>
                <SelectItem value="role">Role</SelectItem>
                <SelectItem value="pipeline">Pipeline</SelectItem>
                <SelectItem value="program">Program</SelectItem>
                <SelectItem value="tag">Vocabulary</SelectItem>
                <SelectItem value="settings">Org settings</SelectItem>
                <SelectItem value="contact_note">Note</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div v-if="activity.entries.length === 0" class="flex flex-col items-center justify-center py-12 text-center">
            <ScrollText class="size-10 text-muted-foreground/40 mb-3" />
            <p class="text-sm font-medium text-muted-foreground">No activity logged yet</p>
            <p class="text-xs text-muted-foreground/60 mt-1">Mutations in Leap will appear here</p>
          </div>
          <Table v-else>
            <TableHeader>
              <TableRow>
                <TableHead>Description</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Actor</TableHead>
                <TableHead class="text-right">Date</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="e in activity.entries" :key="e.id" class="group">
                <TableCell class="font-medium">{{ e.description }}</TableCell>
                <TableCell>{{ e.action }}</TableCell>
                <TableCell>
                  <Badge :variant="resourceBadgeVariant(e.resource_type)" class="text-xs">
                    {{ e.resource_type }}
                  </Badge>
                </TableCell>
                <TableCell class="text-xs text-muted-foreground">{{ e.user_name || '—' }}</TableCell>
                <TableCell class="text-right text-xs text-muted-foreground tabular-nums">
                  {{ formatDateTime(e.created_at) }}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <div class="mt-4 flex items-center justify-between">
            <span class="text-sm text-muted-foreground">
              Page {{ activityPage }} of {{ activityTotalPages }} &middot; {{ activity.total }} total
            </span>
            <div class="flex items-center gap-1">
              <Button variant="outline" size="sm" :disabled="activityPage <= 1" @click="activityPrevPage()">
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                :disabled="activityPage >= activityTotalPages"
                @click="activityNextPage()"
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </template>
    <template #export>
      <SettingsTabExport />
    </template>
  </SettingsSectionMenu>
</template>
