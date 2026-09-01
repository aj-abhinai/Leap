<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { errorMessage } from '@/utils/errors'
import { useRBACStore } from '@/stores/rbac'
import { usePipelineStore } from '@/stores/pipeline'
import { useLeadPipeline } from '@/composables/useLeadPipeline'
import { useLeadDrawerGlobal } from '@/composables/useLeadDrawerGlobal'
import { useLeadsStore } from '@/stores/leads'
import { getLead, type Lead } from '@/api/leads'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { BookOpen, Plus } from '@lucide/vue'
import { formatCurrency, formatContactDetail } from '@/utils/format'
import LeadActivityForm from './LeadActivityForm.vue'
import LeadActivity from './LeadActivity.vue'
import LeadStageHistory from './LeadStageHistory.vue'

const router = useRouter()
const rbac = useRBACStore()
const pipelineStore = usePipelineStore()
const leadsStore = useLeadsStore()
const pipeline = useLeadPipeline()
const { drawerOpen, drawerLeadId, drawerLead, closeLeadDrawer } = useLeadDrawerGlobal()

const lead = shallowRef<Lead | null>(null)
const loading = shallowRef(false)

// Fetch the lead when opened without a preloaded object.
watch(
  () => drawerLeadId.value,
  async (id) => {
    if (!id) return
    if (drawerLead.value) {
      lead.value = drawerLead.value
      if (pipelineStore.pipelines.length === 0) pipelineStore.fetchPipelines()
      return
    }
    loading.value = true
    try {
      const res = await getLead(id)
      lead.value = res.data
      if (pipelineStore.pipelines.length === 0) pipelineStore.fetchPipelines()
    } catch (e) {
      toast.error(errorMessage(e, 'Failed to load lead'))
      closeLeadDrawer()
    } finally {
      loading.value = false
    }
  },
  { immediate: true },
)

// A lead is closed when it sits in a closing stage; the stage's outcome is
// authoritative rather than the possibly-stale leads.outcome.
function isClosed() {
  return lead.value?.stage_outcome === 'won' || lead.value?.stage_outcome === 'lost'
}

// A close_lost quick reply is executed by the backend in the same transaction
// as the activity save (log + stage move + task cancellation); the drawer
// only needs to refresh so the kanban card lands in the lost column.
async function handleCloseLost() {
  pipeline.loadLeads()
  if (!pipeline.selectedPipelineId.value && pipelineStore.pipelines.length > 0) {
    await leadsStore.fetchAllLeads({ pipelineId: pipelineStore.pipelines[0].id })
  }
  closeLeadDrawer()
}

// Saving a task closes the drawer and reloads the board so the kanban card's
// next-task/last-touch preview reflects the new activity immediately.
function handleSaved() {
  pipeline.loadLeads()
  closeLeadDrawer()
}

// Inline task mutations (mark done, snooze, edit, delete) keep the drawer
// open so the user can keep working, but the board must reload so the card's
// next-task/last-touch preview is not stale.
function handleTasksChanged() {
  pipeline.loadLeads()
}

// Repeat business: a closed contact may re-enquire any time (same day, next
// year, same or different program). One click opens the new-lead form on the
// Leads page prefilled with the same contact.
function newLeadForContact() {
  const l = lead.value
  if (!l?.contact_id) return
  closeLeadDrawer()
  router.push({ name: 'Leads', query: { contact: l.contact_id } })
}
</script>

<template>
  <Sheet v-model:open="drawerOpen">
    <SheetContent class="p-0 sm:max-w-lg">
      <SheetHeader class="border-b px-6 py-4">
        <SheetDescription class="sr-only">
          Lead activities, timeline, and stage history for this lead.
        </SheetDescription>
        <div v-if="loading" class="space-y-2">
          <Skeleton class="h-5 w-40" />
          <Skeleton class="h-4 w-64" />
        </div>
        <template v-else-if="lead">
          <div class="flex items-center justify-between gap-2">
            <SheetTitle class="truncate">{{ lead.display_name }}</SheetTitle>
            <Button
              v-if="isClosed() && rbac.can('lead:write')"
              variant="outline"
              size="sm"
              class="shrink-0"
              title="Start a new enquiry for this contact"
              @click="newLeadForContact"
            >
              <Plus class="mr-1 size-3.5" /> New lead
            </Button>
          </div>
          <div class="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            <Badge v-if="lead.stage_name" variant="secondary" class="text-xs">
              {{ lead.stage_name }}
            </Badge>
            <Badge v-if="lead.stage_outcome === 'won'" class="text-xs">Won</Badge>
            <Badge v-if="lead.stage_outcome === 'lost'" variant="destructive" class="text-xs">Lost</Badge>
            <span v-if="lead.program_name" class="flex items-center gap-1">
              <BookOpen class="size-3" /> {{ lead.program_name }}
            </span>
            <span v-if="lead.value" class="font-semibold text-primary tabular-nums">
              {{ formatCurrency(lead.value) }}
            </span>
            <span v-if="lead.contact_phone || lead.contact_email">
              {{ formatContactDetail(lead.contact_phone, lead.contact_email) }}
            </span>
          </div>
        </template>
      </SheetHeader>
      <div v-if="lead" class="flex-1 space-y-4 overflow-y-auto px-6 py-4">
        <LeadActivityForm
          v-if="rbac.can('lead:write')"
          :lead-id="lead.id!"
          @saved="handleSaved"
          @close-lost="handleCloseLost"
        />
        <LeadActivity :lead-id="lead.id!" @close-lost="handleCloseLost" @tasks-changed="handleTasksChanged" />
        <LeadStageHistory :lead-id="lead.id!" />
      </div>
    </SheetContent>
  </Sheet>
</template>
