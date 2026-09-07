import { ref, shallowRef, watch } from 'vue'
import { toast } from 'vue-sonner'
import type { Lead } from '@/stores/leads'
import type { PrefillContact, LeadSaveBody } from '@/components/leads/LeadForm.vue'
import { createLead, updateLead, deleteLead as apiDeleteLead, openLeadConflictLead, type OpenLeadRef } from '@/api/leads'
import { errorMessage } from '@/utils/errors'

export function useLeadDrawer(onSaved: () => void) {
  const drawerOpen = shallowRef(false)
  const editingLead = ref<Lead | null>(null)
  const initialStageId = shallowRef<string | undefined>(undefined)
  const prefillContact = ref<PrefillContact | null>(null)
  const saving = shallowRef(false)
  // openLeadConflict is the existing open lead from a refused create (409);
  // the form renders it as the resolve-or-log banner. Cleared whenever the
  // drawer closes so the next create starts clean.
  const openLeadConflict = ref<OpenLeadRef | null>(null)

  watch(drawerOpen, (open) => {
    if (!open) openLeadConflict.value = null
  })

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
        await updateLead(editingLead.value.id, body)
        toast.success('Lead updated')
      } else {
        await createLead(body)
        if (body.new_contact) {
          toast.success('Contact created & linked')
        }
        toast.success('Lead created')
      }
      drawerOpen.value = false
      onSaved()
    } catch (e) {
      // A create refused because the slot is already held (409) is not an
      // error toast: the banner offers the resolve-or-log path instead. An
      // edit refused the same way keeps the toast — the banner's log-enquiry
      // framing only fits lead entry, so a failed edit must not be swallowed.
      const conflictLead = editingLead.value ? null : openLeadConflictLead(e)
      if (conflictLead) {
        openLeadConflict.value = conflictLead
        return
      }
      toast.error(errorMessage(e, 'Failed to save lead'))
    } finally {
      saving.value = false
    }
  }

  // enquiryLogged closes the drawer after the one-tap [Log enquiry] wrote the
  // touchpoint on the existing open lead, then refreshes the board.
  function handleEnquiryLogged() {
    drawerOpen.value = false
    onSaved()
  }

  async function deleteLead(leadId: string) {
    try {
      await apiDeleteLead(leadId)
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
    openLeadConflict,
    openCreate,
    openEdit,
    handleSave,
    handleEnquiryLogged,
    deleteLead,
  }
}
