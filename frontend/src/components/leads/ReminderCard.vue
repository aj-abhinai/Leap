<script setup lang="ts">
import type { Reminder } from '@/stores/reminders'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
  DropdownMenuSub, DropdownMenuSubContent, DropdownMenuSubTrigger,
} from '@/components/ui/dropdown-menu'
import { AlarmClockPlus, X } from '@lucide/vue'
import { formatReminderText, reminderIcon, snoozePresets } from '@/utils/reminders'
import { formatDateTime } from '@/utils/time'

defineProps<{
  reminder: Reminder
  overdue?: boolean
}>()

const emit = defineEmits<{
  (e: 'snooze', minutes: number): void
  (e: 'dismiss'): void
}>()
</script>

<template>
  <Card :class="overdue ? 'border-warning/60' : ''">
    <CardContent class="p-4">
      <div class="flex items-start justify-between gap-3">
        <div class="flex items-start gap-3">
          <component
            :is="reminderIcon(reminder.type)"
            class="size-5 mt-0.5 shrink-0"
            :class="overdue ? 'text-warning' : 'text-muted-foreground'"
          />
          <div>
            <div class="flex items-center gap-2">
              <p class="font-medium text-sm">{{ formatReminderText(reminder) }}</p>
              <Badge v-if="overdue" variant="destructive" class="text-xs">Overdue</Badge>
            </div>
            <p v-if="reminder.description" class="text-xs text-muted-foreground mt-0.5">{{ reminder.description }}</p>
            <div class="flex flex-wrap gap-x-3 gap-y-0.5 mt-1 text-xs text-muted-foreground">
              <span v-if="reminder.scheduled_at">Scheduled: {{ formatDateTime(reminder.scheduled_at) }}</span>
              <span v-if="reminder.remind_at" :class="overdue ? 'text-warning' : ''">
                Reminder: {{ formatDateTime(reminder.remind_at) }}
              </span>
            </div>
            <p class="text-xs text-muted-foreground mt-1">Created {{ formatDateTime(reminder.created_at) }}</p>
          </div>
        </div>
        <div v-if="!reminder.is_done" class="flex items-center gap-1 shrink-0">
          <DropdownMenu v-if="reminder.remind_at">
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon-sm" aria-label="Snooze reminder">
                <AlarmClockPlus class="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>Snooze</DropdownMenuSubTrigger>
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
            </DropdownMenuContent>
          </DropdownMenu>
          <Button
            v-if="!reminder.is_reminded"
            variant="ghost"
            size="icon-sm"
            title="Dismiss"
            aria-label="Dismiss reminder"
            @click="emit('dismiss')"
          >
            <X class="size-4" />
          </Button>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
