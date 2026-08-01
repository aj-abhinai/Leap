import { ref, shallowRef } from 'vue'
import type { Lead } from '@/stores/leads'
import LeadActivity from '@/components/leads/LeadActivity.vue'

export function useActivityDrawer() {
  const activityDrawerOpen = shallowRef(false)
  const activityLead = ref<Lead | null>(null)
  const activityRef = shallowRef<InstanceType<typeof LeadActivity> | null>(null)

  function openActivities(lead: Lead) {
    activityLead.value = lead
    activityDrawerOpen.value = true
  }

  return {
    activityDrawerOpen,
    activityLead,
    activityRef,
    openActivities,
  }
}
