<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { useRemindersStore } from '@/stores/reminders'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { BellOff, X } from '@lucide/vue'
import { formatReminderText, formatReminderTime, reminderIcon } from '@/utils/reminders'
import { timeAgo } from '@/utils/time'

const emit = defineEmits<{ close: [] }>()
const store = useRemindersStore()
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
      <div v-else-if="store.openReminders.length === 0" class="flex flex-col items-center justify-center flex-1 text-muted-foreground py-12">
        <BellOff class="h-7 w-7 mb-2" />
        <p class="text-xs">No pending reminders</p>
      </div>
      <div v-else class="divide-y">
        <div
          v-for="reminder in store.openReminders"
          :key="reminder.id"
          class="group relative px-3 py-2.5 hover:bg-muted/50 cursor-pointer transition-colors"
        >
          <div class="flex gap-2.5">
            <component :is="reminderIcon(reminder.type)" class="flex-shrink-0 h-4 w-4 mt-0.5 text-muted-foreground" />
            <div class="flex-1 min-w-0">
              <p class="text-xs font-medium">{{ formatReminderText(reminder) }}</p>
              <p v-if="formatReminderTime(reminder)" class="text-xs text-muted-foreground mt-0.5">{{ formatReminderTime(reminder) }}</p>
              <p class="text-xs text-muted-foreground/70 mt-0.5">{{ timeAgo(reminder.created_at) }}</p>
            </div>
            <div class="flex items-start">
              <Button variant="ghost" size="sm" class="h-5 w-5 p-0 text-muted-foreground hover:text-destructive" @click.stop="store.dismissReminder(reminder.id)" aria-label="Dismiss reminder">
                <X class="h-2.5 w-2.5" />
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div v-if="store.openReminders.length > 0" class="p-2 border-t">
      <RouterLink to="/reminders">
        <Button variant="ghost" size="sm" class="w-full text-xs" @click="emit('close')">
          View All Reminders
        </Button>
      </RouterLink>
    </div>
  </div>
</template>
