<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { useContactsStore } from '@/stores/contacts'
import { useLeadsStore } from '@/stores/leads'
import { useActivityStore } from '@/stores/activity'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Users, FolderOpen, TrendingUp, Calendar, Phone, Mail, UserPlus } from '@lucide/vue'
import { timeAgo } from '@/utils/time'

const contactsStore = useContactsStore()
const leadsStore = useLeadsStore()
const activity = useActivityStore()
const loading = shallowRef(true)

onMounted(async () => {
  try {
    await Promise.all([
      contactsStore.fetchTotal(),
      leadsStore.fetchTotal(),
      activity.fetchActivity(1, 10),
    ])
  } finally {
    loading.value = false
  }
})

function activityIcon(action: string) {
  const a = action.toLowerCase()
  if (a.includes('create') || a.includes('added')) return UserPlus
  if (a.includes('update') || a.includes('edit')) return Mail
  if (a.includes('call') || a.includes('phone')) return Phone
  return Calendar
}
</script>

<template>
  <LayoutShell>
    <div class="mx-auto flex w-full max-w-6xl flex-col gap-6 p-6 pt-2">
      <div class="grid gap-4 md:grid-cols-3">
        <Card v-if="loading" class="overflow-hidden">
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <Skeleton class="h-4 w-24" />
            <Skeleton class="size-8 rounded-lg" />
          </CardHeader>
          <CardContent>
            <Skeleton class="h-8 w-16" />
            <Skeleton class="mt-2 h-3 w-20" />
          </CardContent>
        </Card>
        <Card v-else class="relative overflow-hidden animate-fade-in-up">
          <div class="absolute inset-x-0 top-0 h-1 bg-linear-to-r from-primary to-primary/60" />
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Total Contacts</CardTitle>
            <div class="flex size-8 items-center justify-center rounded-lg bg-primary/10">
              <Users class="size-4 text-primary" />
            </div>
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ contactsStore.total }}</div>
            <p class="mt-1 text-xs text-muted-foreground">All contacts in database</p>
          </CardContent>
        </Card>

        <Card v-if="loading" class="overflow-hidden">
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <Skeleton class="h-4 w-20" />
            <Skeleton class="size-8 rounded-lg" />
          </CardHeader>
          <CardContent>
            <Skeleton class="h-8 w-16" />
            <Skeleton class="mt-2 h-3 w-24" />
          </CardContent>
        </Card>
        <Card v-else class="relative overflow-hidden animate-fade-in-up animate-fade-in-up-delay-1">
          <div class="absolute inset-x-0 top-0 h-1 bg-linear-to-r from-primary to-primary/60" />
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Active Leads</CardTitle>
            <div class="flex size-8 items-center justify-center rounded-lg bg-primary/10">
              <FolderOpen class="size-4 text-primary" />
            </div>
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ leadsStore.total }}</div>
            <p class="mt-1 text-xs text-muted-foreground">Open leads across all pipelines</p>
          </CardContent>
        </Card>
      </div>

      <Card class="animate-fade-in-up animate-fade-in-up-delay-3">
        <CardHeader class="flex flex-row items-center justify-between">
          <div class="flex items-center gap-2">
            <TrendingUp class="size-4 text-muted-foreground" />
            <CardTitle>Recent Activity</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div v-if="loading" class="space-y-3">
            <div v-for="i in 5" :key="i" class="flex items-center gap-3">
              <Skeleton class="size-8 rounded-full" />
              <div class="flex-1 space-y-1.5">
                <Skeleton class="h-4 w-3/4" />
                <Skeleton class="h-3 w-1/2" />
              </div>
              <Skeleton class="h-3 w-16" />
            </div>
          </div>
          <div v-else-if="activity.entries.length === 0" class="flex flex-col items-center justify-center py-8 text-center">
            <Calendar class="size-10 text-muted-foreground/40 mb-3" />
            <p class="text-sm font-medium text-muted-foreground">No recent activity</p>
            <p class="text-xs text-muted-foreground/60 mt-1">Activity will appear here as you use the CRM</p>
          </div>
          <div v-else class="space-y-1">
            <div
              v-for="entry in activity.entries"
              :key="entry.id"
              class="flex items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-muted/50"
            >
              <div class="flex size-8 shrink-0 items-center justify-center rounded-full bg-muted">
                <component :is="activityIcon(entry.action)" class="size-3.5 text-muted-foreground" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm">
                  <span class="font-medium">{{ entry.action }}</span>
                  <span class="text-muted-foreground"> · {{ entry.description }}</span>
                </p>
              </div>
              <span class="shrink-0 text-xs text-muted-foreground">
                {{ timeAgo(entry.created_at) }}
              </span>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </LayoutShell>
</template>
