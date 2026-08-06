<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { apiClient } from '@/composables/useApi'
import { useSettingsStore } from '@/stores/settings'
import { toast } from 'vue-sonner'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
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
  MoreHorizontal, Trash2, CheckCircle2, Pencil, AlarmClockPlus,
} from '@lucide/vue'
import { reminderIcon, snoozePresets, snoozeRemindAt } from '@/utils/reminders'
import { formatDateTime } from '@/utils/time'
import { Badge } from '@/components/ui/badge'

interface Activity {
  id: string
  lead_id: string
  stage_id: string
  stage_name?: string
  user_id?: string
  user_name?: string
  type: string
  description: string
  outcome_id?: string
  outcome_name?: string
  scheduled_at?: string
  remind_at?: string
  responded_at?: string
  is_done: boolean
  is_reminded: boolean
  created_at: string
}

const props = defineProps<{ leadId: string }>()

const activities = shallowRef<Activity[]>([])
const loading = shallowRef(false)
const loadError = shallowRef('')

const settings = useSettingsStore()

onMounted(() => {
  fetchActivities()
  if (settings.activityTypes.length === 0) settings.fetchTags()
})

async function fetchActivities() {
  loading.value = true
  loadError.value = ''
  try {
    const res = await apiClient.get(`/api/leads/${props.leadId}/activities`)
    activities.value = res.data
  } catch (e: any) {
    activities.value = []
    loadError.value = e.message || 'Failed to load activities'
  } finally {
    loading.value = false
  }
}

async function deleteActivity(id: string) {
  try {
    await apiClient.delete(`/api/leads/${props.leadId}/activities/${id}`)
    toast.success('Activity deleted')
    await fetchActivities()
  } catch (e: any) {
    toast.error(e.message || 'Failed to delete')
  }
}

async function markResponse(a: Activity) {
  try {
    await apiClient.patch(`/api/leads/${props.leadId}/activities/${a.id}`, {
      outcome_id: a.outcome_id || null,
      is_done: true,
    })
    toast.success('Marked as done')
    await fetchActivities()
  } catch (e: any) {
    toast.error(e.message || 'Failed to mark response')
  }
}

async function snooze(a: Activity, minutes: number) {
  try {
    await apiClient.post(`/api/reminders/${a.id}/snooze`, {
      remind_at: snoozeRemindAt(minutes),
    })
    toast.success('Reminder snoozed')
    await fetchActivities()
  } catch (e: any) {
    toast.error(e.message || 'Failed to snooze')
  }
}

function isOverdue(a: Activity): boolean {
  return !!a.remind_at && new Date(a.remind_at).getTime() < Date.now() && !a.is_reminded && !a.is_done
}

// reka-ui menu items emit a native `select` event instead of click.
function onSnoozeSelect(a: Activity, minutes: number) {
  snooze(a, minutes)
}

// --- inline edit state ---
const editingId = shallowRef<string | null>(null)
const editType = shallowRef('')
const editDescription = shallowRef('')
const editRemindDate = shallowRef('')
const editRemindTime = shallowRef('')
const savingEdit = shallowRef(false)

function startEdit(a: Activity) {
  editingId.value = a.id
  editType.value = a.type
  editDescription.value = a.description
  editRemindDate.value = a.remind_at ? new Date(a.remind_at).toISOString().slice(0, 10) : ''
  editRemindTime.value = a.remind_at ? new Date(a.remind_at).toISOString().slice(11, 16) : ''
}

function cancelEdit() {
  editingId.value = null
}

function mergeEditDateTime(date: string, time: string): string | null {
  if (!date || !time) return null
  return new Date(`${date}T${time}`).toISOString()
}

async function saveEdit(a: Activity) {
  if (!editType.value) {
    toast.error('Type is required')
    return
  }
  if (!editDescription.value.trim()) {
    toast.error('Description is required')
    return
  }
  savingEdit.value = true
  try {
    await apiClient.patch(`/api/leads/${props.leadId}/activities/${a.id}`, {
      type: editType.value,
      description: editDescription.value,
      remind_at: mergeEditDateTime(editRemindDate.value, editRemindTime.value),
    })
    toast.success('Activity updated')
    cancelEdit()
    await fetchActivities()
  } catch (e: any) {
    toast.error(e.message || 'Failed to update')
  } finally {
    savingEdit.value = false
  }
}

function typeLabel(type: string): string {
  const map: Record<string, string> = {
    call_scheduled: 'Call Scheduled',
    call_rescheduled: 'Call Rescheduled',
    wa_message: 'WhatsApp Message',
    note: 'Note',
    other: 'Other',
  }
  return map[type] || type
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
      <p class="text-sm text-muted-foreground">No activities logged yet</p>
    </div>

    <div v-else class="space-y-2">
      <Card v-for="a in activities" :key="a.id">
        <CardContent class="p-3">
          <!-- Inline edit form -->
          <div v-if="editingId === a.id" class="space-y-3">
            <div class="space-y-2">
              <Label class="text-xs">Type</Label>
              <Select v-model="editType">
                <SelectTrigger>
                  <SelectValue placeholder="Select type" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="t in settings.activityTypes" :key="t.id" :value="t.name">
                    {{ t.name }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="space-y-2">
              <Label class="text-xs">Description</Label>
              <Textarea v-model="editDescription" class="min-h-16" />
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
              <Button size="sm" :disabled="savingEdit" @click="saveEdit(a)">
                {{ savingEdit ? 'Saving…' : 'Save' }}
              </Button>
            </div>
          </div>

          <!-- Read view -->
          <div v-else class="flex items-start justify-between gap-2">
            <div class="flex items-start gap-2 min-w-0">
              <component :is="reminderIcon(a.type)" class="size-4 mt-0.5 text-muted-foreground shrink-0" />
              <div>
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="text-xs font-medium">{{ typeLabel(a.type) }}</span>
                  <span v-if="a.outcome_name" class="text-xs px-1.5 py-0.5 rounded-full bg-secondary text-secondary-foreground">
                    {{ a.outcome_name }}
                  </span>
                  <Badge v-if="isOverdue(a)" variant="destructive" class="text-xs">Overdue</Badge>
                  <span class="text-xs text-muted-foreground">{{ a.user_name || 'System' }}</span>
                </div>
                <p v-if="a.description" class="text-sm mt-0.5">{{ a.description }}</p>
                <div class="flex flex-wrap gap-2 mt-1">
                  <span v-if="a.scheduled_at" class="text-xs text-muted-foreground">
                    Scheduled: {{ formatDateTime(a.scheduled_at) }}
                  </span>
                  <span v-if="a.remind_at" class="text-xs text-warning">
                    Reminder: {{ formatDateTime(a.remind_at) }}
                  </span>
                  <span v-if="a.responded_at" class="text-xs text-success">
                    Responded: {{ formatDateTime(a.responded_at) }}
                  </span>
                </div>
                <span class="text-xs text-muted-foreground/70 mt-0.5 block">{{ formatDateTime(a.created_at) }}</span>
              </div>
            </div>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon-sm" class="size-8 shrink-0" aria-label="Activity actions">
                  <MoreHorizontal class="size-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem class="cursor-pointer" @click="startEdit(a)">
                  <Pencil class="size-3.5 mr-2" /> Edit
                </DropdownMenuItem>
                <DropdownMenuSub v-if="a.remind_at && !a.is_done">
                  <DropdownMenuSubTrigger>
                    <AlarmClockPlus class="size-3.5 mr-2" /> Snooze
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent>
                    <DropdownMenuItem
                      v-for="preset in snoozePresets"
                      :key="preset.minutes"
                      class="cursor-pointer"
                      @select="onSnoozeSelect(a, preset.minutes)"
                    >
                      {{ preset.label }}
                    </DropdownMenuItem>
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
                <DropdownMenuItem class="cursor-pointer" @click="markResponse(a)">
                  <CheckCircle2 class="size-3.5 mr-2" /> Mark response
                </DropdownMenuItem>
                <DropdownMenuItem class="text-destructive cursor-pointer" @click="deleteActivity(a.id)">
                  <Trash2 class="size-3.5 mr-2" /> Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
