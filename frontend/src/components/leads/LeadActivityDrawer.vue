<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import { errorMessage } from '@/utils/errors'
import { useRBACStore } from '@/stores/rbac'
import { usePipelineStore } from '@/stores/pipeline'
import { useLeadPipeline } from '@/composables/useLeadPipeline'
import { useLeadDrawerGlobal } from '@/composables/useLeadDrawerGlobal'
import type { Lead } from '@/stores/leads'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
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
const pipeline = useLeadPipeline()
const { drawerOpen, drawerLeadId, drawerLead, closeLeadDrawer } = useLeadDrawerGlobal()

const lead = shallowRef<Lead | null>(null)
const loading = shallowRef(false)
const activityRef = shallowRef<InstanceType<typeof LeadActivity> | null>(null)

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
      const res = await apiClient.get<Lead>(`/api/leads/${id}`)
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

function isClosed() {
  return !!lead.value?.outcome
}

// A close_lost quick reply on an activity logs the reply then moves the lead to
// its pipeline's lost closing stage (declared via stage outcome metadata, so no
// name guessing). Reaching a closing stage resolves the deal and cancels open
// tasks, so the flow "ends there" for the lead.
async function handleCloseLost() {
  const l = lead.value
  if (!l?.id) return
  if (pipelineStore.pipelines.length === 0) await pipelineStore.fetchPipelines()
  const pl = pipelineStore.pipelines.find((p) => p.id === l.pipeline_id)
  const stages = pl?.stages || []
  const target = stages.find((s) => s.is_closing && s.outcome === 'lost')
  if (!target) {
    toast.error('No closing "lost" stage configured in this pipeline')
    return
  }
  if (l.stage_id === target.id) {
    closeLeadDrawer()
    return
  }
  await pipeline.moveStage(l.id, target.id, l.stage_id)
  closeLeadDrawer()
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
            <Badge v-if="lead.outcome === 'won'" class="text-xs">Won</Badge>
            <Badge v-if="lead.outcome === 'lost'" variant="destructive" class="text-xs">Lost</Badge>
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
          @saved="activityRef?.fetchActivities()"
          @close-lost="handleCloseLost"
        />
        <LeadActivity ref="activityRef" :lead-id="lead.id!" @close-lost="handleCloseLost" />
        <LeadStageHistory :lead-id="lead.id!" />
      </div>
    </SheetContent>
  </Sheet>
</template>
