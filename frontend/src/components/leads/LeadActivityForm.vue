<script setup lang="ts">
import { ref, computed } from 'vue'
import { apiClient } from '@/composables/useApi'
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

const activityType = ref('note')
const description = ref('')
const scheduledAt = ref('')
const remindAt = ref('')
const saving = ref(false)
const error = ref('')

const showScheduled = computed(() =>
  activityType.value === 'call_scheduled' || activityType.value === 'call_rescheduled'
)

async function handleSave() {
  error.value = ''
  if (!description.value.trim()) {
    error.value = 'Description is required'
    return
  }
  saving.value = true
  try {
    await apiClient.post(`/api/leads/${props.leadId}/activities`, {
      type: activityType.value,
      description: description.value,
      scheduled_at: scheduledAt.value ? new Date(scheduledAt.value).toISOString() : null,
      remind_at: remindAt.value ? new Date(remindAt.value).toISOString() : null,
    })
    toast.success('Activity logged')
    description.value = ''
    scheduledAt.value = ''
    remindAt.value = ''
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
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="wa_message">WhatsApp Message</SelectItem>
          <SelectItem value="call_scheduled">Call Scheduled</SelectItem>
          <SelectItem value="call_rescheduled">Call Rescheduled</SelectItem>
          <SelectItem value="note">Note</SelectItem>
          <SelectItem value="other">Other</SelectItem>
        </SelectContent>
      </Select>
    </div>
    <div class="space-y-2">
      <Label class="text-xs">Description</Label>
      <Textarea v-model="description" placeholder="What happened?" class="min-h-20" />
    </div>
    <div v-if="showScheduled" class="space-y-2">
      <Label class="text-xs">Scheduled Date/Time</Label>
      <Input v-model="scheduledAt" type="datetime-local" />
    </div>
    <div class="space-y-2">
      <Label class="text-xs">Remind me at</Label>
      <Input v-model="remindAt" type="datetime-local" />
    </div>
    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
    <div class="flex justify-end">
      <Button size="sm" :disabled="saving" @click="handleSave">
        {{ saving ? 'Saving...' : 'Save' }}
      </Button>
    </div>
  </div>
</template>
