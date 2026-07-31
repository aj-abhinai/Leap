import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { apiClient } from '@/composables/useApi'

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
  is_reminded: boolean
  created_at: string
}

export const useRemindersStore = defineStore('reminders', () => {
  const reminders = ref<Reminder[]>([])
  const loading = ref(false)

  const pendingCount = computed(() => reminders.value.length)

  async function fetchReminders() {
    loading.value = true
    try {
      const res = await apiClient.get('/api/reminders')
      reminders.value = res.data
    } finally {
      loading.value = false
    }
  }

  async function dismissReminder(id: string) {
    await apiClient.patch(`/api/reminders/${id}`)
    await fetchReminders()
  }

  return { reminders, loading, pendingCount, fetchReminders, dismissReminder }
})
