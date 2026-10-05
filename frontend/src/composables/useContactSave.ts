import { shallowRef } from 'vue'
import { toast } from 'vue-sonner'
import { useContactsStore } from '@/stores/contacts'
import { ApiError } from '@/composables/useApi'
import { errorMessage } from '@/utils/errors'
import type { Contact, ContactSaveBody, DuplicateMatch } from '@/api/contacts'

// SavedContact is the stored contact returned by a successful create or
// update; a create may carry advisory warnings (for example, a confirmed
// duplicate).
export type SavedContact = Contact & { warnings?: string[] }

// useContactSave owns the contact save lifecycle, including the 409
// duplicate confirm flow. save resolves to the saved contact, or null when a
// duplicate was surfaced in duplicateMatches (the caller renders the dialog)
// or the failure was toasted; refetching or reloading stays the caller's job.
// resolveDuplicates re-submits against the id and body frozen when the 409
// arrived, so navigation between duplicate and confirm cannot target another
// contact.
export function useContactSave() {
  const saving = shallowRef(false)
  const duplicateMatches = shallowRef<DuplicateMatch[] | null>(null)
  let pending: { id: string | null; body: ContactSaveBody } | null = null

  // save creates the contact when id is absent and updates it otherwise.
  async function save(body: ContactSaveBody, id?: string): Promise<SavedContact | null> {
    duplicateMatches.value = null
    pending = { id: id ?? null, body }
    saving.value = true
    try {
      const store = useContactsStore()
      return id ? await store.update(id, body) : await store.create(body)
    } catch (e) {
      if (e instanceof ApiError && e.code === 'DUPLICATE_CONTACT') {
        // The payload is the response envelope; the matches travel under data.
        duplicateMatches.value = e.payload?.data?.duplicates ?? []
        return null
      }
      toast.error(errorMessage(e, 'Failed to save contact'))
      return null
    } finally {
      saving.value = false
    }
  }

  // resolveDuplicates completes a paused duplicate flow: true re-submits the
  // frozen body with confirm_duplicates set, false just closes the dialog.
  async function resolveDuplicates(confirmed: boolean): Promise<SavedContact | null> {
    const p = pending
    duplicateMatches.value = null
    pending = null
    if (!confirmed || !p) return null
    return save({ ...p.body, confirm_duplicates: true }, p.id ?? undefined)
  }

  // reset drops a paused flow when the drawer closes or the route changes.
  function reset() {
    pending = null
    duplicateMatches.value = null
  }

  return { saving, duplicateMatches, save, resolveDuplicates, reset }
}
