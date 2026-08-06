<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { useContactsStore } from '@/stores/contacts'
import { useLeadsStore } from '@/stores/leads'
import { usePipelineStore } from '@/stores/pipeline'
import { useActivityStore } from '@/stores/activity'
import { useRemindersStore } from '@/stores/reminders'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { Users, FolderOpen, Bell, BellOff, TrendingUp, Calendar, Phone, Mail, UserPlus, ArrowRight } from '@lucide/vue'
import { timeAgo } from '@/utils/time'
import { formatCurrency } from '@/utils/format'
import { formatReminderText, reminderIcon } from '@/utils/reminders'

const router = useRouter()
const contactsStore = useContactsStore()
const leadsStore = useLeadsStore()
const pipelineStore = usePipelineStore()
const activity = useActivityStore()
const remindersStore = useRemindersStore()
const loading = shallowRef(true)

onMounted(async () => {
  try {
    await Promise.all([
      contactsStore.fetchTotal(),
      leadsStore.fetchTotal(),
      activity.fetchActivity(1, 10),
      remindersStore.fetchReminders(),
      pipelineStore.fetchPipelines(),
    ])
    // Load the first pipeline's leads so the stage distribution has data.
    if (pipelineStore.pipelines.length > 0) {
      await leadsStore.fetchLeads(pipelineStore.pipelines[0].id, '', 1, 200)
    }
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

const pipeline = computed(() => pipelineStore.pipelines[0] ?? null)
const pipelineLeads = computed(() => leadsStore.leads)

const stageRows = computed(() => {
  if (!pipeline.value?.stages) return []
  return pipeline.value.stages.map((stage) => {
    const stageLeads = pipelineLeads.value.filter((l) => l.stage_id === stage.id)
    return {
      name: stage.name,
      count: stageLeads.length,
      value: stageLeads.reduce((sum, l) => sum + (l.value || 0), 0),
      isClosing: stage.is_closing,
    }
  })
})

const stageMax = computed(() => Math.max(1, ...stageRows.value.map((r) => r.count)))

const wonCount = computed(() => pipelineLeads.value.filter((l) => l.outcome === 'won').length)
const lostCount = computed(() => pipelineLeads.value.filter((l) => l.outcome === 'lost').length)
const openCount = computed(() => Math.max(0, pipelineLeads.value.length - wonCount.value - lostCount.value))
const totalCount = computed(() => Math.max(1, pipelineLeads.value.length))

const reminders = computed(() => remindersStore.reminders.slice(0, 5))

function goToLeads() {
  router.push({ name: 'Leads' })
}

function goToReminders() {
  router.push({ name: 'Reminders' })
}
</script>

<template>
  <LayoutShell>
    <div class="mx-auto flex w-full max-w-6xl flex-col gap-6 p-6 pt-2">
      <div class="flex flex-col">
        <h1 class="text-2xl font-semibold tracking-tight">Dashboard</h1>
        <p class="mt-0.5 text-sm text-muted-foreground">Pipeline health, reminders, and recent activity</p>
      </div>

      <!-- Counters -->
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
        <Card v-else class="relative overflow-hidden">
          <div class="absolute inset-x-0 top-0 h-1 bg-linear-to-r from-primary to-primary/60" />
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Total Contacts</CardTitle>
            <div class="flex size-8 items-center justify-center rounded-lg bg-primary/10">
              <Users class="size-4 text-primary" />
            </div>
          </CardHeader>
          <CardContent>
            <div class="text-3xl font-bold tracking-tight tabular-nums">{{ contactsStore.total }}</div>
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
        <Card v-else class="relative overflow-hidden">
          <div class="absolute inset-x-0 top-0 h-1 bg-linear-to-r from-primary to-primary/60" />
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Active Leads</CardTitle>
            <div class="flex size-8 items-center justify-center rounded-lg bg-primary/10">
              <FolderOpen class="size-4 text-primary" />
            </div>
          </CardHeader>
          <CardContent>
            <div class="text-3xl font-bold tracking-tight tabular-nums">{{ leadsStore.total }}</div>
            <p class="mt-1 text-xs text-muted-foreground">Open leads across all pipelines</p>
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
        <Card v-else class="relative overflow-hidden">
          <div class="absolute inset-x-0 top-0 h-1 bg-linear-to-r from-primary to-primary/60" />
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Pending Reminders</CardTitle>
            <div class="flex size-8 items-center justify-center rounded-lg bg-primary/10">
              <Bell class="size-4 text-primary" />
            </div>
          </CardHeader>
          <CardContent>
            <div class="text-3xl font-bold tracking-tight tabular-nums">{{ remindersStore.reminders.length }}</div>
            <p class="mt-1 text-xs text-muted-foreground">Awaiting action across leads</p>
          </CardContent>
        </Card>
      </div>

      <!-- Pipeline health -->
      <Card>
        <CardHeader class="flex flex-row items-center justify-between">
          <div class="flex items-center gap-2">
            <TrendingUp class="size-4 text-muted-foreground" />
            <CardTitle>{{ pipeline ? `${pipeline.name} health` : 'Pipeline health' }}</CardTitle>
          </div>
          <Button variant="ghost" size="sm" class="text-muted-foreground" @click="goToLeads">
            View kanban <ArrowRight class="ml-1 size-3.5" />
          </Button>
        </CardHeader>
        <CardContent>
          <div v-if="loading" class="space-y-3">
            <div v-for="i in 5" :key="i" class="flex items-center gap-3">
              <Skeleton class="h-4 w-28" />
              <Skeleton class="h-3 flex-1" />
              <Skeleton class="h-4 w-12" />
            </div>
          </div>
          <div v-else-if="stageRows.length === 0" class="flex flex-col items-center justify-center py-10 text-center">
            <FolderOpen class="size-10 text-muted-foreground/40 mb-3" />
            <p class="text-sm font-medium text-muted-foreground">No pipeline configured</p>
            <p class="text-xs text-muted-foreground/60 mt-1">Create a pipeline in Settings to see stage health</p>
          </div>
          <div v-else class="space-y-5">
            <div class="space-y-3">
              <div v-for="row in stageRows" :key="row.name" class="flex items-center gap-3">
                <span class="w-36 shrink-0 truncate text-sm" :class="row.isClosing ? 'font-medium' : 'text-muted-foreground'">
                  {{ row.name }}
                </span>
                <div class="h-2 flex-1 overflow-hidden rounded-full bg-muted">
                  <div
                    class="h-full rounded-full bg-primary/70 transition-all duration-500"
                    :style="{ width: `${(row.count / stageMax) * 100}%` }"
                  ></div>
                </div>
                <span class="w-8 shrink-0 text-right text-sm tabular-nums font-medium">{{ row.count }}</span>
                <span class="hidden w-24 shrink-0 text-right text-xs text-muted-foreground tabular-nums sm:block">
                  {{ row.value ? formatCurrency(row.value) : '–' }}
                </span>
              </div>
            </div>
            <div class="border-t pt-4">
              <div class="flex h-2 overflow-hidden rounded-full bg-muted">
                <div
                  class="h-full bg-primary transition-all duration-500"
                  :style="{ width: `${(openCount / totalCount) * 100}%` }"
                  title="Open"
                ></div>
                <div
                  class="h-full bg-success transition-all duration-500"
                  :style="{ width: `${(wonCount / totalCount) * 100}%` }"
                  title="Won"
                ></div>
                <div
                  class="h-full bg-destructive transition-all duration-500"
                  :style="{ width: `${(lostCount / totalCount) * 100}%` }"
                  title="Lost"
                ></div>
              </div>
              <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                <span class="inline-flex items-center gap-1.5">
                  <span class="size-2 rounded-full bg-primary"></span> Open {{ openCount }}
                </span>
                <span class="inline-flex items-center gap-1.5">
                  <span class="size-2 rounded-full bg-success"></span> Won {{ wonCount }}
                </span>
                <span class="inline-flex items-center gap-1.5">
                  <span class="size-2 rounded-full bg-destructive"></span> Lost {{ lostCount }}
                </span>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Reminders + activity -->
      <div class="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader class="flex flex-row items-center justify-between">
            <div class="flex items-center gap-2">
              <Bell class="size-4 text-muted-foreground" />
              <CardTitle>Upcoming reminders</CardTitle>
            </div>
            <Button variant="ghost" size="sm" class="text-muted-foreground" @click="goToReminders">
              View all <ArrowRight class="ml-1 size-3.5" />
            </Button>
          </CardHeader>
          <CardContent>
            <div v-if="loading" class="space-y-3">
              <div v-for="i in 5" :key="i" class="flex items-center gap-3">
                <Skeleton class="size-8 rounded-full" />
                <div class="flex-1 space-y-1.5">
                  <Skeleton class="h-4 w-3/4" />
                  <Skeleton class="h-3 w-1/2" />
                </div>
              </div>
            </div>
            <div v-else-if="reminders.length === 0" class="flex flex-col items-center justify-center py-10 text-center">
              <BellOff class="size-10 text-muted-foreground/40 mb-3" />
              <p class="text-sm font-medium text-muted-foreground">No pending reminders</p>
              <p class="text-xs text-muted-foreground/60 mt-1">Create activities with reminders from the leads kanban</p>
            </div>
            <div v-else class="space-y-1">
              <div
                v-for="r in reminders"
                :key="r.id"
                class="flex items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-muted/50"
              >
                <div class="flex size-8 shrink-0 items-center justify-center rounded-full bg-muted">
                  <component :is="reminderIcon(r.type)" class="size-3.5 text-muted-foreground" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="truncate text-sm">{{ formatReminderText(r) }}</p>
                  <p class="mt-0.5 text-xs text-muted-foreground">
                    {{ r.lead_id ? 'Lead reminder' : '' }}{{ r.stage_name ? ` · ${r.stage_name}` : '' }}
                  </p>
                </div>
                <span class="shrink-0 text-xs text-muted-foreground tabular-nums">
                  {{ r.remind_at ? timeAgo(r.remind_at) : r.scheduled_at ? timeAgo(r.scheduled_at) : '' }}
                </span>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
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
            <div v-else-if="activity.entries.length === 0" class="flex flex-col items-center justify-center py-10 text-center">
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
    </div>
  </LayoutShell>
</template>
