import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'

export interface Reminder {
  id: string
  lead_id: string
  stage_id: string
  stage_name?: string
  user_id?: string
  user_name?: string
  type: string
  description: string
  scheduled_at?: string
  remind_at?: string
  is_done: boolean
  is_cancelled: boolean
  is_reminded: boolean
  created_at: string
}

export const useRemindersStore = defineStore('reminders', () => {
  const reminders = ref<Reminder[]>([])
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
      const res = await apiClient.get('/api/reminders')
      reminders.value = res.data
    } finally {
      loading.value = false
    }
  }

  async function dismissReminder(reminder: Reminder) {
    try {
      await apiClient.patch(`/api/leads/${reminder.lead_id}/reminders/${reminder.id}`)
      toast.success('Reminder dismissed')
    } catch (e: any) {
      toast.error(e.message || 'Failed to dismiss reminder')
    } finally {
      await fetchReminders()
    }
  }

  async function snoozeReminder(reminder: Reminder, remindAt: string) {
    try {
      await apiClient.post(`/api/leads/${reminder.lead_id}/reminders/${reminder.id}/snooze`, {
        remind_at: remindAt,
      })
      toast.success('Reminder snoozed')
    } catch (e: any) {
      toast.error(e.message || 'Failed to snooze reminder')
    } finally {
      await fetchReminders()
    }
  }

  return { reminders, loading, pendingCount, openReminders, fetchReminders, dismissReminder, snoozeReminder }
})
