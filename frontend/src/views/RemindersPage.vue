<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { useRemindersStore, type Reminder } from '@/stores/reminders'
import { useAuthStore } from '@/stores/auth'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Skeleton } from '@/components/ui/skeleton'
import ReminderCard from '@/components/leads/ReminderCard.vue'
import { BellOff } from '@lucide/vue'
import { snoozeRemindAt } from '@/utils/reminders'

const store = useRemindersStore()
const auth = useAuthStore()
const myOnly = shallowRef(false)

onMounted(() => store.fetchReminders())

const visible = computed(() => {
  if (!myOnly.value || !auth.user?.id) return store.reminders
  return store.reminders.filter((r) => r.user_id === auth.user!.id)
})

const now = () => Date.now()

const overdue = computed(() =>
  visible.value.filter(
    (r) =>
      !r.is_done && !r.is_cancelled &&
      !!r.remind_at && new Date(r.remind_at).getTime() < now() && !r.is_reminded,
  ),
)
const upcoming = computed(() =>
  visible.value.filter(
    (r) =>
      !r.is_done && !r.is_cancelled &&
      ((!!r.remind_at && new Date(r.remind_at).getTime() >= now() && !r.is_reminded) ||
        (!!r.scheduled_at && !r.remind_at && !r.is_reminded)),
  ),
)
const dismissed = computed(() =>
  visible.value.filter((r) => !r.is_done && !r.is_cancelled && r.is_reminded),
)
const done = computed(() => visible.value.filter((r) => r.is_done || r.is_cancelled))

async function snooze(reminder: Reminder, minutes: number) {
  try {
    await store.snoozeReminder(reminder, snoozeRemindAt(minutes))
  } catch {}
}

function hasAny(list: unknown[]): boolean {
  return list.length > 0
}
</script>

<template>
  <LayoutShell>
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

      <div v-if="store.loading" class="space-y-3">
        <Skeleton v-for="i in 5" :key="i" class="h-16 w-full" />
      </div>

      <div
        v-else-if="visible.length === 0"
        class="flex flex-col items-center justify-center py-16 text-center"
      >
        <BellOff class="size-10 text-muted-foreground/40 mb-3" />
        <p class="text-sm font-medium text-muted-foreground">No pending reminders</p>
        <p class="text-xs text-muted-foreground/60 mt-1">Create tasks with reminders from the leads kanban</p>
      </div>

      <div v-else class="space-y-6 max-w-2xl">
        <section v-if="hasAny(overdue)">
          <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-warning">Overdue</h2>
          <div class="space-y-3">
            <ReminderCard
              v-for="reminder in overdue"
              :key="reminder.id"
              :reminder="reminder"
              overdue
              @snooze="snooze(reminder, $event)"
              @dismiss="store.dismissReminder(reminder)"
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
              @dismiss="store.dismissReminder(reminder)"
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
              @dismiss="store.dismissReminder(reminder)"
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
    </div>
  </LayoutShell>
</template>
