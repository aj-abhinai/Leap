<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useLocalStorage } from '@vueuse/core'
import { useSettingsStore } from '@/stores/settings'
import { createLeadActivity, type ActivityFollowUpBody, type ActivityMutationBody } from '@/api/leads'
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
import { toLocalDateInput, toLocalTimeInput, mergeDateTime, allDayRange } from '@/utils/time'
import { errorMessage } from '@/utils/errors'

const props = defineProps<{ leadId: string }>()

const emit = defineEmits<{
  saved: []
  closeLost: []
}>()

const settings = useSettingsStore()

// Reka Select cannot carry an empty string as an item value, so "same type"
// and "system reminder" travel as sentinels and map back at save time.
const SAME_TYPE = '__same__'
const SYSTEM_REMIND = '__system__'

const activityType = ref('')
const description = ref('')
const quickReplyId = ref('')

// The follow-up plan for a `next` outcome: one date, an optional time (a
// date alone is the whole local day), and an optional different type.
const followDate = ref('')
const followTime = ref('')
const followType = ref(SAME_TYPE)
const remindChoice = ref(SYSTEM_REMIND)

const saving = ref(false)
const error = ref('')
const moreOptions = ref(false)

const REMIND_OPTIONS = [
  { value: '0', label: 'At task time' },
  { value: '15', label: '15 min before' },
  { value: '60', label: '1 hour before' },
  { value: '1440', label: '1 day before' },
]

const activityTypes = computed(() => settings.activityTypes)

// Quick-reply chips are a dedicated catalog (settings.quickReplies), separate
// from contact statuses. Each quick reply carries a behavior that decides
// the follow-up when tapped.
const chips = computed(() => settings.quickReplies)
const selectedChip = computed(() => chips.value.find((c) => c.id === quickReplyId.value))
const groupedChips = computed(() => groupQuickReplies(chips.value))
const selectedBehavior = computed(() => selectedChip.value?.behavior || 'log')
const showNextFields = computed(() => selectedBehavior.value === 'next')
const showCloseNotice = computed(() => selectedBehavior.value === 'close_lost')

// A second tap on the picked chip un-picks it. The entered date survives the
// mode change, so picking a chip never erases input.
function pickChip(id: string) {
  quickReplyId.value = quickReplyId.value === id ? '' : id
}

function pickNextPreset(preset: NextPreset) {
  const at = preset.at().toISOString()
  followDate.value = toLocalDateInput(at)
  followTime.value = toLocalTimeInput(at)
  remindChoice.value = SYSTEM_REMIND
}

// Highlights the preset that produced the current follow-up date/time.
const selectedPreset = computed(() =>
  findSelectedPreset(followDate.value, followTime.value, nextPresets),
)

// The follow-up type the picker resolves to: the attempt's type unless the
// user chose another.
const effectiveFollowType = computed(() => {
  const picked = followType.value === SAME_TYPE ? '' : followType.value
  return (picked || activityType.value).trim()
})

// INVALID marks input the form refuses to send: the payload is never built
// from a malformed date, and the user sees an error instead of a silent drop.
const INVALID = Symbol('invalid follow-up')

// makeFollowUp builds the follow-up payload from the entered date. No date
// means no follow-up (null). A date without a time is the whole local day:
// start, end, and the 09:00 remind travel together so the shape survives the
// round trip. A timed follow-up sends only the start; the reminder travels
// only as a relative override.
function makeFollowUp(): ActivityFollowUpBody | null | typeof INVALID {
  if (!followDate.value) return null
  if (followTime.value) {
    const start = mergeDateTime(followDate.value, followTime.value)
    if (!start) return INVALID
    const fu: ActivityFollowUpBody = { scheduled_at: start }
    if (remindChoice.value !== SYSTEM_REMIND) {
      const offset = Number(remindChoice.value)
      fu.remind_at = new Date(new Date(start).getTime() - offset * 60_000).toISOString()
    }
    return withType(fu)
  }
  const day = allDayRange(followDate.value)
  if (!day) return INVALID
  return withType({ scheduled_at: day.start, scheduled_end_at: day.end, remind_at: day.remind })
}

function withType(fu: ActivityFollowUpBody): ActivityFollowUpBody {
  const t = effectiveFollowType.value
  if (t && t !== activityType.value.trim()) fu.type = t
  return fu
}

// The preview states exactly what the save will do, before it happens.
const preview = computed(() => {
  if (showCloseNotice.value) return 'Will log the attempt and close this deal as lost.'
  if (!showNextFields.value) return 'Will log today\u2019s attempt.'
  if (!followDate.value) return 'No follow-up will be created.'
  const when = followTime.value
    ? new Date(mergeDateTime(followDate.value, followTime.value) || '').toLocaleString([], {
        weekday: 'short',
        month: 'short',
        day: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
      })
    : `${new Date(`${followDate.value}T00:00`).toLocaleDateString([], {
        weekday: 'short',
        month: 'short',
        day: 'numeric',
      })} (all day)`
  const remind = followTime.value
    ? remindChoice.value === SYSTEM_REMIND
      ? 'system reminder'
      : REMIND_OPTIONS.find((o) => o.value === remindChoice.value)?.label.toLowerCase() || 'system reminder'
    : 'reminder 9:00 AM'
  const what = effectiveFollowType.value || 'Task'
  return `Will log today\u2019s ${activityType.value || 'attempt'}, and create: ${what} \u00b7 ${when} \u00b7 ${remind}`
})

// Pre-fill the type with the last successfully logged one (per browser) so a
// follow-up log usually needs no type selection.
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

function clearForm() {
  activityType.value = ''
  applyLastType()
  description.value = ''
  quickReplyId.value = ''
  followDate.value = ''
  followTime.value = ''
  followType.value = SAME_TYPE
  remindChoice.value = SYSTEM_REMIND
}

function applyLastType() {
  const last = lastActivityType.value
  if (last && settings.activityTypes.some((t) => t.name === last)) {
    activityType.value = last
  }
}

// One action: the attempt is logged with the outcome, and a `next` outcome
// with a date also creates the follow-up task. Nothing is discarded — a
// malformed date stops the save with an error.
async function handleSave() {
  error.value = ''
  if (!activityType.value) {
    error.value = 'Type is required'
    return
  }
  const fu = showNextFields.value ? makeFollowUp() : null
  if (fu === INVALID) {
    error.value = 'Enter a valid date'
    return
  }

  const payload: ActivityMutationBody = {
    type: activityType.value,
    description: description.value.trim(),
    quick_reply_id: quickReplyId.value || null,
    is_done: true,
  }
  if (fu) payload.follow_up = fu
  const wasCloseLost = showCloseNotice.value

  saving.value = true
  try {
    await createLeadActivity(props.leadId, payload)
    toast.success(fu ? 'Attempt logged, follow-up created' : 'Attempt logged')
    lastActivityType.value = activityType.value
    clearForm()
    // A close_lost save emits only closeLost: the drawer's handleCloseLost
    // already reloads the board and closes, so emitting saved as well would
    // double-fire both. Plain saves emit saved.
    if (wasCloseLost) {
      emit('closeLost')
    } else {
      emit('saved')
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
      <h4 class="text-sm font-medium">What happened?</h4>

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
           chips flowing in a row beneath it. A second tap un-picks a chip. -->
      <div v-if="chips.length" class="space-y-2">
        <Label class="text-xs">Outcome</Label>
        <div class="space-y-2.5">
          <div v-for="g in groupedChips" :key="g.group" class="space-y-1">
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
                @click="pickChip(c.id)"
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
        <div class="grid grid-cols-2 gap-2">
          <div class="space-y-1.5">
            <Label class="text-xs">Follow-up date</Label>
            <Input v-model="followDate" type="date" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">Time (optional)</Label>
            <Input v-model="followTime" type="time" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-2">
          <div class="space-y-1.5">
            <Label class="text-xs">Follow-up type</Label>
            <Select v-model="followType">
              <SelectTrigger size="sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem :value="SAME_TYPE">Same as above</SelectItem>
                <SelectItem v-for="t in activityTypes" :key="t.id" :value="t.name">
                  {{ t.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">Reminder</Label>
            <Select v-if="followTime" v-model="remindChoice">
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
            <p v-else class="px-1 py-2 text-xs text-muted-foreground">9:00 AM on the day</p>
          </div>
        </div>
      </template>
    </div>

    <!-- Action row: the preview states what Save will do; it sits right under
         the fields it acts on. -->
    <div class="flex items-center justify-between gap-2 border-t px-3 py-2.5">
      <p v-if="error" class="text-xs text-destructive">{{ error }}</p>
      <p v-else class="text-xs text-muted-foreground">{{ preview }}</p>
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
      </div>
    </div>
  </div>
</template>
