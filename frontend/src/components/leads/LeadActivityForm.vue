<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
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

const props = defineProps<{ leadId: string }>()

const emit = defineEmits<{
  saved: []
}>()

const settings = useSettingsStore()

const activityType = shallowRef('')
const description = shallowRef('')
const outcomeId = shallowRef('')
const scheduledDate = shallowRef('')
const scheduledTime = shallowRef('')
const remindDate = shallowRef('')
const remindTime = shallowRef('')
const saving = shallowRef(false)
const error = shallowRef('')

const activityTypes = computed(() => settings.activityTypes)

const showScheduled = computed(() => {
  const t = activityType.value.toLowerCase()
  return t.includes('call') || t.includes('reschedule')
})

onMounted(() => {
  if (settings.activityTypes.length === 0) settings.fetchTags()
})

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
  saving.value = true
  try {
    await apiClient.post(`/api/leads/${props.leadId}/activities`, {
      type: activityType.value,
      description: description.value,
      outcome_id: outcomeId.value || null,
      scheduled_at: showScheduled.value ? mergeDateTime(scheduledDate.value, scheduledTime.value) : null,
      remind_at: mergeDateTime(remindDate.value, remindTime.value),
    })
    toast.success('Activity logged')
    description.value = ''
    outcomeId.value = ''
    scheduledDate.value = ''
    scheduledTime.value = ''
    remindDate.value = ''
    remindTime.value = ''
    emit('saved')
  } catch (e: any) {
    error.value = e.message || 'Failed to save'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="space-y-3 rounded-lg border p-3">
    <h4 class="text-sm font-medium">Log Activity</h4>
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
    <div class="space-y-2">
      <Label class="text-xs">Description</Label>
      <Textarea v-model="description" placeholder="What happened?" class="min-h-20" />
    </div>
    <div class="space-y-2">
      <Label class="text-xs">Status (optional)</Label>
      <Select v-model="outcomeId">
        <SelectTrigger>
          <SelectValue placeholder="No status" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="s in settings.statuses" :key="s.id" :value="s.id">
            {{ s.name }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>
    <div v-if="showScheduled" class="grid grid-cols-2 gap-2">
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
    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
    <div class="flex justify-end">
      <Button size="sm" :disabled="saving" @click="handleSave">
        {{ saving ? 'Saving...' : 'Save' }}
      </Button>
    </div>
  </div>
</template>
