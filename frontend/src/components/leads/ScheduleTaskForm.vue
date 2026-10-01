<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import { createLeadActivity, type ActivityMutationBody } from '@/api/leads'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { CalendarPlus } from '@lucide/vue'
import { mergeDateTime, allDayRange } from '@/utils/time'
import { errorMessage } from '@/utils/errors'

// Schedule task creates an open task with nothing behind it: the first task
// on a fresh enquiry, or planned work. It never writes history — that is the
// What-happened form's job.
const props = defineProps<{ leadId: string }>()

const emit = defineEmits<{ saved: [] }>()

const settings = useSettingsStore()

// Reka Select cannot carry an empty string as an item value, so the system
// reminder travels as a sentinel.
const SYSTEM_REMIND = '__system__'

const taskType = ref('')
const schedDate = ref('')
const schedTime = ref('')
const remindChoice = ref(SYSTEM_REMIND)

const saving = ref(false)
const error = ref('')

const REMIND_OPTIONS = [
  { value: '0', label: 'At task time' },
  { value: '15', label: '15 min before' },
  { value: '60', label: '1 hour before' },
  { value: '1440', label: '1 day before' },
]

const activityTypes = computed(() => settings.activityTypes)

// The preview states what the save will do, before it happens.
const preview = computed(() => {
  if (!schedDate.value) return ''
  if (!schedTime.value) {
    return `Will create: ${taskType.value || 'Task'} \u00b7 all day \u00b7 reminder 9:00 AM`
  }
  const when = new Date(mergeDateTime(schedDate.value, schedTime.value) || '').toLocaleString([], {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
  const remind = remindChoice.value === SYSTEM_REMIND
    ? 'system reminder'
    : REMIND_OPTIONS.find((o) => o.value === remindChoice.value)?.label.toLowerCase() || 'system reminder'
  return `Will create: ${taskType.value || 'Task'} \u00b7 ${when} \u00b7 ${remind}`
})

// The drawer swaps leads without remounting this form; reset on lead change.
watch(
  () => props.leadId,
  () => {
    clearForm()
  },
)

function clearForm() {
  taskType.value = ''
  schedDate.value = ''
  schedTime.value = ''
  remindChoice.value = SYSTEM_REMIND
  error.value = ''
}

// One date, optional time: a date without a time is the whole local day
// (start, end, and the 09:00 remind travel together). A malformed date stops
// the save with an error — input is never discarded.
async function handleSave() {
  error.value = ''
  if (!taskType.value) {
    error.value = 'Type is required'
    return
  }
  if (!schedDate.value) {
    error.value = 'Date is required'
    return
  }

  let payload: ActivityMutationBody
  if (!schedTime.value) {
    const day = allDayRange(schedDate.value)
    if (!day) {
      error.value = 'Enter a valid date'
      return
    }
    payload = {
      type: taskType.value,
      scheduled_at: day.start,
      scheduled_end_at: day.end,
      remind_at: day.remind,
    }
  } else {
    const start = mergeDateTime(schedDate.value, schedTime.value)
    if (!start) {
      error.value = 'Enter a valid date'
      return
    }
    payload = { type: taskType.value, scheduled_at: start }
    if (remindChoice.value !== SYSTEM_REMIND) {
      const offset = Number(remindChoice.value)
      payload.remind_at = new Date(new Date(start).getTime() - offset * 60_000).toISOString()
    }
  }

  saving.value = true
  try {
    await createLeadActivity(props.leadId, payload)
    toast.success('Task scheduled')
    clearForm()
    emit('saved')
  } catch (e) {
    error.value = errorMessage(e, 'Failed to schedule')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="rounded-lg border">
    <div class="space-y-2.5 p-3">
      <h4 class="flex items-center gap-1.5 text-sm font-medium">
        <CalendarPlus class="size-3.5 text-muted-foreground" /> Schedule task
      </h4>

      <div class="space-y-1.5">
        <Label class="text-xs">Type</Label>
        <Select v-model="taskType">
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

      <div class="grid grid-cols-2 gap-2">
        <div class="space-y-1.5">
          <Label class="text-xs">Date</Label>
          <Input v-model="schedDate" type="date" />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">Time (optional)</Label>
          <Input v-model="schedTime" type="time" />
        </div>
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Reminder</Label>
        <Select v-if="schedTime" v-model="remindChoice">
          <SelectTrigger size="sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem :value="SYSTEM_REMIND">System default</SelectItem>
            <SelectItem v-for="o in REMIND_OPTIONS" :key="o.value" :value="o.value">
              {{ o.label }}
            </SelectItem>
          </SelectContent>
        </Select>
        <p v-else class="text-xs text-muted-foreground">9:00 AM on the day</p>
      </div>
    </div>

    <div class="flex items-center justify-between gap-2 border-t px-3 py-2.5">
      <p v-if="error" class="text-xs text-destructive">{{ error }}</p>
      <p v-else class="text-xs text-muted-foreground">{{ preview || 'A date alone is the whole day' }}</p>
      <Button size="sm" :disabled="saving" @click="handleSave()">
        {{ saving ? 'Saving...' : 'Schedule' }}
      </Button>
    </div>
  </div>
</template>
