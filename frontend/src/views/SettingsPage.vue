<script setup lang="ts">
import { ref } from 'vue'
import { useActivityStore } from '@/stores/activity'
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
import SettingsTabUsers from '@/components/settings/SettingsTabUsers.vue'
import SettingsTabRoles from '@/components/settings/SettingsTabRoles.vue'
import SettingsTabPipelines from '@/components/settings/SettingsTabPipelines.vue'

const activity = useActivityStore()
const activityPage = ref(1)

function loadActivity() {
  activity.fetchActivity(activityPage.value, 20)
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

        <TabsContent value="users" class="mt-4">
          <SettingsTabUsers />
        </TabsContent>

        <TabsContent value="roles" class="mt-4">
          <SettingsTabRoles />
        </TabsContent>

        <TabsContent value="pipelines" class="mt-4">
          <SettingsTabPipelines />
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
