<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { apiClient } from '@/composables/useApi'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { Badge } from '@/components/ui/badge'
import LeadKanban from '@/components/leads/LeadKanban.vue'
import LeadForm from '@/components/leads/LeadForm.vue'
import LeadActivity from '@/components/leads/LeadActivity.vue'
import LeadActivityForm from '@/components/leads/LeadActivityForm.vue'
import { useRBACStore } from '@/stores/rbac'
import { toast } from 'vue-sonner'
import { Plus, Layers, BookOpen } from '@lucide/vue'
import { formatCurrency, formatContactDetail } from '@/utils/format'
import type { LeadSaveBody } from '@/components/leads/LeadForm.vue'
import { useLeadPipeline } from '@/composables/useLeadPipeline'
import { useLeadDrawer } from '@/composables/useLeadDrawer'
import { useActivityDrawer } from '@/composables/useActivityDrawer'

const route = useRoute()
const rbac = useRBACStore()

const {
  pipelineStore,
  leadsStore,
  selectedPipelineId,
  selectedPipeline,
  kanbanColumns,
  loadLeads,
  moveStage,
  bulkMoveStage,
} = useLeadPipeline()

const {
  drawerOpen,
  editingLead,
  initialStageId,
  prefillContact,
  saving,
  openCreate,
  openEdit,
  handleSave,
  deleteLead,
} = useLeadDrawer(loadLeads)

const {
  activityDrawerOpen,
  activityLead,
  activityRef,
  openActivities,
} = useActivityDrawer()

// A close_lost quick reply on an activity logs the reply then moves the lead to
// its pipeline's closing stage (prefer a "lost"-named stage, else the first
// closing stage). Reaching a closing stage resolves the deal and cancels open
// tasks, so the flow "ends there" for the lead.
async function handleCloseLost() {
  if (!activityLead.value?.id) return
  const lead = activityLead.value
  const pipeline = pipelineStore.pipelines.find((p) => p.id === lead.pipeline_id)
  const stages = pipeline?.stages || []
  const closing = stages.filter((s) => s.is_closing)
  const target =
    closing.find((s) => /lost/i.test(s.name)) ||
    closing.find((s) => /closed/i.test(s.name)) ||
    closing[0]
  if (!target) {
    toast.error('No closing stage available in this pipeline')
    return
  }
  if (lead.stage_id === target.id) return
  await moveStage(lead.id, target.id, lead.stage_id)
  activityDrawerOpen.value = false
}

// After a lead edit/delete from the activity drawer, the drawer header would
// keep stale stage/value/contact data. Re-sync from the freshly reloaded
// kanban columns; close the drawer if the lead no longer exists.
function syncActivityLead() {
  const id = activityLead.value?.id
  if (!id) return
  const fresh = kanbanColumns.value.flatMap((c) => c.leads).find((l) => l.id === id)
  if (fresh) activityLead.value = fresh
  else activityDrawerOpen.value = false
}

async function onLeadSaved(body: LeadSaveBody) {
  await handleSave(body)
  syncActivityLead()
}

async function onLeadDeleted(leadId: string) {
  await deleteLead(leadId)
  syncActivityLead()
}

onMounted(async () => {
  await pipelineStore.fetchPipelines()
  if (pipelineStore.pipelines.length > 0) {
    selectedPipelineId.value = pipelineStore.pipelines[0].id
    loadLeads()
  }
  const contactIdQuery = route.query.contact as string | undefined
  const contactId = contactIdQuery || route.query.contact_id as string | undefined
  if (contactId) {
    try {
      const res = await apiClient.get(`/api/contacts/${contactId}`)
      const c = res.data as { id: string; name: string; email?: string; phone?: string }
      prefillContact.value = {
        id: c.id,
        name: c.name,
        email: c.email,
        phone: c.phone,
      }
      if (pipelineStore.pipelines.length > 0 && pipelineStore.pipelines[0].stages?.length) {
        initialStageId.value = pipelineStore.pipelines[0].stages[0].id
      }
      drawerOpen.value = true
    } catch {}
  }
})
</script>

<template>
  <div class="flex min-w-0 flex-1 flex-col gap-4 p-6 pt-2">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="flex min-w-0 flex-col">
        <h1 class="text-2xl font-semibold tracking-tight">Leads</h1>
        <p v-if="selectedPipeline" class="mt-0.5 text-sm text-muted-foreground">
          {{ selectedPipeline.name }}
          <span class="tabular-nums text-muted-foreground/70">{{ kanbanColumns.reduce((n, c) => n + c.leads.length, 0) }} leads</span>
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <Select v-model="selectedPipelineId" @update:model-value="loadLeads()">
          <SelectTrigger class="w-48">
            <SelectValue placeholder="Select pipeline" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem
              v-for="p in pipelineStore.pipelines"
              :key="p.id"
              :value="p.id"
            >
              {{ p.name }}
            </SelectItem>
          </SelectContent>
        </Select>
        <Sheet v-if="rbac.can('lead:write')" v-model:open="drawerOpen">
          <SheetTrigger as-child>
            <Button @click="openCreate()">
              <Plus class="mr-2 size-4" /> Add Lead
            </Button>
          </SheetTrigger>
          <SheetContent>
            <SheetHeader>
              <SheetTitle>{{ editingLead ? 'Edit Lead' : 'Add Lead' }}</SheetTitle>
              <SheetDescription>Enter lead details below.</SheetDescription>
            </SheetHeader>
            <LeadForm
              :key="editingLead?.id ?? 'create'"
              :editing-lead="editingLead"
              :stages="selectedPipeline?.stages || []"
              :pipeline-id="selectedPipelineId"
              :initial-stage-id="initialStageId"
              :prefill-contact="prefillContact"
              :saving="saving"
              @save="onLeadSaved"
              @delete="onLeadDeleted"
            />
          </SheetContent>
        </Sheet>

        <Sheet v-model:open="activityDrawerOpen">
          <SheetContent class="p-0 sm:max-w-lg">
            <SheetHeader class="border-b px-6 py-4">
              <SheetTitle class="truncate">{{ activityLead?.display_name ?? 'Activities' }}</SheetTitle>
              <div v-if="activityLead" class="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                <Badge v-if="activityLead.stage_name" variant="secondary" class="text-xs">
                  {{ activityLead.stage_name }}
                </Badge>
                <Badge v-if="activityLead.outcome === 'won'" class="text-xs">Won</Badge>
                <Badge v-if="activityLead.outcome === 'lost'" variant="destructive" class="text-xs">Lost</Badge>
                <span v-if="activityLead.program_name" class="flex items-center gap-1">
                  <BookOpen class="size-3" /> {{ activityLead.program_name }}
                </span>
                <span v-if="activityLead.value" class="font-semibold text-primary tabular-nums">
                  {{ formatCurrency(activityLead.value) }}
                </span>
                <span v-if="activityLead.contact_phone || activityLead.contact_email">
                  {{ formatContactDetail(activityLead.contact_phone, activityLead.contact_email) }}
                </span>
              </div>
            </SheetHeader>
            <div v-if="activityLead" class="flex-1 space-y-4 overflow-y-auto px-6 py-4">
              <LeadActivityForm
                v-if="rbac.can('lead:write')"
                :lead-id="activityLead.id!"
                @saved="activityRef?.fetchActivities()"
                @close-lost="handleCloseLost"
              />
              <LeadActivity ref="activityRef" :lead-id="activityLead.id!" @close-lost="handleCloseLost" />
            </div>
          </SheetContent>
        </Sheet>
      </div>
    </div>

    <div v-if="leadsStore.loading" class="flex gap-4 overflow-x-auto pb-4">
      <div v-for="i in 4" :key="i" class="min-w-64 flex-1 rounded-lg border bg-muted/30 p-4 space-y-3">
        <Skeleton class="h-5 w-24" />
        <Skeleton class="h-4 w-full" />
        <Skeleton class="h-4 w-3/4" />
        <Skeleton class="h-4 w-1/2" />
      </div>
    </div>

    <div v-else-if="kanbanColumns.length === 0" class="flex flex-col items-center justify-center py-16 text-center">
      <Layers class="size-12 text-muted-foreground/30 mb-4" />
      <p class="text-sm font-medium text-muted-foreground">No pipelines configured</p>
      <p class="text-xs text-muted-foreground/60 mt-1">Create a pipeline in Settings to get started</p>
    </div>

    <LeadKanban
      v-else
      :columns="kanbanColumns"
      :stages="selectedPipeline?.stages || []"
      :pipeline-id="selectedPipelineId"
      @create="openCreate"
      @edit="openEdit"
      @view-activities="openActivities"
      @move-stage="moveStage"
      @bulk-move="bulkMoveStage"
      @stage-added="async () => { await pipelineStore.fetchPipelines(); loadLeads() }"
    />
  </div>
</template>
