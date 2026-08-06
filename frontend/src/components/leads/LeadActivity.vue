<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { MoreHorizontal, Trash2, CheckCircle2 } from '@lucide/vue'
import { reminderIcon } from '@/utils/reminders'
import { formatDateTime } from '@/utils/time'

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

onMounted(() => fetchActivities())

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
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-start gap-2 min-w-0">
              <component :is="reminderIcon(a.type)" class="size-4 mt-0.5 text-muted-foreground shrink-0" />
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-xs font-medium">{{ typeLabel(a.type) }}</span>
                  <span v-if="a.outcome_name" class="text-xs px-1.5 py-0.5 rounded-full bg-secondary text-secondary-foreground">
                    {{ a.outcome_name }}
                  </span>
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
