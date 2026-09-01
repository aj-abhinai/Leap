<script setup lang="ts">
import { computed, onMounted, shallowRef, watch } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import { useRemindersStore } from '@/stores/reminders'
import { listLeadActivities, deleteLeadActivity, updateLeadActivity, type LeadActivity } from '@/api/leads'
import { toast } from 'vue-sonner'
import { Card, CardContent } from '@/components/ui/card'
import PageState from '@/components/PageState.vue'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { MoreHorizontal, Trash2 } from '@lucide/vue'
import { reminderIcon, snoozeRemindAt } from '@/utils/reminders'
import { formatDateTime } from '@/utils/time'
import { typeLabel, isOverdue as isActivityOverdue } from '@/utils/activity'
import { Badge } from '@/components/ui/badge'
import TaskRow from '@/components/leads/TaskRow.vue'
import { errorMessage } from '@/utils/errors'

const props = defineProps<{ leadId: string }>()

const emit = defineEmits<{
  closeLost: []
}>()

const activities = shallowRef<LeadActivity[]>([])
const loading = shallowRef(false)
const loadError = shallowRef('')

const settings = useSettingsStore()
const remindersStore = useRemindersStore()

onMounted(() => {
  if (settings.activityTypes.length === 0) settings.fetchTags()
})

// The drawer's first screenful shows the newest touchpoints without scrolling;
// "Show all" expands the section to the full history. Declared above the
// leadId watcher — its callback runs synchronously (immediate: true) and
// resets this ref.
const RECENT_LIMIT = 4
const historyExpanded = shallowRef(false)

// Watch the prop: the instance may be reused with a different lead id (the
// drawer swaps activityLead without closing), so refetch when it changes.
let fetchSeq = 0
watch(
  () => props.leadId,
  () => {
    historyExpanded.value = false
    fetchActivities()
  },
  { immediate: true },
)

async function fetchActivities() {
  const seq = ++fetchSeq
  loading.value = true
  loadError.value = ''
  try {
    const res = await listLeadActivities(props.leadId)
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

function isTouchpoint(a: LeadActivity): boolean {
  return !!a.is_done || !!a.occurred_at || !!a.responded_at
}

// Overdue uses the shared due-boundary rule (ADR 004) so the timeline and the
// Activities page never disagree about what slipped.
function isOverdue(a: LeadActivity): boolean {
  return isActivityOverdue(a)
}

function isUpcoming(a: LeadActivity): boolean {
  return !isTouchpoint(a) && !isOverdue(a)
}

// Upcoming sorted by soonest first; overdue sink to the top.
const openActivities = computed(() => {
  return activities.value
    .filter((a) => !isTouchpoint(a))
    .sort((x, y) => {
      const tx = new Date(x.scheduled_end_at || x.scheduled_at || x.remind_at || x.created_at).getTime()
      const ty = new Date(y.scheduled_end_at || y.scheduled_at || y.remind_at || y.created_at).getTime()
      return tx - ty
    })
})

const overdueActivities = computed(() => openActivities.value.filter(isOverdue))
const upcomingActivities = computed(() => openActivities.value.filter(isUpcoming))
const doneActivities = computed(() => activities.value.filter(isTouchpoint))

const visibleDone = computed(() =>
  historyExpanded.value ? doneActivities.value : doneActivities.value.slice(0, RECENT_LIMIT),
)

async function deleteActivity(id: string) {
  try {
    await deleteLeadActivity(props.leadId, id)
    toast.success('Activity deleted')
    await fetchActivities()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to delete'))
  }
}

async function markDone(a: LeadActivity) {
  try {
    await updateLeadActivity(props.leadId, a.id, { is_done: true })
    toast.success('Marked as done')
    await fetchActivities()
    // Completing a task whose quick reply closes lost moves the lead server
    // side; signal the drawer so it refreshes the kanban and closes. The tag
    // catalog may still be loading, so fetch it before the behavior lookup.
    if (a.quick_reply_id) {
      if (settings.quickReplies.length === 0) await settings.fetchTags()
      const qr = settings.quickReplies.find((s) => s.id === a.quick_reply_id)
      if (qr?.behavior === 'close_lost') {
        emit('closeLost')
      }
    }
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to mark response'))
  }
}

async function snooze(a: LeadActivity, minutes: number) {
  try {
    await remindersStore.snoozeReminder(props.leadId, a.id, snoozeRemindAt(minutes))
    toast.success('Reminder snoozed')
    await fetchActivities()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to snooze'))
  }
}

defineExpose({ fetchActivities })
</script>

<template>
  <div class="space-y-3">
    <PageState
      :loading="loading"
      :error="loadError"
      :empty="activities.length === 0"
      empty-title="No tasks logged yet"
      :skeleton-count="3"
      skeleton-class="h-16 w-full"
      @retry="fetchActivities"
    >
      <section v-if="doneActivities.length">
        <div class="mb-2 flex items-center justify-between">
          <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">History</h3>
          <Button
            v-if="doneActivities.length > RECENT_LIMIT"
            variant="ghost"
            size="sm"
            class="h-6 px-2 text-xs text-muted-foreground"
            @click="historyExpanded = !historyExpanded"
          >
            {{ historyExpanded ? 'Show less' : `Show all ${doneActivities.length}` }}
          </Button>
        </div>
        <div class="space-y-2">
          <Card v-for="a in visibleDone" :key="a.id" :class="a.is_cancelled ? 'opacity-60' : ''">
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
    </PageState>
  </div>
</template>
