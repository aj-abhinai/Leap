<script setup lang="ts">
import { computed, nextTick, onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { useContactsStore } from '@/stores/contacts'
import { useLeadsStore } from '@/stores/leads'
import { usePipelineStore } from '@/stores/pipeline'
import { useActivitiesStore } from '@/stores/activities'
import { useRemindersStore } from '@/stores/reminders'
import { useRBACStore } from '@/stores/rbac'
import { useLeadDrawerGlobal } from '@/composables/useLeadDrawerGlobal'
import { fetchLeadStats, type StageStat } from '@/api/leads'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { Users, FolderOpen, Bell, BellOff, TrendingUp, Calendar, ArrowRight } from '@lucide/vue'
import { timeAgo } from '@/utils/time'
import { formatCurrency } from '@/utils/format'
import { iconFor, resolveTask } from '@/tasks'
import { useTasks } from '@/tasks/vue'
import type { Reminder } from '@/api/reminders'

const router = useRouter()
const contactsStore = useContactsStore()
const leadsStore = useLeadsStore()
const pipelineStore = usePipelineStore()
const activitiesStore = useActivitiesStore()
const remindersStore = useRemindersStore()
const tasks = useTasks()
const rbac = useRBACStore()
const { openLeadDrawer } = useLeadDrawerGlobal()
const loading = shallowRef(true)
// stats carries the first pipeline's server-side aggregates; the health
// bars render from it without loading lead rows.
const stats = shallowRef<StageStat[]>([])

// Bars render at zero and grow after the first paint, so the dashboard
// reads as loading into place instead of snapping. Replays on remount.
const barsIn = shallowRef(false)

onMounted(async () => {
  // App.vue fetches permissions fire-and-forget, which races this mount on a
  // hard refresh; awaiting them here means can() gates the fetches on the
  // real permission set instead of an empty one. fetchPermissions swallows
  // its own errors, so the dashboard still loads when it fails.
  await rbac.fetchPermissions()
  const fetches = [
    contactsStore.fetchTotal(),
    leadsStore.fetchTotal(),
    remindersStore.fetchReminders(),
  ]
  if (rbac.can('lead:read')) {
    fetches.push(activitiesStore.fetchRecent(10))
    // The catalog must be loaded before the stats call needs its first
    // pipeline id; chaining inside the batch keeps every other fetch parallel.
    fetches.push(
      pipelineStore.fetchPipelines().then(async () => {
        const first = pipelineStore.pipelines[0]
        if (!first) return
        const res = await fetchLeadStats({ pipelineId: first.id })
        stats.value = res.data?.stages ?? []
      }),
    )
  }
  try {
    await Promise.all(fetches)
  } catch {
    // Best-effort dashboard: one failed fetch must not blank the rest.
  } finally {
    loading.value = false
    // Two frames: the first paints the zero-width bars, the second flips
    // them so the width transition actually runs.
    await nextTick()
    requestAnimationFrame(() => requestAnimationFrame(() => { barsIn.value = true }))
  }
})

const pipeline = computed(() => pipelineStore.pipelines[0] ?? null)
const statsByStage = computed(() => new Map(stats.value.map((s) => [s.stage_id, s])))

// stageRows maps the first pipeline's stages to their server aggregates;
// stages without live leads are absent from the payload and read as zero.
const stageRows = computed(() => {
  if (!pipeline.value?.stages) return []
  return pipeline.value.stages.map((stage) => {
    const stat = statsByStage.value.get(stage.id)
    return {
      name: stage.name,
      count: stat?.count ?? 0,
      value: stat?.value_sum ?? 0,
      isClosing: stage.is_closing,
      outcome: stage.outcome,
    }
  })
})

const stageMax = computed(() => Math.max(1, ...stageRows.value.map((r) => r.count)))

const wonCount = computed(() => stageRows.value.filter((r) => r.outcome === 'won').reduce((n, r) => n + r.count, 0))
const lostCount = computed(() => stageRows.value.filter((r) => r.outcome === 'lost').reduce((n, r) => n + r.count, 0))
const openCount = computed(() => stageRows.value.filter((r) => r.outcome === 'open').reduce((n, r) => n + r.count, 0))
const totalCount = computed(() => Math.max(1, stageRows.value.reduce((n, r) => n + r.count, 0)))

const reminders = computed(() => remindersStore.openReminders.slice(0, 5))

// reminderTextOf renders the reminder's label for the dashboard list.
function reminderTextOf(r: Reminder): string {
  const resolved = resolveTask(r, tasks.env)
  return resolved.description ? `${resolved.typeName}: ${resolved.description}` : resolved.typeName
}

function goToLeads() {
  router.push({ name: 'Leads' })
}

function goToReminders() {
  router.push({ name: 'Reminders' })
}

function goToActivities() {
  router.push({ name: 'Activities' })
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-6xl flex-col gap-6 p-6 pt-2">
    <div class="flex flex-col">
      <h1 class="text-2xl font-semibold tracking-tight">Dashboard</h1>
      <p class="mt-0.5 text-sm text-muted-foreground">Pipeline health, reminders, and recent tasks</p>
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
      <Card v-else class="enter-rise relative overflow-hidden">
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
      <Card v-else class="enter-rise relative overflow-hidden" style="animation-delay: 60ms">
        <div class="absolute inset-x-0 top-0 h-1 bg-linear-to-r from-primary to-primary/60" />
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle class="text-sm font-medium">Total Leads</CardTitle>
          <div class="flex size-8 items-center justify-center rounded-lg bg-primary/10">
            <FolderOpen class="size-4 text-primary" />
          </div>
        </CardHeader>
        <CardContent>
          <div class="text-3xl font-bold tracking-tight tabular-nums">{{ leadsStore.total }}</div>
          <p class="mt-1 text-xs text-muted-foreground">All leads across pipelines</p>
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
      <Card v-else class="enter-rise relative overflow-hidden" style="animation-delay: 120ms">
        <div class="absolute inset-x-0 top-0 h-1 bg-linear-to-r from-primary to-primary/60" />
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle class="text-sm font-medium">Pending Reminders</CardTitle>
          <div class="flex size-8 items-center justify-center rounded-lg bg-primary/10">
            <Bell class="size-4 text-primary" />
          </div>
        </CardHeader>
        <CardContent>
          <div class="text-3xl font-bold tracking-tight tabular-nums">{{ remindersStore.openReminders.length }}</div>
          <p class="mt-1 text-xs text-muted-foreground">Awaiting action across leads</p>
        </CardContent>
      </Card>
    </div>

    <!-- Pipeline health -->
    <Card class="enter-rise" style="animation-delay: 160ms">
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
                  class="h-full rounded-full bg-primary/70 transition-[width] duration-500 ease-out"
                  :style="{ width: barsIn ? `${(row.count / stageMax) * 100}%` : '0%' }"
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
                class="h-full bg-primary transition-[width] duration-500 ease-out"
                :style="{ width: barsIn ? `${(openCount / totalCount) * 100}%` : '0%' }"
                title="Open"
              ></div>
              <div
                class="h-full bg-success transition-[width] duration-500 ease-out"
                :style="{ width: barsIn ? `${(wonCount / totalCount) * 100}%` : '0%' }"
                title="Won"
              ></div>
              <div
                class="h-full bg-destructive transition-[width] duration-500 ease-out"
                :style="{ width: barsIn ? `${(lostCount / totalCount) * 100}%` : '0%' }"
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
              class="flex items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-muted/50 cursor-pointer"
              @click="openLeadDrawer(r.lead_id)"
            >
              <div class="flex size-8 shrink-0 items-center justify-center rounded-full bg-muted">
                <component :is="iconFor(r.type)" class="size-3.5 text-muted-foreground" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm">
                  <span v-if="r.lead_display_name" class="font-medium">{{ r.lead_display_name }}</span>
                  <span v-if="r.lead_display_name && reminderTextOf(r)" class="text-muted-foreground"> · {{ reminderTextOf(r) }}</span>
                  <template v-else>{{ reminderTextOf(r) }}</template>
                </p>
                <p v-if="r.stage_name" class="mt-0.5 text-xs text-muted-foreground">{{ r.stage_name }}</p>
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
            <CardTitle>Recent tasks</CardTitle>
          </div>
          <Button variant="ghost" size="sm" class="text-muted-foreground" @click="goToActivities">
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
              <Skeleton class="h-3 w-16" />
            </div>
          </div>
          <div v-else-if="activitiesStore.recent.length === 0" class="flex flex-col items-center justify-center py-10 text-center">
            <Calendar class="size-10 text-muted-foreground/40 mb-3" />
            <p class="text-sm font-medium text-muted-foreground">No recent tasks</p>
            <p class="text-xs text-muted-foreground/60 mt-1">Tasks will appear here as you work leads</p>
          </div>
          <div v-else class="space-y-1">
            <div
              v-for="entry in activitiesStore.recent"
              :key="entry.id"
              class="flex items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-muted/50 cursor-pointer"
              @click="openLeadDrawer(entry.lead_id)"
            >
              <div class="flex size-8 shrink-0 items-center justify-center rounded-full bg-muted">
                <component :is="iconFor(entry.type)" class="size-3.5 text-muted-foreground" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm">
                  <span class="font-medium">{{ entry.type }}</span>
                  <span class="text-muted-foreground"> · {{ entry.lead_display_name }}</span>
                  <span v-if="entry.quick_reply_name" class="text-muted-foreground"> · {{ entry.quick_reply_name }}</span>
                </p>
                <p v-if="entry.description" class="mt-0.5 truncate text-xs text-muted-foreground">
                  {{ entry.description }}
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
</template>
