<script setup lang="ts">
import { shallowRef, onMounted } from 'vue'
import { useActivityStore } from '@/stores/activity'
import { useRBACStore } from '@/stores/rbac'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
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
import SettingsTabUsers from '@/components/settings/SettingsTabUsers.vue'
import SettingsTabRoles from '@/components/settings/SettingsTabRoles.vue'
import SettingsTabPipelines from '@/components/settings/SettingsTabPipelines.vue'
import SettingsTabGeneral from '@/components/settings/SettingsTabGeneral.vue'
import { RefreshCw, User, Shield, Layers, Activity, Settings } from '@lucide/vue'

const activity = useActivityStore()
const rbac = useRBACStore()
const activityPage = shallowRef(1)

const permissionsLoaded = shallowRef(false)

onMounted(async () => {
  await rbac.fetchPermissions()
  permissionsLoaded.value = true
})

function canOrLoading(permission: string): boolean {
  if (!permissionsLoaded.value) return true
  return rbac.can(permission)
}

function loadActivity() {
  activity.fetchActivity(activityPage.value, 20)
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
  <LayoutShell>
    <div class="flex flex-1 flex-col gap-4 p-6 pt-2">
      <Tabs defaultValue="general" class="w-full">
        <TabsList class="mb-4 w-full justify-start rounded-lg border bg-muted/50 p-1">
          <TabsTrigger value="general" class="gap-2 rounded-md data-[state=active]:bg-background data-[state=active]:shadow-sm">
            <Settings class="size-4" />
            <span class="hidden sm:inline">General</span>
          </TabsTrigger>
          <TabsTrigger v-show="canOrLoading('rbac:manage')" value="users" class="gap-2 rounded-md data-[state=active]:bg-background data-[state=active]:shadow-sm">
            <User class="size-4" />
            <span class="hidden sm:inline">Users</span>
          </TabsTrigger>
          <TabsTrigger v-show="canOrLoading('rbac:manage')" value="roles" class="gap-2 rounded-md data-[state=active]:bg-background data-[state=active]:shadow-sm">
            <Shield class="size-4" />
            <span class="hidden sm:inline">Roles</span>
          </TabsTrigger>
          <TabsTrigger value="pipelines" class="gap-2 rounded-md data-[state=active]:bg-background data-[state=active]:shadow-sm">
            <Layers class="size-4" />
            <span class="hidden sm:inline">Pipelines</span>
          </TabsTrigger>
          <TabsTrigger v-show="canOrLoading('activity:read')" value="activity" class="gap-2 rounded-md data-[state=active]:bg-background data-[state=active]:shadow-sm">
            <Activity class="size-4" />
            <span class="hidden sm:inline">Activity</span>
          </TabsTrigger>
        </TabsList>

        <TabsContent value="general" class="mt-0">
          <SettingsTabGeneral />
        </TabsContent>

        <TabsContent v-if="canOrLoading('rbac:manage')" value="users" class="mt-0">
          <SettingsTabUsers />
        </TabsContent>

        <TabsContent v-if="canOrLoading('rbac:manage')" value="roles" class="mt-0">
          <SettingsTabRoles />
        </TabsContent>

        <TabsContent value="pipelines" class="mt-0">
          <SettingsTabPipelines />
        </TabsContent>

        <TabsContent v-if="canOrLoading('activity:read')" value="activity" class="mt-0">
          <Card>
            <CardHeader class="flex flex-row items-center justify-between">
              <CardTitle>Activity Log</CardTitle>
              <Button variant="outline" size="sm" @click="loadActivity()">
                <RefreshCw class="mr-2 size-3.5" /> Refresh
              </Button>
            </CardHeader>
            <CardContent>
              <div v-if="activity.entries.length === 0" class="flex flex-col items-center justify-center py-12 text-center">
                <Activity class="size-10 text-muted-foreground/40 mb-3" />
                <p class="text-sm font-medium text-muted-foreground">No activity logged yet</p>
                <p class="text-xs text-muted-foreground/60 mt-1">Actions in the CRM will appear here</p>
              </div>
              <Table v-else>
                <TableHeader>
                  <TableRow>
                    <TableHead>Description</TableHead>
                    <TableHead>Action</TableHead>
                    <TableHead>Type</TableHead>
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
                    <TableCell class="text-right text-xs text-muted-foreground tabular-nums">
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
