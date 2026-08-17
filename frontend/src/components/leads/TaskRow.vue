<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
  DropdownMenuSub, DropdownMenuSubContent, DropdownMenuSubTrigger,
} from '@/components/ui/dropdown-menu'
import {
  MoreHorizontal, Trash2, CheckCircle2, Pencil, AlarmClockPlus, CalendarClock, Check,
} from '@lucide/vue'
import { nextPresets, reminderIcon, snoozePresets, type NextPreset } from '@/utils/reminders'
import { formatDateTime, toLocalDateInput, toLocalTimeInput } from '@/utils/time'
import { Badge } from '@/components/ui/badge'
import { errorMessage } from '@/utils/errors'

export interface ActivityRowActivity {
  id: string
  lead_id: string
  type: string
  description: string
  quick_reply_name?: string
  scheduled_at?: string
  remind_at?: string
  occurred_at?: string
  responded_at?: string
  is_done: boolean
  is_cancelled: boolean
  is_reminded: boolean
  user_name?: string
  created_at: string
}

export interface QuickReplyChip {
  id: string
  name: string
  group_name?: string
  sort_order: number
  behavior: 'log' | 'next' | 'close_lost'
}

export interface ActivityTypeOption {
  id: string
  name: string
}

const props = defineProps<{
  leadId: string
  activity: ActivityRowActivity
  overdue?: boolean
  quickReplies: QuickReplyChip[]
  activityTypes: ActivityTypeOption[]
}>()

const emit = defineEmits<{
  changed: []
  markDone: []
  snooze: [minutes: number]
  delete: []
  closeLost: []
}>()

// --- local edit state ---
const editing = shallowRef(false)
const editType = shallowRef('')
const editDescription = shallowRef('')
const editScheduledDate = shallowRef('')
const editScheduledTime = shallowRef('')
const editRemindDate = shallowRef('')
const editRemindTime = shallowRef('')
const savingEdit = shallowRef(false)

function startEdit() {
  editing.value = true
  editType.value = props.activity.type
  editDescription.value = props.activity.description
  editScheduledDate.value = props.activity.scheduled_at ? toLocalDateInput(props.activity.scheduled_at) : ''
  editScheduledTime.value = props.activity.scheduled_at ? toLocalTimeInput(props.activity.scheduled_at) : ''
  editRemindDate.value = props.activity.remind_at ? toLocalDateInput(props.activity.remind_at) : ''
  editRemindTime.value = props.activity.remind_at ? toLocalTimeInput(props.activity.remind_at) : ''
}

function cancelEdit() {
  editing.value = false
}

function mergeDateTime(date: string, time: string): string | null {
  if (!date || !time) return null
  return new Date(`${date}T${time}`).toISOString()
}

async function saveEdit() {
  if (!editType.value) {
    toast.error('Type is required')
    return
  }
  savingEdit.value = true
  try {
    await apiClient.patch(`/api/leads/${props.leadId}/activities/${props.activity.id}`, {
      type: editType.value,
      description: editDescription.value.trim(),
      scheduled_at: mergeDateTime(editScheduledDate.value, editScheduledTime.value),
      remind_at: mergeDateTime(editRemindDate.value, editRemindTime.value),
    })
    toast.success('Activity updated')
    cancelEdit()
    emit('changed')
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to update'))
  } finally {
    savingEdit.value = false
  }
}

// --- local reschedule state: log attempt + create next task ---
const rescheduling = shallowRef(false)
const rescheduleDate = shallowRef('')
const rescheduleTime = shallowRef('')
const rescheduleQuickReply = shallowRef('')
const savingReschedule = shallowRef(false)

// Group the quick-reply chips into the ordered palette; a picked chip's
// behavior decides the follow-up (log only / schedule next / close lost).
const groupedQuickReplies = computed(() => {
  const groups = new Map<string, QuickReplyChip[]>()
  for (const s of props.quickReplies) {
    const key = s.group_name || 'Quick reply'
    const bucket = groups.get(key) ?? []
    bucket.push(s)
    groups.set(key, bucket)
  }
  return [...groups.entries()].map(([group, items]) => ({
    group,
    items: items.slice().sort((a, b) => a.sort_order - b.sort_order),
  }))
})

const selectedRescheduleChip = computed(() =>
  props.quickReplies.find((s) => s.id === rescheduleQuickReply.value),
)

const rescheduleBehavior = computed(() => selectedRescheduleChip.value?.behavior || 'next')

const showRescheduleNext = computed(() => rescheduleBehavior.value === 'next')
const showRescheduleCloseNotice = computed(() => rescheduleBehavior.value === 'close_lost')

function startReschedule() {
  rescheduling.value = true
  rescheduleDate.value = props.activity.scheduled_at ? toLocalDateInput(props.activity.scheduled_at) : ''
  rescheduleTime.value = props.activity.scheduled_at ? toLocalTimeInput(props.activity.scheduled_at) : ''
  rescheduleQuickReply.value = ''
}

function cancelReschedule() {
  rescheduling.value = false
}

function pickrescheduleQuickReply(id: string) {
  rescheduleQuickReply.value = id
  rescheduleDate.value = ''
  rescheduleTime.value = ''
}

function pickReschedulePreset(preset: NextPreset) {
  const at = preset.at().toISOString()
  rescheduleDate.value = toLocalDateInput(at)
  rescheduleTime.value = toLocalTimeInput(at)
}

// One action: with a next time, the attempt is logged and the next task is
// created; without one, the attempt is logged on its own.
async function saveReschedule() {
  const behavior = rescheduleBehavior.value
  savingReschedule.value = true
  try {
    if (behavior === 'next') {
      const body: Record<string, unknown> = {
        is_done: true,
        quick_reply_id: rescheduleQuickReply.value || null,
      }
      const next = mergeDateTime(rescheduleDate.value, rescheduleTime.value)
      if (next) body.reschedule_at = next
      await apiClient.patch(`/api/leads/${props.leadId}/activities/${props.activity.id}`, body)
      toast.success(next ? 'Attempt logged, next task created' : 'Attempt logged')
      cancelReschedule()
      emit('changed')
      return
    }

    // log / close_lost: complete without a next task.
    await apiClient.patch(`/api/leads/${props.leadId}/activities/${props.activity.id}`, {
      is_done: true,
      quick_reply_id: rescheduleQuickReply.value || null,
    })
    toast.success('Attempt logged')
    cancelReschedule()
    emit('changed')
    if (behavior === 'close_lost') {
      emit('closeLost')
    }
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to complete'))
  } finally {
    savingReschedule.value = false
  }
}

function statusChipClass(id: string): string {
  return rescheduleQuickReply.value === id
    ? 'border-primary bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground'
    : ''
}

// Highlights the preset that produced the current reschedule date/time.
const selectedReschedulePreset = computed(() => {
  const hasTime = !!mergeDateTime(rescheduleDate.value, rescheduleTime.value)
  if (!hasTime) return ''
  const preset = nextPresets.find((p) => {
    const at = p.at().toISOString()
    return toLocalDateInput(at) === rescheduleDate.value && toLocalTimeInput(at) === rescheduleTime.value
  })
  return preset?.label ?? ''
})

function typeLabel(type: string): string {
  return type || 'Task'
}
</script>

<template>
  <Card :class="overdue ? 'border-destructive/40' : ''">
    <CardContent class="p-3">
      <!-- Inline edit form -->
      <div v-if="editing" class="space-y-3">
        <div class="space-y-2">
          <Label class="text-xs">Type</Label>
          <Select v-model="editType">
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
        <div class="space-y-2">
          <Label class="text-xs">Description (optional)</Label>
          <Textarea v-model="editDescription" class="min-h-16" />
        </div>
        <div class="grid grid-cols-2 gap-2">
          <div class="space-y-1.5">
            <Label class="text-xs">Scheduled date</Label>
            <Input v-model="editScheduledDate" type="date" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">Scheduled time</Label>
            <Input v-model="editScheduledTime" type="time" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-2">
          <div class="space-y-1.5">
            <Label class="text-xs">Remind date</Label>
            <Input v-model="editRemindDate" type="date" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">Remind time</Label>
            <Input v-model="editRemindTime" type="time" />
          </div>
        </div>
        <div class="flex justify-end gap-2">
          <Button size="sm" variant="ghost" @click="cancelEdit">Cancel</Button>
          <Button size="sm" :disabled="savingEdit" @click="saveEdit">
            {{ savingEdit ? 'Saving…' : 'Save' }}
          </Button>
        </div>
      </div>

      <!-- Inline reschedule form: log attempt + create next task -->
      <div v-else-if="rescheduling" class="space-y-3">
        <p class="text-sm">Log this attempt.</p>
        <div v-if="groupedQuickReplies.length" class="space-y-2">
          <Label class="text-xs">Quick reply</Label>
          <div class="space-y-2.5">
            <div v-for="g in groupedQuickReplies" :key="g.group">
              <p class="mb-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
                {{ g.group }}
              </p>
              <div class="flex flex-wrap gap-1.5">
                <Button
                  v-for="s in g.items"
                  :key="s.id"
                  size="sm"
                  variant="outline"
                  class="h-7 gap-1 px-2.5 text-xs"
                  :class="statusChipClass(s.id)"
                  @click="pickrescheduleQuickReply(s.id)"
                >
                  <Check v-if="rescheduleQuickReply === s.id" class="size-3" />
                  {{ s.name }}
                </Button>
              </div>
            </div>
          </div>
        </div>

        <template v-if="showRescheduleCloseNotice">
          <p class="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
            This marks the lead as Closed Lost — open tasks will be cancelled.
          </p>
        </template>

        <template v-else-if="showRescheduleNext">
          <div class="space-y-1.5">
            <Label class="text-xs">Quick schedule</Label>
            <div class="flex flex-wrap gap-1.5">
              <Button
                v-for="p in nextPresets"
                :key="p.label"
                size="sm"
                variant="outline"
                class="h-7 gap-1 px-2.5 text-xs"
                :class="
                  selectedReschedulePreset === p.label
                    ? 'border-primary bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground'
                    : ''
                "
                @click="pickReschedulePreset(p)"
              >
                <Check v-if="selectedReschedulePreset === p.label" class="size-3" />
                {{ p.label }}
              </Button>
            </div>
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1.5">
              <Label class="text-xs">Next date</Label>
              <Input v-model="rescheduleDate" type="date" />
            </div>
            <div class="space-y-1.5">
              <Label class="text-xs">Next time</Label>
              <Input v-model="rescheduleTime" type="time" />
            </div>
          </div>
        </template>

        <div class="flex justify-end gap-2">
          <Button size="sm" variant="ghost" @click="cancelReschedule">Cancel</Button>
          <Button size="sm" :disabled="savingReschedule" @click="saveReschedule()">
            {{ savingReschedule ? 'Saving…' : 'Save' }}
          </Button>
        </div>
      </div>

      <!-- Read view -->
      <div v-else class="flex items-start justify-between gap-2">
        <div class="flex items-start gap-2 min-w-0">
          <component :is="reminderIcon(activity.type)" class="size-4 mt-0.5 text-muted-foreground shrink-0" />
          <div class="min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-xs font-medium">{{ typeLabel(activity.type) }}</span>
              <span v-if="activity.quick_reply_name" class="text-xs px-1.5 py-0.5 rounded-full bg-secondary text-secondary-foreground">
                {{ activity.quick_reply_name }}
              </span>
              <Badge v-if="overdue" variant="destructive" class="text-xs">Overdue</Badge>
              <Badge v-if="activity.is_reminded && activity.remind_at" variant="secondary" class="text-xs">Reminded</Badge>
            </div>
            <p v-if="activity.description" class="text-sm mt-0.5">{{ activity.description }}</p>
            <div class="flex flex-wrap gap-2 mt-1">
              <span v-if="activity.scheduled_at" class="text-xs text-muted-foreground">
                Scheduled: {{ formatDateTime(activity.scheduled_at) }}
              </span>
              <span v-if="activity.remind_at" class="text-xs" :class="overdue ? 'text-warning' : 'text-muted-foreground'">
                Reminder: {{ formatDateTime(activity.remind_at) }}
              </span>
            </div>
            <span class="text-xs text-muted-foreground/70 mt-0.5 block">
              {{ formatDateTime(activity.created_at) }} · {{ activity.user_name || 'System' }}
            </span>
          </div>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button variant="ghost" size="icon-sm" class="size-8 shrink-0" aria-label="Task actions">
              <MoreHorizontal class="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem class="cursor-pointer" @click="startEdit">
              <Pencil class="size-3.5 mr-2" /> Edit
            </DropdownMenuItem>
            <DropdownMenuItem class="cursor-pointer" @click="startReschedule">
              <CalendarClock class="size-3.5 mr-2" /> Reschedule
            </DropdownMenuItem>
            <DropdownMenuSub v-if="activity.remind_at">
              <DropdownMenuSubTrigger>
                <AlarmClockPlus class="size-3.5 mr-2" /> Snooze
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                <DropdownMenuItem
                  v-for="preset in snoozePresets"
                  :key="preset.minutes"
                  class="cursor-pointer"
                  @select="emit('snooze', preset.minutes)"
                >
                  {{ preset.label }}
                </DropdownMenuItem>
              </DropdownMenuSubContent>
            </DropdownMenuSub>
            <DropdownMenuItem class="cursor-pointer" @click="emit('markDone')">
              <CheckCircle2 class="size-3.5 mr-2" /> Mark done
            </DropdownMenuItem>
            <DropdownMenuItem class="text-destructive cursor-pointer" @click="emit('delete')">
              <Trash2 class="size-3.5 mr-2" /> Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </CardContent>
  </Card>
</template>
