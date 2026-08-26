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

  const pendingCount = computed(() => openReminders.value.filter((r) => !r.is_reminded).length)

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