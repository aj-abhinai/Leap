<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/composables/useApi'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Users, FolderOpen, CheckCircle2 } from '@lucide/vue'
import { useActivityStore, type ActivityEntry } from '@/stores/activity'

const totalContacts = ref(0)
const totalLeads = ref(0)
const activity = useActivityStore()

onMounted(async () => {
  try {
    const contactsRes = await apiClient.get('/api/contacts?per_page=1')
    totalContacts.value = contactsRes.meta?.total || 0
  } catch {}
  try {
    const leadsRes = await apiClient.get('/api/leads?per_page=1')
    totalLeads.value = leadsRes.meta?.total || 0
  } catch {}
  activity.fetchActivity(1, 10)
})
</script>

<template>
  <LayoutShell>
    <div class="flex flex-col gap-4 p-4 pt-0">
      <div class="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Total Contacts</CardTitle>
            <Users class="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ totalContacts }}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Active Leads</CardTitle>
            <FolderOpen class="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ totalLeads }}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Won This Month</CardTitle>
            <CheckCircle2 class="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">0</div>
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Recent Activity</CardTitle>
        </CardHeader>
        <CardContent>
          <div v-if="activity.entries.length === 0" class="text-sm text-muted-foreground">
            No recent activity
          </div>
          <div v-else class="space-y-2">
            <div
              v-for="entry in activity.entries.slice(0, 10)"
              :key="entry.id"
              class="flex items-center gap-2 text-sm"
            >
              <span class="font-medium">{{ entry.action }}</span>
              <span class="text-muted-foreground">{{ entry.description }}</span>
              <span class="ml-auto text-xs text-muted-foreground">
                {{ new Date(entry.created_at).toLocaleString() }}
              </span>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </LayoutShell>
</template>
