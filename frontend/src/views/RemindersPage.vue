<script setup lang="ts">
import { onMounted } from 'vue'
import { useRemindersStore, type Reminder } from '@/stores/reminders'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { BellOff, Phone, MessageCircle, NotepadText, X } from '@lucide/vue'

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

function formatText(r: Reminder): string {
  switch (r.type) {
    case 'call_scheduled': return `Call scheduled: ${r.description || 'Follow-up call'}`
    case 'call_rescheduled': return `Call rescheduled: ${r.description || 'Follow-up call'}`
    case 'wa_message': return `WhatsApp: ${r.description || 'Send message'}`
    default: return r.description || 'Reminder'
  }
}

function formatDate(date: string): string {
  return new Date(date).toLocaleString()
}
</script>

<template>
  <LayoutShell>
    <div class="p-6">
      <h2 class="text-xl font-semibold mb-4">All Reminders</h2>

      <div v-if="store.loading" class="space-y-3">
        <Skeleton v-for="i in 5" :key="i" class="h-16 w-full" />
      </div>

      <div v-else-if="store.reminders.length === 0" class="flex flex-col items-center justify-center py-16 text-center">
        <BellOff class="size-10 text-muted-foreground/40 mb-3" />
        <p class="text-sm font-medium text-muted-foreground">No pending reminders</p>
        <p class="text-xs text-muted-foreground/60 mt-1">Create activities with reminders from the leads kanban</p>
      </div>

      <div v-else class="space-y-3 max-w-2xl">
        <Card v-for="reminder in store.reminders" :key="reminder.id">
          <CardContent class="p-4">
            <div class="flex items-start justify-between gap-3">
              <div class="flex items-start gap-3">
                <component :is="getIcon(reminder.type)" class="size-5 mt-0.5 text-muted-foreground shrink-0" />
                <div>
                  <p class="font-medium text-sm">{{ formatText(reminder) }}</p>
                  <div class="flex flex-wrap gap-x-3 gap-y-0.5 mt-1 text-xs text-muted-foreground">
                    <span v-if="reminder.scheduled_at">Scheduled: {{ formatDate(reminder.scheduled_at) }}</span>
                    <span v-if="reminder.remind_at" class="text-amber-600">Reminder: {{ formatDate(reminder.remind_at) }}</span>
                  </div>
                  <p class="text-xs text-muted-foreground mt-1">Created {{ formatDate(reminder.created_at) }}</p>
                </div>
              </div>
              <Button variant="ghost" size="icon-sm" class="shrink-0" @click="store.dismissReminder(reminder.id)" title="Dismiss">
                <X class="size-4" />
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  </LayoutShell>
</template>
