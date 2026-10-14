import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as api from '@/api/reminders'
import { localEnv, resolveTask } from '@/tasks'

export type { Reminder } from '@/api/reminders'

const tasksEnv = localEnv()

export const useRemindersStore = defineStore('reminders', () => {
  const reminders = ref<api.Reminder[]>([])
  const loading = ref(false)
  const loadError = ref('')

  // Open tasks = not done, not cancelled, not yet reminded — a dismissed
  // reminder leaves the bell (snooze re-opens it). The panel and the badge
  // read the same set, so a dismissed row cannot be dismissed twice.
  const openReminders = computed(() =>
    reminders.value.filter(
      (r) => !r.is_done && !r.is_cancelled && !r.is_reminded && (!!r.remind_at || !!r.scheduled_at),
    ),
  )

  const pendingCount = computed(() => {
    const endOfToday = new Date()
    endOfToday.setHours(23, 59, 59, 999)
    // The badge is the day-start digest: all tasks due today — overdue plus
    // upcoming today — so the first open of the day reads "N tasks today".
    return openReminders.value.filter((r) => {
      const state = resolveTask(r, tasksEnv).state
      if (state.kind !== 'open' || !state.dueAt) return false
      return new Date(state.dueAt).getTime() <= endOfToday.getTime()
    }).length
  })

  // fetchReminders never rejects: the bell poller and the page mount call it
  // fire-and-forget, so a failed refresh keeps the last known list and
  // reports through loadError instead of an unhandled rejection per poll.
  async function fetchReminders() {
    loading.value = true
    loadError.value = ''
    try {
      const res = await api.listReminders()
      reminders.value = res.data
    } catch {
      loadError.value = 'Failed to refresh reminders'
    } finally {
      loading.value = false
    }
  }

  // Pure mutation actions: they hit the API and leave toasts/refetches to
  // the calling view so the store stays UI-free.
  async function dismissReminder(leadId: string, reminderId: string) {
    await api.dismissReminder(leadId, reminderId)
  }

  async function snoozeReminder(leadId: string, reminderId: string, remindAt: string) {
    await api.snoozeReminder(leadId, reminderId, remindAt)
  }

  return { reminders, loading, loadError, pendingCount, openReminders, fetchReminders, dismissReminder, snoozeReminder }
})