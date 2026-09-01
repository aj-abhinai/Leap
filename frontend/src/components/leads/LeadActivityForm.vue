<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useLocalStorage } from '@vueuse/core'
import { useSettingsStore } from '@/stores/settings'
import { createLeadActivity } from '@/api/leads'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { ChevronDown, ChevronUp, Check } from '@lucide/vue'
import { nextPresets, groupQuickReplies, findSelectedPreset, type NextPreset } from '@/utils/reminders'
import { toLocalDateInput, toLocalTimeInput, mergeDateTime } from '@/utils/time'
import { errorMessage } from '@/utils/errors'

const props = defineProps<{ leadId: string }>()

const emit = defineEmits<{
  saved: []
  closeLost: []
}>()

const settings = useSettingsStore()

const activityType = ref('')
const description = ref('')
const quickReplyId = ref('')

// Schedule fields are date+time pairs so a "time without a date" cannot
// exist; clear()/payload helpers keep the reset and submit paths in one place.
interface ScheduleSlot {
  date: string
  time: string
}
const schedule = reactive<{
  start: ScheduleSlot
  end: ScheduleSlot
  remind: ScheduleSlot
  next: ScheduleSlot
}>({
  start: { date: '', time: '' },
  end: { date: '', time: '' },
  remind: { date: '', time: '' },
  next: { date: '', time: '' },
})

function clearSchedule() {
  for (const slot of [schedule.start, schedule.end, schedule.remind, schedule.next]) {
    slot.date = ''
    slot.time = ''
  }
}

function slotPayload(slot: ScheduleSlot): string | undefined {
  return mergeDateTime(slot.date, slot.time) || undefined
}

const saving = ref(false)
const error = ref('')

const activityTypes = computed(() => settings.activityTypes)

// Quick-reply chips are a dedicated catalog (settings.quickReplies), separate
// from contact statuses. Each quick reply carries a behavior that
// decides the follow-up when tapped.
const chips = computed(() => settings.quickReplies)

const selectedChip = computed(() => chips.value.find((c) => c.id === quickReplyId.value))

const groupedChips = computed(() => groupQuickReplies(chips.value))

const selectedBehavior = computed(() => selectedChip.value?.behavior || 'log')

const showNextFields = computed(() => selectedBehavior.value === 'next')
const showCloseNotice = computed(() => selectedBehavior.value === 'close_lost')
const showMoreScheduleFields = computed(() => !showNextFields.value && !showCloseNotice.value)
const hasNextTime = computed(() => !!mergeDateTime(schedule.next.date, schedule.next.time))
const moreOptions = ref(false)

onMounted(() => {
  if (settings.activityTypes.length === 0) settings.fetchTags()
})

// Pre-fill the type with the last successfully logged one (per browser) so a
// follow-up log usually needs no type selection. clearForm() re-applies it
// after saves; this watcher covers the async tags load on first mount.
const lastActivityType = useLocalStorage('crm:lastActivityType', '')

watch(
  () => settings.activityTypes,
  (types) => {
    if (activityType.value) return
    const last = lastActivityType.value
    if (last && types.some((t) => t.name === last)) {
      activityType.value = last
    }
  },
  { immediate: true, deep: true },
)

// The drawer swaps leads without remounting this form; reset on lead change
// so state never leaks between leads.
watch(
  () => props.leadId,
  () => {
    clearForm()
    moreOptions.value = false
  },
)

// When a schedule is entered and the reminder is untouched, default the
// reminder to the scheduled time — a reminder is the nudge for a task.
watch(
  () => [schedule.start.date, schedule.start.time],
  () => {
    if (!schedule.start.date || !schedule.start.time) return
    if (!schedule.remind.date && !schedule.remind.time) {
      schedule.remind.date = schedule.start.date
      schedule.remind.time = schedule.start.time
    }
  },
)

function setChip(id: string) {
  quickReplyId.value = id
  // Switching behaviors resets stale next/schedule state so a schedule entered
  // under one behavior is never silently submitted under another.
  clearSchedule()
}

function applyLastType() {
  const last = lastActivityType.value
  if (last && settings.activityTypes.some((t) => t.name === last)) {
    activityType.value = last
  }
}

function clearForm() {
  activityType.value = ''
  description.value = ''
  quickReplyId.value = ''
  clearSchedule()
  // A follow-up log right after a save should still skip the type pick.
  applyLastType()
}

function pickNextPreset(preset: NextPreset) {
  const at = preset.at().toISOString()
  schedule.next.date = toLocalDateInput(at)
  schedule.next.time = toLocalTimeInput(at)
}

// Highlights the preset that produced the current next date/time so a tap has
// visible feedback (the date/time inputs themselves live under More options).
const selectedPreset = computed(() =>
  findSelectedPreset(schedule.next.date, schedule.next.time, nextPresets),
)

function formatNextAt(): string {
  const next = mergeDateTime(schedule.next.date, schedule.next.time)
  if (!next) return ''
  return new Date(next).toLocaleString([], {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

// One action: if a next time is picked, the attempt is logged and the next
// task is created; without a time, the attempt is logged on its own.
async function handleSave() {
  error.value = ''
  if (!activityType.value) {
    error.value = 'Type is required'
    return
  }

  let payload: Record<string, unknown> = {
    type: activityType.value,
    description: description.value.trim(),
    quick_reply_id: quickReplyId.value || null,
  }

  // Capture before clearForm() resets the state these derive from.
  const wasCloseLost = showCloseNotice.value
  const hasNext = showNextFields.value && !!mergeDateTime(schedule.next.date, schedule.next.time)

  if (showNextFields.value) {
    const next = slotPayload(schedule.next)
    if (next) {
      // "Log attempt + next": the current activity completes with the quick reply
      // and the next occurrence of the same type is created for this time.
      payload.reschedule_at = next
    } else {
      // No schedule picked: log the attempt as a completed touchpoint only.
      payload.is_done = true
    }
  } else {
    payload.scheduled_at = slotPayload(schedule.start)
    payload.scheduled_end_at = slotPayload(schedule.end)
    payload.remind_at = slotPayload(schedule.remind)
    // A close_lost quick reply completes the activity immediately so it is logged
    // as the closing touchpoint, not cancelled by the subsequent stage move.
    if (wasCloseLost) {
      payload.is_done = true
    }
  }

  saving.value = true
  try {
    await createLeadActivity(props.leadId, payload)
    toast.success(hasNext ? 'Attempt logged, next task created' : showNextFields.value ? 'Attempt logged' : 'Activity logged')
    lastActivityType.value = activityType.value
    clearForm()
    emit('saved')
    if (wasCloseLost) {
      emit('closeLost')
    }
  } catch (e) {
    error.value = errorMessage(e, 'Failed to save')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="rounded-lg border">
    <div class="space-y-2.5 p-3">
      <h4 class="text-sm font-medium">Log Task</h4>

      <div class="space-y-1.5">
        <Label class="text-xs">Type</Label>
        <Select v-model="activityType">
          <SelectTrigger>
            <SelectValue placeholder="Select type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="t in activityTypes" :key="t.id" :value="t.name">
              {{ t.name }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>

      <!-- Grouped quick replies: each group is a block — label line, then its
           chips flowing in a row beneath it. -->
      <div v-if="chips.length" class="space-y-2">
        <Label class="text-xs">What happened?</Label>
        <div class="space-y-2.5">
          <div v-for="g in groupedChips" :key="g.group">
            <p class="mb-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
              {{ g.group }}
            </p>
            <div class="flex flex-wrap gap-1.5">
              <Button
                v-for="c in g.items"
                :key="c.id"
                size="sm"
                variant="outline"
                class="h-7 gap-1 px-2.5 text-xs"
                :class="
                  quickReplyId === c.id
                    ? 'border-primary bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground'
                    : ''
                "
                @click="setChip(c.id)"
              >
                <Check v-if="quickReplyId === c.id" class="size-3" />
                {{ c.name }}
              </Button>
            </div>
          </div>
        </div>
      </div>

      <template v-if="showCloseNotice">
        <p class="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
          This quick reply marks the lead as Closed Lost — open tasks will be cancelled.
        </p>
      </template>

      <template v-else-if="showNextFields">
        <div class="space-y-1.5">
          <div class="flex items-baseline justify-between gap-2">
            <Label class="text-xs">Quick schedule</Label>
            <span v-if="hasNextTime" class="text-xs tabular-nums text-muted-foreground">
              {{ formatNextAt() }}
            </span>
          </div>
          <div class="flex flex-wrap gap-1.5">
            <Button
              v-for="p in nextPresets"
              :key="p.label"
              size="sm"
              variant="outline"
              class="h-7 gap-1 px-2.5 text-xs"
              :class="
                selectedPreset === p.label
                  ? 'border-primary bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground'
                  : ''
              "
              @click="pickNextPreset(p)"
            >
              <Check v-if="selectedPreset === p.label" class="size-3" />
              {{ p.label }}
            </Button>
          </div>
        </div>
      </template>
    </div>

    <!-- Action row: sits right under the fields it acts on. -->
    <div class="flex items-center justify-between gap-2 border-t px-3 py-2.5">
      <p v-if="error" class="text-xs text-destructive">{{ error }}</p>
      <p v-else-if="showNextFields" class="text-xs text-muted-foreground">
        {{ hasNextTime ? 'Also schedules the next task' : 'No next task will be created' }}
      </p>
      <span v-else />
      <Button size="sm" :disabled="saving" @click="handleSave()">
        {{ saving ? 'Saving...' : 'Save' }}
      </Button>
    </div>

    <!-- More options: last band; expands downward without pushing Save away. -->
    <div class="border-t">
      <Button
        variant="ghost"
        size="sm"
        class="flex h-8 w-full items-center justify-between rounded-none px-3 text-xs text-muted-foreground hover:text-foreground"
        @click="moreOptions = !moreOptions"
      >
        <span class="flex items-center gap-1.5">
          More options
          <span v-if="description.trim()" class="size-1.5 rounded-full bg-primary" />
        </span>
        <component :is="moreOptions ? ChevronUp : ChevronDown" class="size-3.5" />
      </Button>
      <div v-if="moreOptions" class="space-y-3 border-t p-3">
        <div class="space-y-2">
          <Label class="text-xs">Notes (optional)</Label>
          <Textarea v-model="description" placeholder="Optional notes…" class="min-h-16" />
        </div>
        <template v-if="showNextFields">
          <p class="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Another time</p>
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1.5">
              <Label class="text-xs">Next date</Label>
              <Input v-model="schedule.next.date" type="date" />
            </div>
            <div class="space-y-1.5">
              <Label class="text-xs">Next time</Label>
              <Input v-model="schedule.next.time" type="time" />
            </div>
          </div>
        </template>
        <template v-else-if="showMoreScheduleFields">
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1.5">
              <Label class="text-xs">Scheduled date</Label>
              <Input v-model="schedule.start.date" type="date" />
            </div>
            <div class="space-y-1.5">
              <Label class="text-xs">Scheduled time</Label>
              <Input v-model="schedule.start.time" type="time" />
            </div>
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1.5">
              <Label class="text-xs" for="until-date">Until date (optional)</Label>
              <Input id="until-date" v-model="schedule.end.date" type="date" />
            </div>
            <div class="space-y-1.5">
              <Label class="text-xs" for="until-time">Until time</Label>
              <Input id="until-time" v-model="schedule.end.time" type="time" />
            </div>
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1.5">
              <Label class="text-xs">Remind date</Label>
              <Input v-model="schedule.remind.date" type="date" />
            </div>
            <div class="space-y-1.5">
              <Label class="text-xs">Remind time</Label>
              <Input v-model="schedule.remind.time" type="time" />
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>