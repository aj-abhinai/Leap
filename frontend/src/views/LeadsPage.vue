<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { apiClient } from '@/composables/useApi'
import { usePipelineStore } from '@/stores/pipeline'
import { useLeadsStore, type Lead } from '@/stores/leads'
import { toast } from 'vue-sonner'
import LayoutShell from '@/components/layout/LayoutShell.vue'
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
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import LeadKanban from '@/components/leads/LeadKanban.vue'
import LeadForm, { type PrefillContact } from '@/components/leads/LeadForm.vue'
import { Plus, Layers } from '@lucide/vue'
import { toast } from 'vue-sonner'

const route = useRoute()
const pipelineStore = usePipelineStore()
const leadsStore = useLeadsStore()

const selectedPipelineId = ref('')

const selectedPipeline = computed(() =>
  pipelineStore.pipelines.find((p) => p.id === selectedPipelineId.value)
)

const kanbanColumns = computed(() => {
  if (!selectedPipeline.value?.stages) return []
  return selectedPipeline.value.stages.map((stage) => ({
    ...stage,
    leads: leadsStore.leads.filter((l) => l.stage_id === stage.id),
  }))
})

const drawerOpen = ref(false)
const editingLead = ref<Lead | null>(null)
const initialStageId = ref<string | undefined>(undefined)
const prefillContact = ref<PrefillContact | null>(null)

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

async function loadLeads() {
  if (!selectedPipelineId.value) return
  try {
    await leadsStore.fetchLeads(selectedPipelineId.value, '', 1, 100)
  } catch {
    toast.error('Failed to load leads')
  }
}

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
    loadLeads()
  } catch (e: any) {
    toast.error(e.message || 'Failed to save lead')
  }
}

async function moveStage(leadId: string, newStageId: string, previousStageId?: string) {
  try {
    await apiClient.patch(`/api/leads/${leadId}`, { stage_id: newStageId })
    toast.success('Lead moved', {
      action: previousStageId ? {
        label: 'Undo',
        onClick: async () => {
          await apiClient.patch(`/api/leads/${leadId}`, { stage_id: previousStageId })
          await loadLeads()
        },
      } : undefined,
      duration: 5000,
    })
    loadLeads()
  } catch (e: any) {
    toast.error(e.message || 'Failed to move lead')
    loadLeads()
  }
}

async function deleteLead(leadId: string) {
  try {
    await apiClient.delete(`/api/leads/${leadId}`)
    toast.success('Lead deleted')
    drawerOpen.value = false
    loadLeads()
  } catch (e: any) {
    toast.error(e.message || 'Failed to delete lead')
  }
}
</script>

<template>
  <LayoutShell>
    <div class="flex flex-1 flex-col gap-4 p-6 pt-2">
      <div class="flex items-center gap-2">
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
        <Sheet v-model:open="drawerOpen">
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
              @save="handleSave"
              @delete="deleteLead"
            />
          </SheetContent>
        </Sheet>
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
        @move-stage="moveStage"
      />
    </div>
  </LayoutShell>
</template>
