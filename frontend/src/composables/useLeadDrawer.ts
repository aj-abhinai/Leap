import { ref, shallowRef } from 'vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import type { Lead } from '@/stores/leads'
import type { PrefillContact, LeadSaveBody } from '@/components/leads/LeadForm.vue'
import { errorMessage } from '@/utils/errors'

export function useLeadDrawer(onSaved: () => void) {
  const drawerOpen = shallowRef(false)
  const editingLead = ref<Lead | null>(null)
  const initialStageId = shallowRef<string | undefined>(undefined)
  const prefillContact = ref<PrefillContact | null>(null)
  const saving = shallowRef(false)

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

  async function handleSave(body: LeadSaveBody) {
    saving.value = true
    try {
      if (editingLead.value) {
        await apiClient.patch(`/api/leads/${editingLead.value.id}`, body)
        toast.success('Lead updated')
      } else {
        await apiClient.post('/api/leads', body)
        if (body.new_contact) {
          toast.success('Contact created & linked')
        }
        toast.success('Lead created')
      }
      drawerOpen.value = false
      onSaved()
    } catch (e) {
      toast.error(errorMessage(e, 'Failed to save lead'))
    } finally {
      saving.value = false
    }
  }

  async function deleteLead(leadId: string) {
    try {
      await apiClient.delete(`/api/leads/${leadId}`)
      toast.success('Lead deleted')
      drawerOpen.value = false
      onSaved()
    } catch (e) {
      toast.error(errorMessage(e, 'Failed to delete lead'))
    }
  }

  return {
    drawerOpen,
    editingLead,
    initialStageId,
    prefillContact,
    saving,
    openCreate,
    openEdit,
    handleSave,
    deleteLead,
  }
}
