<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRemindersStore, type Reminder } from '@/stores/reminders'
import ReminderCard from '@/components/leads/ReminderCard.vue'
import PageState from '@/components/PageState.vue'
import { BellOff } from '@lucide/vue'
import { snoozeTarget } from '@/utils/reminders'
import { statusLabel } from '@/utils/activity'
import { toast } from 'vue-sonner'
import { errorMessage } from '@/utils/errors'

const store = useRemindersStore()

onMounted(() => store.fetchReminders())

// /api/reminders is already the current user's recipient queue (lead assignee
// → task creator on unassigned leads → everyone on genuinely unowned work), so
// the page renders it as-is; filtering by task creator here would hide work the
// user is responsible for.
const overdue = computed(() => store.reminders.filter((r) => statusLabel(r) === 'Overdue'))
const upcoming = computed(() => store.reminders.filter((r) => statusLabel(r) === 'Open'))
const dismissed = computed(() => store.reminders.filter((r) => statusLabel(r) === 'Reminded'))

// snooze pushes the reminder forward by minutes; failures surface as a toast
// and leave the card in place.
async function snooze(reminder: Reminder, minutes: number) {
  try {
    await store.snoozeReminder(reminder.lead_id, reminder.id, snoozeTarget(reminder, minutes))
    toast.success('Reminder snoozed')
    await store.fetchReminders()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to snooze'))
  }
}

async function dismiss(reminder: Reminder) {
  try {
    await store.dismissReminder(reminder.lead_id, reminder.id)
    toast.success('Reminder dismissed')
    await store.fetchReminders()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to dismiss reminder'))
  }
}

function hasAny(list: unknown[]): boolean {
  return list.length > 0
}
</script>

<template>
  <div class="p-6">
    <div class="mb-4 flex flex-col">
      <h1 class="text-2xl font-semibold tracking-tight">Reminders</h1>
      <p v-if="!store.loading && store.reminders.length" class="mt-0.5 text-sm text-muted-foreground">
        <span class="tabular-nums">{{ overdue.length + upcoming.length }}</span> pending
      </p>
    </div>

    <PageState
      :loading="store.loading"
      :empty="store.reminders.length === 0"
      empty-title="No pending reminders"
      empty-hint="Create tasks with reminders from the leads kanban"
      :skeleton-count="5"
      skeleton-class="h-16 w-full"
    >
      <template #empty-icon>
        <BellOff class="mb-3 size-10 text-muted-foreground/40" />
      </template>
      <div class="space-y-6 max-w-2xl">
      <section v-if="hasAny(overdue)">
        <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-warning">Overdue</h2>
        <div class="space-y-3">
          <ReminderCard
            v-for="reminder in overdue"
            :key="reminder.id"
            :reminder="reminder"
            overdue
            @snooze="snooze(reminder, $event)"
            @dismiss="dismiss(reminder)"
          />
        </div>
      </section>

      <section v-if="hasAny(upcoming)">
        <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">Upcoming</h2>
        <div class="space-y-3">
          <ReminderCard
            v-for="reminder in upcoming"
            :key="reminder.id"
            :reminder="reminder"
            @snooze="snooze(reminder, $event)"
            @dismiss="dismiss(reminder)"
          />
        </div>
      </section>

      <section v-if="hasAny(dismissed)">
        <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">Dismissed</h2>
        <div class="space-y-3 opacity-70">
          <ReminderCard
            v-for="reminder in dismissed"
            :key="reminder.id"
            :reminder="reminder"
            @snooze="snooze(reminder, $event)"
            @dismiss="dismiss(reminder)"
          />
        </div>
      </section>
      </div>
    </PageState>
  </div>
</template>
