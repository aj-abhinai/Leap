import { ref, shallowRef, watch } from 'vue'
import type { Lead } from '@/stores/leads'

// App-level singleton state for the lead activities drawer. Any view can open
// the same drawer over the current page (no navigation), keeping the user's
// place and the lead's full context in one click.
const drawerOpen = shallowRef(false)
const drawerLeadId = shallowRef('')
const drawerLead = ref<Lead | null>(null)

// Clear the cached lead whenever the drawer closes. The Sheet binds v-model:open
// straight to drawerOpen, so its own close controls (X, Escape, overlay) bypass
// closeLeadDrawer; without this, reopening the same lead id would skip the
// refetch and show a stale copy (e.g. after the lead's stage/outcome changed).
watch(drawerOpen, (open) => {
  if (!open) {
    drawerLeadId.value = ''
    drawerLead.value = null
  }
})

export function useLeadDrawerGlobal() {
  function openLeadDrawer(leadId: string, lead?: Lead) {
    drawerLeadId.value = leadId
    drawerLead.value = lead ?? null
    drawerOpen.value = true
  }

  function closeLeadDrawer() {
    drawerOpen.value = false
    // Clear both so reopening the same lead id always refetches instead of
    // showing a stale copy (e.g. after the lead's stage/outcome changed).
    drawerLeadId.value = ''
    drawerLead.value = null
  }

  return {
    drawerOpen,
    drawerLeadId,
    drawerLead,
    openLeadDrawer,
    closeLeadDrawer,
  }
}
