<script setup lang="ts">
import { onMounted } from 'vue'
import { useRemindersStore } from '@/stores/reminders'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { BellOff, X } from '@lucide/vue'
import { formatReminderText, reminderIcon } from '@/utils/reminders'
import { formatDateTime } from '@/utils/time'

const store = useRemindersStore()

onMounted(() => store.fetchReminders())
</script>

<template>
  <LayoutShell>
    <div class="p-6">
      <div class="mb-4 flex flex-col">
        <h1 class="text-2xl font-semibold tracking-tight">Reminders</h1>
        <p v-if="!store.loading && store.reminders.length" class="mt-0.5 text-sm text-muted-foreground">
          <span class="tabular-nums">{{ store.reminders.length }}</span> pending
        </p>
      </div>

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
                <component :is="reminderIcon(reminder.type)" class="size-5 mt-0.5 text-muted-foreground shrink-0" />
                <div>
                  <p class="font-medium text-sm">{{ formatReminderText(reminder) }}</p>
                  <div class="flex flex-wrap gap-x-3 gap-y-0.5 mt-1 text-xs text-muted-foreground">
                    <span v-if="reminder.scheduled_at">Scheduled: {{ formatDateTime(reminder.scheduled_at) }}</span>
                    <span v-if="reminder.remind_at" class="text-warning">Reminder: {{ formatDateTime(reminder.remind_at) }}</span>
                  </div>
                  <p class="text-xs text-muted-foreground mt-1">Created {{ formatDateTime(reminder.created_at) }}</p>
                </div>
              </div>
              <Button variant="ghost" size="icon-sm" class="shrink-0" @click="store.dismissReminder(reminder.id)" title="Dismiss" aria-label="Dismiss reminder">
                <X class="size-4" />
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  </LayoutShell>
</template>
