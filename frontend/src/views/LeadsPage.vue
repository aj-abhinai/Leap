<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { apiClient } from '@/composables/useApi'
import { usePipelineStore } from '@/stores/pipeline'
import { useLeadsStore, type Lead } from '@/stores/leads'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Button } from '@/components/ui/button'
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
import { Plus } from '@lucide/vue'

const pipelineStore = usePipelineStore()
const leadsStore = useLeadsStore()

const selectedPipelineId = ref('')
const loading = ref(false)

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

onMounted(async () => {
  await pipelineStore.fetchPipelines()
  if (pipelineStore.pipelines.length > 0) {
    selectedPipelineId.value = pipelineStore.pipelines[0].id
    loadLeads()
  }
})

async function loadLeads() {
  if (!selectedPipelineId.value) return
  loading.value = true
  try {
    await leadsStore.fetchLeads(selectedPipelineId.value, '', 1, 100)
  } finally {
    loading.value = false
  }
}

function openCreate(stageId?: string) {
  editingLead.value = null
  initialStageId.value = stageId
  drawerOpen.value = true
}

function openEdit(lead: Lead) {
  editingLead.value = lead
  initialStageId.value = undefined
  drawerOpen.value = true
}

async function handleSave(body: Record<string, any>) {
  if (editingLead.value) {
    await apiClient.patch(`/api/leads/${editingLead.value.id}`, body)
  } else {
    await apiClient.post('/api/leads', body)
  }
  drawerOpen.value = false
  loadLeads()
}

async function moveStage(leadId: string, newStageId: string) {
  try {
    await apiClient.patch(`/api/leads/${leadId}`, { stage_id: newStageId })
    loadLeads()
  } catch {}
}

async function deleteLead(leadId: string) {
  try {
    await apiClient.delete(`/api/leads/${leadId}`)
    drawerOpen.value = false
    loadLeads()
  } catch {}
}

</script>

<template>
  <LayoutShell>
    <div class="flex flex-1 flex-col gap-4 p-4 pt-0">
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
              :editing-lead="editingLead"
              :stages="selectedPipeline?.stages || []"
              :pipeline-id="selectedPipelineId"
              :initial-stage-id="initialStageId"
              @save="handleSave"
              @delete="deleteLead"
            />
          </SheetContent>
        </Sheet>
      </div>
      <div v-if="loading" class="text-sm text-muted-foreground">Loading...</div>
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
