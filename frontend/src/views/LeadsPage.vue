<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { apiClient } from '@/composables/useApi'
import { usePipelineStore, type Pipeline, type Stage } from '@/stores/pipeline'
import { type Lead } from '@/stores/leads'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Plus, MoreHorizontal, Loader2 } from '@lucide/vue'

const pipelineStore = usePipelineStore()

const selectedPipelineId = ref('')
const leads = ref<Lead[]>([])
const loading = ref(false)

const selectedPipeline = computed(() =>
  pipelineStore.pipelines.find((p) => p.id === selectedPipelineId.value)
)

const kanbanColumns = computed(() => {
  if (!selectedPipeline.value?.stages) return []
  return selectedPipeline.value.stages.map((stage) => ({
    ...stage,
    leads: leads.value.filter((l) => l.stage_id === stage.id),
  }))
})

const drawerOpen = ref(false)
const editingLead = ref<Lead | null>(null)
const formName = ref('')
const formEmail = ref('')
const formPhone = ref('')
const formValue = ref<number | undefined>(undefined)
const formNotes = ref('')
const formStageId = ref('')
const formError = ref('')
const saving = ref(false)

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
    const res = await apiClient.get(`/api/leads?pipeline_id=${selectedPipelineId.value}&per_page=100`)
    leads.value = res.data
  } finally {
    loading.value = false
  }
}

function openCreate(stageId?: string) {
  editingLead.value = null
  formName.value = ''
  formEmail.value = ''
  formPhone.value = ''
  formValue.value = undefined
  formNotes.value = ''
  formStageId.value = stageId || (selectedPipeline.value?.stages?.[0]?.id || '')
  formError.value = ''
  drawerOpen.value = true
}

function openEdit(lead: Lead) {
  editingLead.value = lead
  formName.value = lead.name
  formEmail.value = lead.email || ''
  formPhone.value = lead.phone || ''
  formValue.value = lead.value
  formNotes.value = lead.notes || ''
  formStageId.value = lead.stage_id
  formError.value = ''
  drawerOpen.value = true
}

async function handleSave() {
  formError.value = ''
  if (!formName.value) {
    formError.value = 'Name is required'
    return
  }
  saving.value = true
  try {
    const body = {
      name: formName.value,
      email: formEmail.value || null,
      phone: formPhone.value || null,
      value: formValue.value || null,
      notes: formNotes.value || null,
      pipeline_id: selectedPipelineId.value,
      stage_id: formStageId.value,
    }
    if (editingLead.value) {
      await apiClient.patch(`/api/leads/${editingLead.value.id}`, body)
    } else {
      await apiClient.post('/api/leads', body)
    }
    drawerOpen.value = false
    loadLeads()
  } catch (e: any) {
    formError.value = e.message || 'Save failed'
  } finally {
    saving.value = false
  }
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
    loadLeads()
  } catch {}
}

function formatCurrency(value?: number) {
  if (!value) return ''
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value)
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
            <div class="mt-4 space-y-4">
              <div class="space-y-2">
                <Label for="lname">Name *</Label>
                <Input id="lname" v-model="formName" placeholder="Lead name" />
              </div>
              <div class="space-y-2">
                <Label for="lemail">Email</Label>
                <Input id="lemail" v-model="formEmail" type="email" placeholder="Email" />
              </div>
              <div class="space-y-2">
                <Label for="lphone">Phone</Label>
                <Input id="lphone" v-model="formPhone" placeholder="Phone" />
              </div>
              <div class="space-y-2">
                <Label for="lvalue">Value</Label>
                <Input id="lvalue" v-model.number="formValue" type="number" placeholder="Deal value" />
              </div>
              <div class="space-y-2">
                <Label for="lnotes">Notes</Label>
                <Textarea id="lnotes" v-model="formNotes" placeholder="Notes..." rows="3" />
              </div>
              <div class="space-y-2">
                <Label>Stage</Label>
                <Select v-model="formStageId">
                  <SelectTrigger>
                    <SelectValue placeholder="Select stage" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem
                      v-for="s in selectedPipeline?.stages || []"
                      :key="s.id"
                      :value="s.id"
                    >
                      {{ s.name }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div v-if="formError" class="text-sm text-destructive">{{ formError }}</div>
              <div class="flex gap-2">
                <Button @click="handleSave" :disabled="saving" class="flex-1">
                  <Loader2 v-if="saving" class="mr-2 size-4 animate-spin" />
                  {{ editingLead ? 'Update' : 'Create' }}
                </Button>
                <Button
                  v-if="editingLead"
                  variant="destructive"
                  @click="deleteLead(editingLead.id!); drawerOpen = false"
                >
                  Delete
                </Button>
              </div>
            </div>
          </SheetContent>
        </Sheet>
      </div>
      <div v-if="loading" class="text-sm text-muted-foreground">Loading...</div>
      <div v-else class="flex gap-4 overflow-x-auto pb-4">
        <Card
          v-for="col in kanbanColumns"
          :key="col.id"
          class="min-w-64 flex-1 bg-muted/30"
        >
          <CardHeader class="flex flex-row items-center justify-between pb-2">
            <CardTitle class="text-sm font-medium">
              {{ col.name }}
              <Badge variant="secondary" class="ml-2">{{ col.leads.length }}</Badge>
            </CardTitle>
            <Button variant="ghost" size="icon" @click="openCreate(col.id)">
              <Plus class="size-4" />
            </Button>
          </CardHeader>
          <CardContent class="space-y-2">
            <div
              v-for="lead in col.leads"
              :key="lead.id"
              class="rounded-lg border bg-card p-3 text-sm shadow-sm cursor-pointer hover:shadow-md transition-shadow"
              @click="openEdit(lead)"
            >
              <div class="font-medium">{{ lead.name }}</div>
              <div v-if="lead.email" class="text-xs text-muted-foreground">{{ lead.email }}</div>
              <div v-if="lead.value" class="mt-1 font-medium text-primary">
                {{ formatCurrency(lead.value) }}
              </div>
              <div class="mt-2 flex gap-1">
                <Button
                  v-for="s in (selectedPipeline?.stages || []).filter(s => s.id !== col.id)"
                  :key="s.id"
                  variant="ghost"
                  size="sm"
                  class="h-6 text-xs"
                  @click.stop="moveStage(lead.id, s.id)"
                >
                  &rarr; {{ s.name }}
                </Button>
              </div>
            </div>
            <div
              v-if="col.leads.length === 0"
              class="text-xs text-muted-foreground text-center py-4"
            >
              No leads
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  </LayoutShell>
</template>
