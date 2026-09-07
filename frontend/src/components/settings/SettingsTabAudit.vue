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
import SettingsTabExport from '@/components/settings/SettingsTabExport.vue'
import { RefreshCw, ScrollText } from '@lucide/vue'
import { formatDateTime } from '@/utils/time'

// Audit log tab: the mutation trail and the data-export card. The export
// card gates itself on data:export, so a settings:manage user without it
// still sees the trail, and a data:export-only user sees the card.
const activity = useActivityStore()
const rbac = useRBACStore()
const activityPage = shallowRef(1)
const activityPerPage = 20
const activityAction = shallowRef('')
const activityResourceType = shallowRef('')
const activityUserId = shallowRef('')
const users = shallowRef<User[]>([])

const activityTotalPages = computed(() => Math.ceil(activity.total / activityPerPage) || 1)

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
  }
  return map[type] ?? 'outline'
}
</script>

<template>
  <div class="space-y-4">
    <Card v-if="rbac.can('settings:manage')">
      <CardHeader class="flex flex-row items-center justify-between">
        <CardTitle>Audit Log</CardTitle>
        <Button variant="outline" size="sm" @click="loadActivity()">
          <RefreshCw class="mr-2 size-3.5" /> Refresh
        </Button>
      </CardHeader>
      <CardContent>
        <div class="mb-4 flex flex-wrap gap-2">
          <select
            v-model="activityUserId"
            class="h-8 rounded-md border bg-background px-2 text-sm"
            @change="applyActivityFilters()"
          >
            <option value="">All users</option>
            <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
          </select>
          <select
            v-model="activityAction"
            class="h-8 rounded-md border bg-background px-2 text-sm"
            @change="applyActivityFilters()"
          >
            <option value="">All actions</option>
            <option value="create">Create</option>
            <option value="update">Update</option>
            <option value="delete">Delete</option>
            <option value="move_stage">Move stage</option>
            <option value="reset_password">Reset password</option>
            <option value="reactivate">Reactivate</option>
          </select>
          <select
            v-model="activityResourceType"
            class="h-8 rounded-md border bg-background px-2 text-sm"
            @change="applyActivityFilters()"
          >
            <option value="">All types</option>
            <option value="contact">Contact</option>
            <option value="lead">Lead</option>
            <option value="user">User</option>
            <option value="role">Role</option>
            <option value="pipeline">Pipeline</option>
            <option value="program">Program</option>
            <option value="contact_note">Note</option>
          </select>
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
    <SettingsTabExport v-if="rbac.can('data:export')" />
  </div>
</template>