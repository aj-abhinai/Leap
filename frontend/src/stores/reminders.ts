import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as api from '@/api/reminders'

export type { Reminder } from '@/api/reminders'

export const useRemindersStore = defineStore('reminders', () => {
  const reminders = ref<api.Reminder[]>([])
  const loading = ref(false)

  // Open tasks = not done, not cancelled, not yet reminded. Reminders surface
  // open tasks that have a remind time or a scheduled time.
  const openReminders = computed(() =>
    reminders.value.filter(
      (r) => !r.is_done && !r.is_cancelled && (!!r.remind_at || !!r.scheduled_at),
    ),
  )

  const pendingCount = computed(() => {
    const endOfToday = new Date()
    endOfToday.setHours(23, 59, 59, 999)
    // The badge is the day-start digest (ADR 004): all tasks due today —
    // overdue plus upcoming today — so the first open of the day reads
    // "N tasks today".
    return openReminders.value.filter((r) => {
      if (r.is_reminded) return false
      const due = new Date(r.scheduled_end_at ?? r.scheduled_at ?? r.remind_at ?? r.created_at).getTime()
      return due <= endOfToday.getTime() && !Number.isNaN(due)
    }).length
  })

  async function fetchReminders() {
    loading.value = true
    try {
      const res = await api.listReminders()
      reminders.value = res.data
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

  return { reminders, loading, pendingCount, openReminders, fetchReminders, dismissReminder, snoozeReminder }
})