<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
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
import { Plus, Layers, Search, X } from '@lucide/vue'
import type { LeadSaveBody } from '@/components/leads/LeadForm.vue'
import { useLeadPipeline } from '@/composables/useLeadPipeline'
import { useLeadDrawer } from '@/composables/useLeadDrawer'
import { useLeadDrawerGlobal } from '@/composables/useLeadDrawerGlobal'
import { debounce } from '@/utils/debounce'
import { getContact } from '@/api/contacts'

const route = useRoute()
const router = useRouter()
const rbac = useRBACStore()
const users = useUsersStore()

const {
  pipelineStore,
  selectedPipelineId,
  selectedPipeline,
  kanbanColumns,
  loading,
  hasBoard,
  boardRevision,
  search,
  outcomeFilter,
  assigneeFilter,
  fromDate,
  toDate,
  activeFilterCount,
  clearFilters,
  syncPipelineSelection,
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
  openLeadConflict,
  openCreate,
  openEdit,
  handleSave,
  handleEnquiryLogged,
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

// Press / anywhere (except inside an input or an open dialog) to jump to
// search. Dialogs trap focus, so the shortcut stays inert while one is open.
const searchInput = ref<InstanceType<typeof Input> | null>(null)
function onBeltKeydown(e: KeyboardEvent) {
  if (e.key !== '/' || e.ctrlKey || e.metaKey || e.altKey) return
  const el = e.target as HTMLElement | null
  const typing = el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)
  if (typing || document.querySelector('[role="dialog"]')) return
  e.preventDefault()
  searchInput.value?.$el.focus()
}
onMounted(() => window.addEventListener('keydown', onBeltKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onBeltKeydown))

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
    // Consume the query so refresh (or navigating back) does not re-open the
    // create drawer with the same prefill.
    router.replace({ query: {} })
  } catch {}
}

onMounted(async () => {
  // If pipelines are already loaded (revisiting the page), the watcher below
  // won't fire on population, so load here. On a cold start the watcher fires
  // when the empty list populates, making this call redundant.
  const hadPipelines = pipelineStore.pipelines.length > 0
  await pipelineStore.fetchPipelines()
  users.fetchOptions()
  syncPipelineSelection()
  if (hadPipelines) loadLeads()
  const contactIdQuery = route.query.contact as string | undefined
  await handleContactPrefill(contactIdQuery || (route.query.contact_id as string | undefined))
})

// Late-arriving pipelines (created elsewhere) select and load on arrival; a
// deleted pipeline falls back to the remembered or first one.
watch(
  () => pipelineStore.pipelines.map((p) => p.id).join(','),
  () => {
    syncPipelineSelection()
    loadLeads()
  },
)

watch(
  () => route.query.contact as string | undefined,
  (id) => {
    if (id) handleContactPrefill(id)
  },
)
</script>

<template>
  <div class="flex min-w-0 flex-1 flex-col gap-4 p-6 pt-4">
    <!-- Row 1: page header — title + count left, board actions right -->
    <div class="mx-auto flex w-4/5 flex-wrap items-center justify-between gap-3">
      <div class="flex min-w-0 items-baseline gap-2">
        <h1 class="text-xl font-semibold tracking-tight">Leads</h1>
        <span v-if="hasBoard" class="text-sm tabular-nums text-muted-foreground">
          {{ totalShown }} leads shown
        </span>
      </div>

      <div class="flex shrink-0 items-center gap-2">
        <Select v-model="selectedPipelineId" @update:model-value="loadLeads()">
          <SelectTrigger class="w-44" aria-label="Pipeline">
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
              :open-lead-conflict="openLeadConflict"
              :saving="saving"
              @save="onLeadSaved"
              @delete="onLeadDeleted"
              @enquiry-logged="handleEnquiryLogged"
            />
          </SheetContent>
        </Sheet>
      </div>
    </div>

    <!-- Row 2: filters — app-standard individual controls, aligned to the band -->
    <div class="mx-auto flex w-4/5 flex-wrap items-center gap-2">
      <div class="relative min-w-44 flex-1">
        <Search class="absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          ref="searchInput"
          v-model="search"
          type="search"
          aria-label="Search leads"
          placeholder="Search name, phone, email…"
          class="h-9 pl-8 pr-8 [&::-webkit-search-cancel-button]:hidden"
        />
        <button
          v-if="search"
          type="button"
          class="absolute top-1/2 right-2 -translate-y-1/2 rounded p-0.5 text-muted-foreground hover:text-foreground"
          :aria-label="`Clear search: ${search}`"
          @click="search = ''"
        >
          <X class="size-3.5" />
        </button>
      </div>

      <div
        role="radiogroup"
        aria-label="Outcome filter"
        class="flex items-center gap-0.5 rounded-md border p-0.5"
      >
        <button
          v-for="opt in outcomeOptions"
          :key="opt.value"
          type="button"
          role="radio"
          :aria-checked="outcomeFilter === opt.value"
          class="h-8 rounded px-2.5 text-xs transition-colors"
          :class="outcomeFilter === opt.value ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-accent'"
          @click="outcomeFilter = opt.value"
        >
          {{ opt.label }}
        </button>
      </div>

      <Select v-model="assigneeFilter">
        <SelectTrigger class="h-9 w-36" aria-label="Assignee filter">
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
        <Input id="from-date" v-model="fromDate" type="date" class="h-9 w-28" aria-label="From date" title="From date" />
        <span class="text-muted-foreground">–</span>
        <Input id="to-date" v-model="toDate" type="date" class="h-9 w-28" aria-label="To date" title="To date" />
      </div>

      <Button
        v-if="activeFilterCount > 0"
        variant="outline"
        class="h-9"
        @click="clearFilters"
      >
        <X class="size-3.5" /> Clear
      </Button>
    </div>

    <div v-if="loading && !hasBoard" class="flex gap-4 overflow-x-auto pb-4">
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
      <p class="text-xs text-muted-foreground/60 mt-1">Create a pipeline to get started</p>
      <Button
        v-if="rbac.can('settings:manage')"
        class="mt-4"
        variant="outline"
        @click="router.push('/settings')"
      >
        <Plus class="mr-2 size-4" /> Create pipeline
      </Button>
    </div>

    <div v-else class="flex min-w-0 flex-1 flex-col" :class="loading ? 'opacity-60' : ''">
      <LeadKanban
        :key="`board-${boardRevision}`"
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
  </div>
</template>
