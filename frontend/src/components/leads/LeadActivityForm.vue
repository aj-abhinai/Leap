<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { apiClient } from '@/composables/useApi'
import { useSettingsStore } from '@/stores/settings'
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
const scheduledDate = ref('')
const scheduledTime = ref('')
const remindDate = ref('')
const remindTime = ref('')
const nextDate = ref('')
const nextTime = ref('')
const saving = ref(false)
const error = ref('')

const activityTypes = computed(() => settings.activityTypes)

// Quick-reply chips are a dedicated catalog (settings.quickReplies), separate
// from contact statuses (ADR 020). Each quick reply carries a behavior that
// decides the follow-up when tapped.
const chips = computed(() => settings.quickReplies)

const selectedChip = computed(() => chips.value.find((c) => c.id === quickReplyId.value))

const groupedChips = computed(() => {
  const groups = new Map<string, typeof chips.value>()
  for (const chip of chips.value) {
    const key = chip.group_name || 'Quick reply'
    const bucket = groups.get(key) ?? []
    bucket.push(chip)
    groups.set(key, bucket)
  }
  return [...groups.entries()].map(([group, items]) => ({
    group,
    items: items.slice().sort((a, b) => a.sort_order - b.sort_order),
  }))
})

const selectedBehavior = computed(() => selectedChip.value?.behavior || 'log')

const showNextFields = computed(() => selectedBehavior.value === 'next')
const showCloseNotice = computed(() => selectedBehavior.value === 'close_lost')
const showScheduleFields = computed(() => !showNextFields.value)

onMounted(() => {
  if (settings.activityTypes.length === 0) settings.fetchTags()
})

// When a schedule is entered and the reminder is untouched, default the
// reminder to the scheduled time — a reminder is the nudge for a task.
watch([scheduledDate, scheduledTime], () => {
  if (!scheduledDate.value || !scheduledTime.value) return
  if (!remindDate.value && !remindTime.value) {
    remindDate.value = scheduledDate.value
    remindTime.value = scheduledTime.value
  }
})

function setChip(id: string) {
  quickReplyId.value = id
  // Switching behaviors resets stale next/schedule state.
  nextDate.value = ''
  nextTime.value = ''
}

function clearForm() {
  activityType.value = ''
  description.value = ''
  quickReplyId.value = ''
  scheduledDate.value = ''
  scheduledTime.value = ''
  remindDate.value = ''
  remindTime.value = ''
  nextDate.value = ''
  nextTime.value = ''
}

function mergeDateTime(date: string, time: string): string | null {
  if (!date || !time) return null
  return new Date(`${date}T${time}`).toISOString()
}

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

  // Capture the behavior before clearForm() resets quickReplyId — showCloseNotice
  // derives from it and would recompute to false otherwise.
  const wasCloseLost = showCloseNotice.value

  if (showNextFields.value) {
    const next = mergeDateTime(nextDate.value, nextTime.value)
    if (!next) {
      error.value = 'Pick the next date and time'
      return
    }
    // "Log attempt + next": the current activity completes with the quick reply and
    // the next occurrence of the same type is created for this time.
    payload.reschedule_at = next
  } else {
    payload.scheduled_at = mergeDateTime(scheduledDate.value, scheduledTime.value)
    payload.remind_at = mergeDateTime(remindDate.value, remindTime.value)
    // A close_lost quick reply completes the activity immediately so it is logged
    // as the closing touchpoint, not cancelled by the subsequent stage move.
    if (wasCloseLost) {
      payload.is_done = true
    }
  }

  saving.value = true
  try {
    await apiClient.post(`/api/leads/${props.leadId}/activities`, payload)
    toast.success(showNextFields.value ? 'Attempt logged, next task created' : 'Activity logged')
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
  <div class="space-y-3 rounded-lg border p-3">
    <h4 class="text-sm font-medium">Log Task</h4>
    <div class="space-y-2">
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

    <div v-if="chips.length" class="space-y-2">
      <Label class="text-xs">What happened?</Label>
      <div class="space-y-3">
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
              class="h-7 px-2.5 text-xs"
              :class="quickReplyId === c.id ? 'border-primary text-primary' : ''"
              @click="setChip(c.id)"
            >
              {{ c.name }}
            </Button>
          </div>
        </div>
      </div>
    </div>

    <div class="space-y-2">
      <Label class="text-xs">Notes (optional)</Label>
      <Textarea v-model="description" placeholder="Optional notes…" class="min-h-16" />
    </div>

    <template v-if="showCloseNotice">
      <p class="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
        This quick reply marks the lead as Closed Lost — open tasks will be cancelled.
      </p>
    </template>

    <template v-else-if="showNextFields">
      <div class="grid grid-cols-2 gap-2">
        <div class="space-y-1.5">
          <Label class="text-xs">Next date</Label>
          <Input v-model="nextDate" type="date" />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">Next time</Label>
          <Input v-model="nextTime" type="time" />
        </div>
      </div>
      <p class="text-xs text-muted-foreground">
        The current attempt is logged as done and a new <strong>{{ activityType || 'task' }}</strong> is scheduled.
      </p>
    </template>

    <template v-else>
      <div class="grid grid-cols-2 gap-2">
        <div class="space-y-1.5">
          <Label class="text-xs">Scheduled date</Label>
          <Input v-model="scheduledDate" type="date" />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">Scheduled time</Label>
          <Input v-model="scheduledTime" type="time" />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-2">
        <div class="space-y-1.5">
          <Label class="text-xs">Remind date</Label>
          <Input v-model="remindDate" type="date" />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">Remind time</Label>
          <Input v-model="remindTime" type="time" />
        </div>
      </div>
    </template>

    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
    <div class="flex justify-end">
      <Button size="sm" :disabled="saving" @click="handleSave">
        {{ saving ? 'Saving...' : 'Save' }}
      </Button>
    </div>
  </div>
</template>