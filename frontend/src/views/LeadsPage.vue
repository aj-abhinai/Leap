<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Input } from '@/components/ui/input'
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
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import LeadKanban from '@/components/leads/LeadKanban.vue'
import LeadForm from '@/components/leads/LeadForm.vue'
import { useRBACStore } from '@/stores/rbac'
import { useUsersStore } from '@/stores/users'
import { toast } from 'vue-sonner'
import { Plus, Layers, Search } from '@lucide/vue'
import type { LeadSaveBody } from '@/components/leads/LeadForm.vue'
import { useLeadPipeline } from '@/composables/useLeadPipeline'
import { useLeadDrawer } from '@/composables/useLeadDrawer'
import { useLeadDrawerGlobal } from '@/composables/useLeadDrawerGlobal'
import { debounce } from '@/utils/debounce'
import { getContact } from '@/api/contacts'

const route = useRoute()
const rbac = useRBACStore()
const users = useUsersStore()

const {
  pipelineStore,
  selectedPipelineId,
  selectedPipeline,
  kanbanColumns,
  loading,
  loadLeads,
  moveStage,
  bulkMoveStage,
  search,
  outcomeFilter,
  assigneeFilter,
  fromDate,
  toDate,
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

const { openLeadDrawer } = useLeadDrawerGlobal()

const totalShown = computed(() => kanbanColumns.value.reduce((n, c) => n + c.leads.length, 0))

const outcomeOptions = [
  { value: '', label: 'All' },
  { value: 'open', label: 'Open' },
  { value: 'won', label: 'Won' },
  { value: 'lost', label: 'Lost' },
] as const

// Debounce the text search; outcome/assignee/date filters apply immediately.
const debouncedLoad = debounce(() => loadLeads(), 300)
watch(search, debouncedLoad)
watch([outcomeFilter, assigneeFilter, fromDate, toDate], () => loadLeads())

async function onLeadSaved(body: LeadSaveBody) {
  await handleSave(body)
}

async function onLeadDeleted(leadId: string) {
  await deleteLead(leadId)
}

// Opens the create form prefilled with an existing contact (used by the
// "New lead for this contact" action on closed leads). Reactive so it also
// works when already on this page with a new ?contact= query.
async function handleContactPrefill(contactId?: string) {
  if (!contactId) return
  try {
    const res = await getContact(contactId)
    const c = res.data as { id: string; name: string; email?: string; phone?: string }
    editingLead.value = null
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

onMounted(async () => {
  await pipelineStore.fetchPipelines()
  users.fetchOptions()
  if (pipelineStore.pipelines.length > 0) {
    selectedPipelineId.value = pipelineStore.pipelines[0].id
    loadLeads()
  }
  const contactIdQuery = route.query.contact as string | undefined
  await handleContactPrefill(contactIdQuery || (route.query.contact_id as string | undefined))
})

watch(
  () => route.query.contact as string | undefined,
  (id) => {
    if (id) handleContactPrefill(id)
  },
)
</script>

<template>
  <div class="flex min-w-0 flex-1 flex-col gap-4 p-6 pt-2">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="flex min-w-0 flex-col">
        <h1 class="text-2xl font-semibold tracking-tight">Leads</h1>
        <p v-if="selectedPipeline" class="mt-0.5 text-sm text-muted-foreground">
          {{ selectedPipeline.name }}
          <span class="tabular-nums text-muted-foreground/70">
            {{ totalShown }} leads shown
          </span>
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <div class="relative">
          <Search class="absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input v-model="search" placeholder="Search name, phone, email…" class="h-9 w-56 pl-8" />
        </div>

        <div class="flex items-center gap-0.5 rounded-md border p-0.5" role="group" aria-label="Outcome filter">
          <button
            v-for="opt in outcomeOptions"
            :key="opt.value"
            type="button"
            class="rounded px-2.5 py-1 text-xs transition-colors"
            :class="outcomeFilter === opt.value ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-accent'"
            @click="outcomeFilter = opt.value"
          >
            {{ opt.label }}
          </button>
        </div>

        <Select v-model="assigneeFilter">
          <SelectTrigger class="h-9 w-44">
            <SelectValue placeholder="Assignee" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All assignees</SelectItem>
            <SelectItem value="none">Unassigned</SelectItem>
            <SelectItem v-for="u in users.options" :key="u.id" :value="u.id">
              {{ u.name }}
            </SelectItem>
          </SelectContent>
        </Select>

        <div class="flex items-center gap-1.5">
          <Input v-model="fromDate" type="date" class="h-9 w-40" aria-label="From date" title="From date" />
          <span class="text-muted-foreground">–</span>
          <Input v-model="toDate" type="date" class="h-9 w-40" aria-label="To date" title="To date" />
        </div>

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
      </div>
    </div>

    <div v-if="loading" class="flex gap-4 overflow-x-auto pb-4">
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
      @view-activities="(lead) => openLeadDrawer(lead.id!, lead)"
      @move-stage="moveStage"
      @bulk-move="bulkMoveStage"
      @stage-added="async () => { await pipelineStore.fetchPipelines(); loadLeads() }"
    />
  </div>
</template>
