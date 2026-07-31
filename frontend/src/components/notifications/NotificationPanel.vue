<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useRemindersStore, type Reminder } from '@/stores/reminders'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { BellOff, Phone, MessageCircle, NotepadText, X } from '@lucide/vue'

const emit = defineEmits<{ close: [] }>()
const store = useRemindersStore()

onMounted(() => store.fetchReminders())

function getIcon(type: string) {
  switch (type) {
    case 'call_scheduled': return Phone
    case 'call_rescheduled': return Phone
    case 'wa_message': return MessageCircle
    default: return NotepadText
  }
}

function formatReminderText(r: Reminder): string {
  switch (r.type) {
    case 'call_scheduled': return `Call scheduled: ${r.description || 'Follow-up call'}`
    case 'call_rescheduled': return `Call rescheduled: ${r.description || 'Follow-up call'}`
    case 'wa_message': return `WhatsApp: ${r.description || 'Send message'}`
    default: return r.description || 'Reminder'
  }
}

function formatTime(r: Reminder): string {
  if (r.scheduled_at) {
    return `Scheduled for ${new Date(r.scheduled_at).toLocaleString()}`
  }
  if (r.remind_at) {
    return `Reminder at ${new Date(r.remind_at).toLocaleString()}`
  }
  return ''
}

function relativeTime(date: string): string {
  const diff = Date.now() - new Date(date).getTime()
  const minutes = Math.floor(Math.abs(diff) / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}
</script>

<template>
  <div class="flex flex-col min-h-64 max-h-96">
    <div class="flex items-center justify-between px-3 py-2.5 border-b">
      <h3 class="font-semibold text-sm">Reminders</h3>
    </div>
    <div class="flex-1 overflow-y-auto">
      <div v-if="store.loading && store.reminders.length === 0" class="divide-y">
        <div v-for="i in 4" :key="i" class="flex gap-2.5 px-3 py-2.5">
          <Skeleton class="h-7 w-7 rounded-full shrink-0" />
          <div class="flex-1 space-y-1.5">
            <Skeleton class="h-3 w-3/4" />
            <Skeleton class="h-3 w-1/2" />
          </div>
        </div>
      </div>
      <div v-else-if="store.reminders.length === 0" class="flex flex-col items-center justify-center flex-1 text-muted-foreground py-12">
        <BellOff class="h-7 w-7 mb-2" />
        <p class="text-xs">No pending reminders</p>
      </div>
      <div v-else class="divide-y">
        <div
          v-for="reminder in store.reminders"
          :key="reminder.id"
          class="group relative px-3 py-2.5 hover:bg-muted/50 cursor-pointer transition-colors"
        >
          <div class="flex gap-2.5">
            <component :is="getIcon(reminder.type)" class="flex-shrink-0 h-4 w-4 mt-0.5 text-muted-foreground" />
            <div class="flex-1 min-w-0">
              <p class="text-xs font-medium">{{ formatReminderText(reminder) }}</p>
              <p v-if="formatTime(reminder)" class="text-xs text-muted-foreground mt-0.5">{{ formatTime(reminder) }}</p>
              <p class="text-xs text-muted-foreground/70 mt-0.5">{{ relativeTime(reminder.created_at) }}</p>
            </div>
            <div class="flex items-start opacity-0 group-hover:opacity-100 transition-opacity">
              <Button variant="ghost" size="sm" class="h-5 w-5 p-0 hover:text-destructive" @click.stop="store.dismissReminder(reminder.id)">
                <X class="h-2.5 w-2.5" />
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div v-if="store.reminders.length > 0" class="p-2 border-t">
      <RouterLink to="/reminders">
        <Button variant="ghost" size="sm" class="w-full text-xs" @click="emit('close')">
          View All Reminders
        </Button>
      </RouterLink>
    </div>
  </div>
</template>
