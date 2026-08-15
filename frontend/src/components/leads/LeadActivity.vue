<script setup lang="ts">
import { computed, onMounted, shallowRef, watch } from 'vue'
import { apiClient } from '@/composables/useApi'
import { useSettingsStore } from '@/stores/settings'
import { toast } from 'vue-sonner'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { MoreHorizontal, Trash2 } from '@lucide/vue'
import { reminderIcon, snoozeRemindAt } from '@/utils/reminders'
import { formatDateTime } from '@/utils/time'
import { Badge } from '@/components/ui/badge'
import TaskRow, { type ActivityRowActivity } from '@/components/leads/TaskRow.vue'
import { errorMessage } from '@/utils/errors'

interface Activity extends ActivityRowActivity {}

const props = defineProps<{ leadId: string }>()

const emit = defineEmits<{
  closeLost: []
}>()

const activities = shallowRef<Activity[]>([])
const loading = shallowRef(false)
const loadError = shallowRef('')

const settings = useSettingsStore()

onMounted(() => {
  if (settings.activityTypes.length === 0) settings.fetchTags()
})

// Watch the prop: the instance may be reused with a different lead id (the
// drawer swaps activityLead without closing), so refetch when it changes.
let fetchSeq = 0
watch(() => props.leadId, () => fetchActivities(), { immediate: true })

async function fetchActivities() {
  const seq = ++fetchSeq
  loading.value = true
  loadError.value = ''
  try {
    const res = await apiClient.get(`/api/leads/${props.leadId}/activities`)
    if (seq !== fetchSeq) return
    activities.value = res.data
  } catch (e) {
    if (seq !== fetchSeq) return
    activities.value = []
    loadError.value = errorMessage(e, 'Failed to load activities')
  } finally {
    if (seq === fetchSeq) loading.value = false
  }
}

function isTouchpoint(a: Activity): boolean {
  return !!a.is_done || !!a.occurred_at || !!a.responded_at
}

function isOverdue(a: Activity): boolean {
  if (isTouchpoint(a)) return false
  const at = a.scheduled_at || a.remind_at
  return !!at && new Date(at).getTime() < Date.now()
}

function isUpcoming(a: Activity): boolean {
  return !isTouchpoint(a) && !isOverdue(a)
}

// Upcoming sorted by soonest first; overdue sink to the top.
const openActivities = computed(() => {
  return activities.value
    .filter((a) => !isTouchpoint(a))
    .sort((x, y) => {
      const tx = new Date(x.scheduled_at || x.remind_at || x.created_at).getTime()
      const ty = new Date(y.scheduled_at || y.remind_at || y.created_at).getTime()
      return tx - ty
    })
})

const overdueActivities = computed(() => openActivities.value.filter(isOverdue))
const upcomingActivities = computed(() => openActivities.value.filter(isUpcoming))
const doneActivities = computed(() => activities.value.filter(isTouchpoint))

async function deleteActivity(id: string) {
  try {
    await apiClient.delete(`/api/leads/${props.leadId}/activities/${id}`)
    toast.success('Activity deleted')
    await fetchActivities()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to delete'))
  }
}

async function markDone(a: Activity) {
  try {
    await apiClient.patch(`/api/leads/${props.leadId}/activities/${a.id}`, {
      is_done: true,
    })
    toast.success('Marked as done')
    await fetchActivities()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to mark response'))
  }
}

async function snooze(a: Activity, minutes: number) {
  try {
    await apiClient.post(`/api/leads/${props.leadId}/reminders/${a.id}/snooze`, {
      remind_at: snoozeRemindAt(minutes),
    })
    toast.success('Reminder snoozed')
    await fetchActivities()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to snooze'))
  }
}

function typeLabel(type: string): string {
  return type || 'Task'
}

defineExpose({ fetchActivities })
</script>

<template>
  <div class="space-y-3">
    <div v-if="loading" class="space-y-2">
      <Skeleton v-for="i in 3" :key="i" class="h-16 w-full" />
    </div>

    <div v-else-if="loadError" class="rounded-lg border border-dashed p-6 text-center">
      <p class="text-sm text-destructive">{{ loadError }}</p>
    </div>

    <div v-else-if="activities.length === 0" class="rounded-lg border border-dashed p-6 text-center">
      <p class="text-sm text-muted-foreground">No tasks logged yet</p>
    </div>

    <template v-else>
      <section v-if="overdueActivities.length">
        <h3 class="mb-2 text-xs font-semibold uppercase tracking-wide text-destructive">Overdue</h3>
        <div class="space-y-2">
          <TaskRow
            v-for="a in overdueActivities"
            :key="a.id"
            :lead-id="props.leadId"
            :activity="a"
            overdue
            :quick-replies="settings.quickReplies"
            :activity-types="settings.activityTypes"
            @changed="fetchActivities"
            @mark-done="markDone(a)"
            @snooze="snooze(a, $event)"
            @delete="deleteActivity(a.id)"
            @close-lost="emit('closeLost')"
          />
        </div>
      </section>

      <section v-if="upcomingActivities.length">
        <h3 class="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Upcoming</h3>
        <div class="space-y-2">
          <TaskRow
            v-for="a in upcomingActivities"
            :key="a.id"
            :lead-id="props.leadId"
            :activity="a"
            :quick-replies="settings.quickReplies"
            :activity-types="settings.activityTypes"
            @changed="fetchActivities"
            @mark-done="markDone(a)"
            @snooze="snooze(a, $event)"
            @delete="deleteActivity(a.id)"
            @close-lost="emit('closeLost')"
          />
        </div>
      </section>

      <section v-if="doneActivities.length">
        <h3 class="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Past</h3>
        <div class="space-y-2">
          <Card v-for="a in doneActivities" :key="a.id" :class="a.is_cancelled ? 'opacity-60' : ''">
            <CardContent class="p-3">
              <div class="flex items-start justify-between gap-2">
                <div class="flex items-start gap-2 min-w-0">
                  <component :is="reminderIcon(a.type)" class="size-4 mt-0.5 text-muted-foreground shrink-0" />
                  <div class="min-w-0">
                    <div class="flex items-center gap-2 flex-wrap">
                      <span
                        class="text-xs font-medium"
                        :class="a.is_cancelled ? 'line-through decoration-muted-foreground/40' : ''"
                      >
                        {{ typeLabel(a.type) }}
                      </span>
                      <span v-if="a.quick_reply_name" class="text-xs px-1.5 py-0.5 rounded-full bg-secondary text-secondary-foreground">
                        {{ a.quick_reply_name }}
                      </span>
                      <Badge v-if="a.is_cancelled" variant="secondary" class="text-xs">Cancelled</Badge>
                      <Badge v-else-if="a.is_done" variant="secondary" class="text-xs">Done</Badge>
                    </div>
                    <p v-if="a.description" class="text-sm mt-0.5">{{ a.description }}</p>
                    <span
                      v-if="a.occurred_at || a.responded_at"
                      class="text-xs text-muted-foreground mt-1 block"
                    >
                      {{ a.occurred_at ? 'Happened' : 'Responded' }}: {{ formatDateTime(a.occurred_at || a.responded_at!) }} ·
                      {{ a.user_name || 'System' }}
                    </span>
                    <span v-else class="text-xs text-muted-foreground/70 mt-1 block">
                      {{ formatDateTime(a.created_at) }} · {{ a.user_name || 'System' }}
                    </span>
                  </div>
                </div>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <Button variant="ghost" size="icon-sm" class="size-8 shrink-0" aria-label="Activity actions">
                      <MoreHorizontal class="size-3.5" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem class="text-destructive cursor-pointer" @click="deleteActivity(a.id)">
                      <Trash2 class="size-3.5 mr-2" /> Delete
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </CardContent>
          </Card>
        </div>
      </section>
    </template>
  </div>
</template>
