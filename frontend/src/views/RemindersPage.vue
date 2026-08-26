<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { useRemindersStore, type Reminder } from '@/stores/reminders'
import { useAuthStore } from '@/stores/auth'
import ReminderCard from '@/components/leads/ReminderCard.vue'
import PageState from '@/components/PageState.vue'
import { BellOff } from '@lucide/vue'
import { snoozeRemindAt } from '@/utils/reminders'
import { statusLabel } from '@/utils/activity'
import { toast } from 'vue-sonner'
import { errorMessage } from '@/utils/errors'

const store = useRemindersStore()
const auth = useAuthStore()
const myOnly = shallowRef(false)

onMounted(() => store.fetchReminders())

const visible = computed(() => {
  if (!myOnly.value || !auth.user?.id) return store.reminders
  return store.reminders.filter((r) => r.user_id === auth.user!.id)
})

// Buckets derive from the shared status derivation: overdue = past remind_at
// not yet reminded; upcoming = open with a future remind/schedule; dismissed =
// reminded but open; done = done or cancelled.
const overdue = computed(() => visible.value.filter((r) => statusLabel(r) === 'Overdue'))
const upcoming = computed(() => visible.value.filter((r) => statusLabel(r) === 'Open'))
const dismissed = computed(() => visible.value.filter((r) => statusLabel(r) === 'Reminded'))
const done = computed(() => visible.value.filter((r) => r.is_done || r.is_cancelled))

// snooze pushes the reminder forward by minutes; failures surface as a toast
// and leave the card in place.
async function snooze(reminder: Reminder, minutes: number) {
  try {
    await store.snoozeReminder(reminder.lead_id, reminder.id, snoozeRemindAt(minutes))
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
        <span class="tabular-nums">{{ overdue.length + upcoming.length }}</span>
        {{ myOnly ? 'of my' : '' }} pending
      </p>
    </div>

    <div class="mb-4 flex items-center gap-3">
      <label class="flex items-center gap-2 text-sm text-muted-foreground">
        <input v-model="myOnly" type="checkbox" class="size-4" />
        My reminders only
      </label>
    </div>

    <PageState
      :loading="store.loading"
      :empty="visible.length === 0"
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

      <section v-if="hasAny(done)">
        <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">Done</h2>
        <div class="space-y-3 opacity-70">
          <ReminderCard
            v-for="reminder in done"
            :key="reminder.id"
            :reminder="reminder"
          />
        </div>
      </section>
      </div>
    </PageState>
  </div>
</template>
