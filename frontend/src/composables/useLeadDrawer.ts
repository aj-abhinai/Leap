import { ref, shallowRef } from 'vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import type { Lead } from '@/stores/leads'
import type { PrefillContact } from '@/components/leads/LeadForm.vue'

export function useLeadDrawer(onSaved: () => void) {
  const drawerOpen = shallowRef(false)
  const editingLead = ref<Lead | null>(null)
  const initialStageId = shallowRef<string | undefined>(undefined)
  const prefillContact = ref<PrefillContact | null>(null)

  function openCreate(stageId?: string) {
    editingLead.value = null
    prefillContact.value = null
    initialStageId.value = stageId
    drawerOpen.value = true
  }

  function openEdit(lead: Lead) {
    editingLead.value = lead
    initialStageId.value = undefined
    drawerOpen.value = true
  }

  async function handleSave(body: Record<string, any>) {
    try {
      if (editingLead.value) {
        await apiClient.patch(`/api/leads/${editingLead.value.id}`, body)
        toast.success('Lead updated')
      } else {
        await apiClient.post('/api/leads', body)
        toast.success('Lead created')
      }
      drawerOpen.value = false
      onSaved()
    } catch (e: any) {
      toast.error(e.message || 'Failed to save lead')
    }
  }

  async function deleteLead(leadId: string) {
    try {
      await apiClient.delete(`/api/leads/${leadId}`)
      toast.success('Lead deleted')
      drawerOpen.value = false
      onSaved()
    } catch (e: any) {
      toast.error(e.message || 'Failed to delete lead')
    }
  }

  return {
    drawerOpen,
    editingLead,
    initialStageId,
    prefillContact,
    openCreate,
    openEdit,
    handleSave,
    deleteLead,
  }
}
