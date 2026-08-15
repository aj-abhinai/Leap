<script setup lang="ts">
import { computed } from 'vue'
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

const props = defineProps<{
  reminder: Reminder
  overdue?: boolean
}>()

const emit = defineEmits<{
  (e: 'snooze', minutes: number): void
  (e: 'dismiss'): void
}>()

const reminderText = computed(() => formatReminderText(props.reminder))
const icon = computed(() => reminderIcon(props.reminder.type))
const scheduledLabel = computed(() =>
  props.reminder.scheduled_at ? formatDateTime(props.reminder.scheduled_at) : '',
)
const remindLabel = computed(() =>
  props.reminder.remind_at ? formatDateTime(props.reminder.remind_at) : '',
)
const createdLabel = computed(() => formatDateTime(props.reminder.created_at))
</script>

<template>
  <Card :class="overdue ? 'border-warning/60' : ''">
    <CardContent class="p-4">
      <div class="flex items-start justify-between gap-3">
        <div class="flex items-start gap-3">
          <component
            :is="icon"
            class="size-5 mt-0.5 shrink-0"
            :class="overdue ? 'text-warning' : 'text-muted-foreground'"
          />
          <div>
            <div class="flex items-center gap-2">
              <p class="font-medium text-sm">{{ reminderText }}</p>
              <Badge v-if="overdue" variant="destructive" class="text-xs">Overdue</Badge>
            </div>
            <p v-if="reminder.description" class="text-xs text-muted-foreground mt-0.5">{{ reminder.description }}</p>
            <div class="flex flex-wrap gap-x-3 gap-y-0.5 mt-1 text-xs text-muted-foreground">
              <span v-if="scheduledLabel">Scheduled: {{ scheduledLabel }}</span>
              <span v-if="remindLabel" :class="overdue ? 'text-warning' : ''">
                Reminder: {{ remindLabel }}
              </span>
            </div>
            <p class="text-xs text-muted-foreground mt-1">Created {{ createdLabel }}</p>
          </div>
        </div>
        <div v-if="!reminder.is_done && !reminder.is_cancelled" class="flex items-center gap-1 shrink-0">
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
