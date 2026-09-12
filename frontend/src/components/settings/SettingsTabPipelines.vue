<script setup lang="ts">
import { onMounted, ref, shallowRef } from 'vue'
import { storeToRefs } from 'pinia'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ArrowDown, ArrowUp, Check, Layers, Plus, Trash2, Pencil, X } from '@lucide/vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { errorMessage } from '@/utils/errors'
import { usePipelineStore } from '@/stores/pipeline'
import { createPipeline as apiCreatePipeline, updatePipeline as apiUpdatePipeline, deletePipeline as apiDeletePipeline, addStage, updateStage, deleteStage as apiDeleteStage, reorderStages, type Stage, type Pipeline } from '@/api/pipelines'

// readonly renders pipelines and stages without any mutation control: the
// read-only domain-tab view for users without settings:manage.
defineProps<{ readonly?: boolean }>()

// The shared store is the single frontend pipeline catalog: board and lead
// consumers read it, so settings mutations refresh it instead of a private
// copy that would leave other views stale.
const pipelineStore = usePipelineStore()
const { pipelines } = storeToRefs(pipelineStore)
const newPipelineName = shallowRef('')
const newPipelineDesc = shallowRef('')
const newPipelineError = shallowRef('')
const creatingPipeline = shallowRef(false)
const newStageNames = ref<Record<string, string>>({})
const editingStageId = shallowRef('')
const editingStageName = shallowRef('')

// Pipeline edit + delete-confirm state.
const editingPipelineId = shallowRef('')
const editingPipelineName = shallowRef('')
const editingPipelineDesc = shallowRef('')
const savingPipelineEdit = shallowRef(false)
const deletingPipeline = shallowRef<Pipeline | null>(null)
const deletingStage = shallowRef<{ id: string; name: string } | null>(null)

onMounted(() => loadPipelines())

async function loadPipelines() {
  try {
    await pipelineStore.fetchPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to load pipelines'))
  }
}

async function createPipeline() {
  newPipelineError.value = ''
  if (!newPipelineName.value) {
    newPipelineError.value = 'Name is required'
    return
  }
  creatingPipeline.value = true
  try {
    await apiCreatePipeline({ name: newPipelineName.value, description: newPipelineDesc.value })
    toast.success('Pipeline created')
    newPipelineName.value = ''
    newPipelineDesc.value = ''
    loadPipelines()
  } catch (e) {
    newPipelineError.value = errorMessage(e, 'Failed to create pipeline')
  } finally {
    creatingPipeline.value = false
  }
}

async function deletePipeline(pipelineId: string) {
  try {
    await apiDeletePipeline(pipelineId)
    toast.success('Pipeline deleted')
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to delete pipeline'))
  } finally {
    deletingPipeline.value = null
  }
}

function startEditPipeline(p: Pipeline) {
  editingPipelineId.value = p.id
  editingPipelineName.value = p.name
  editingPipelineDesc.value = p.description ?? ''
}

function cancelEditPipeline() {
  editingPipelineId.value = ''
}

async function savePipelineEdit(pipelineId: string) {
  const name = editingPipelineName.value.trim()
  if (!name) {
    toast.error('Pipeline name is required')
    return
  }
  savingPipelineEdit.value = true
  try {
    await apiUpdatePipeline(pipelineId, {
      name,
      description: editingPipelineDesc.value.trim(),
    })
    toast.success('Pipeline updated')
    cancelEditPipeline()
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to update pipeline'))
  } finally {
    savingPipelineEdit.value = false
  }
}

function requestDeletePipeline(p: Pipeline) {
  deletingPipeline.value = p
}

function requestDeleteStage(s: Stage) {
  deletingStage.value = { id: s.id, name: s.name }
}

async function confirmDeleteStage() {
  const stage = deletingStage.value
  if (!stage) return
  try {
    await apiDeleteStage(stage.id)
    toast.success('Stage deleted')
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to delete stage'))
  } finally {
    deletingStage.value = null
  }
}

async function createStage(pipelineId: string) {
  const name = newStageNames.value[pipelineId]?.trim()
  if (!name) return
  try {
    await addStage(pipelineId, { name })
    toast.success('Stage added')
    newStageNames.value[pipelineId] = ''
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to add stage'))
  }
}

function startEditStage(stageId: string, name: string) {
  editingStageId.value = stageId
  editingStageName.value = name
}

function cancelEditStage() {
  editingStageId.value = ''
  editingStageName.value = ''
}

async function renameStage(stageId: string) {
  const name = editingStageName.value.trim()
  if (!name) {
    toast.error('Stage name is required')
    return
  }
  try {
    await updateStage(stageId, { name })
    toast.success('Stage renamed')
    cancelEditStage()
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to rename stage'))
  }
}

// moveStage swaps a stage with its neighbor and submits the complete order in
// one call, so the server never sees a tie or a partial reorder.
async function moveStage(pipeline: Pipeline, stageId: string, direction: -1 | 1) {
  const stages = [...(pipeline.stages ?? [])].sort((a, b) => a.order - b.order)
  const idx = stages.findIndex((s) => s.id === stageId)
  const target = idx + direction
  if (idx < 0 || target < 0 || target >= stages.length) return
  const ordered = stages.map((s) => s.id)
  ;[ordered[idx], ordered[target]] = [ordered[target], ordered[idx]]
  try {
    await reorderStages(pipeline.id, ordered)
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to reorder stage'))
  }
}

// A stage's outcome is the single source of truth: 'open' keeps the lead in
// play, 'won'/'lost' make the stage closing (which cancels open tasks). The
// outcome is chosen explicitly so close-lost never guesses by stage name.
async function setStageOutcome(stage: Stage, outcome: string) {
  try {
    await updateStage(stage.id, { outcome })
    toast.success(outcome === 'open' ? 'Stage is now open' : `Stage marked as ${outcome}`)
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to update stage outcome'))
  }
}
</script>

<template>
  <div class="space-y-4">
    <Card v-if="!readonly">
      <CardHeader>
        <CardTitle class="text-base">Create Pipeline</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex flex-wrap gap-2">
          <Input v-model="newPipelineName" placeholder="Pipeline name" class="min-w-40 flex-1" />
          <Input v-model="newPipelineDesc" placeholder="Description" class="min-w-40 flex-1" />
          <Button @click="createPipeline" :disabled="creatingPipeline">
            <Plus class="mr-2 size-4" /> Add Pipeline
          </Button>
        </div>
        <div v-if="newPipelineError" class="mt-2 text-sm text-destructive">{{ newPipelineError }}</div>
      </CardContent>
    </Card>
    <div v-if="pipelines.length === 0" class="flex flex-col items-center justify-center py-12 text-center">
      <Layers class="size-10 text-muted-foreground/40 mb-3" />
      <p class="text-sm text-muted-foreground">No pipelines configured</p>
    </div>
    <Card v-for="p in pipelines" :key="p.id">
      <CardHeader class="flex flex-row items-center justify-between pb-2">
        <div>
          <CardTitle class="text-base">{{ p.name }}</CardTitle>
          <p v-if="p.description" class="text-sm text-muted-foreground mt-0.5">{{ p.description }}</p>
        </div>
        <div v-if="!readonly" class="flex items-center gap-1">
          <Button
            v-if="editingPipelineId !== p.id"
            variant="ghost"
            size="sm"
            title="Edit pipeline"
            @click="startEditPipeline(p)"
          >
            <Pencil class="mr-1 size-3.5" /> Edit
          </Button>
          <Button
            v-if="editingPipelineId !== p.id"
            variant="ghost"
            size="sm"
            title="Delete pipeline"
            @click="requestDeletePipeline(p)"
          >
            <Trash2 class="mr-1 size-3.5" /> Delete
          </Button>
        </div>
      </CardHeader>
      <CardContent v-if="!readonly && editingPipelineId === p.id" class="flex flex-wrap items-end gap-2 border-t pt-3">
        <div class="space-y-1">
          <Label class="text-xs">Name</Label>
          <Input v-model="editingPipelineName" class="min-w-40" @keyup.enter="savePipelineEdit(p.id)" />
        </div>
        <div class="space-y-1">
          <Label class="text-xs">Description</Label>
          <Input v-model="editingPipelineDesc" class="min-w-52" @keyup.enter="savePipelineEdit(p.id)" />
        </div>
        <Button :disabled="savingPipelineEdit" @click="savePipelineEdit(p.id)">
          {{ savingPipelineEdit ? 'Saving…' : 'Save' }}
        </Button>
        <Button variant="ghost" @click="cancelEditPipeline">Cancel</Button>
      </CardContent>
      <CardContent class="space-y-3">
        <div v-if="!readonly" class="flex flex-wrap gap-2">
          <Input
            v-model="newStageNames[p.id]"
            placeholder="Stage name"
            class="min-w-40 flex-1"
            @keyup.enter="createStage(p.id)"
          />
          <Button variant="outline" size="sm" @click="createStage(p.id)">
            <Plus class="mr-1 size-3.5" /> Add Stage
          </Button>
        </div>
        <div class="flex flex-col gap-1.5">
          <div
            v-for="(s, idx) in p.stages ?? []"
            :key="s.id"
            class="flex flex-wrap items-center gap-1.5 rounded-md border bg-muted/30 px-2 py-1.5"
          >
            <Badge variant="secondary" class="text-xs">
              {{ s.name }}
            </Badge>
            <template v-if="!readonly">
              <span class="text-xs text-muted-foreground">Outcome</span>
              <Select
                :model-value="s.outcome ?? 'open'"
                @update:model-value="(v) => setStageOutcome(s, String(v ?? 'open'))"
              >
                <SelectTrigger class="h-7 w-28 text-xs" :aria-label="`Outcome for ${s.name}`">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="open">Open</SelectItem>
                  <SelectItem value="won">Won</SelectItem>
                  <SelectItem value="lost">Lost</SelectItem>
                </SelectContent>
              </Select>
            </template>
            <div v-if="!readonly" class="ml-auto flex items-center gap-1">
              <template v-if="editingStageId === s.id">
                <Input
                  v-model="editingStageName"
                  class="h-8 w-40"
                  autofocus
                  @keyup.enter="renameStage(s.id)"
                  @keyup.esc="cancelEditStage"
                />
                <Button variant="outline" size="icon-sm" :title="`Save ${s.name}`" :aria-label="`Save ${s.name}`" @click="renameStage(s.id)">
                  <Check class="size-3.5" />
                </Button>
                <Button variant="ghost" size="icon-sm" title="Cancel" aria-label="Cancel" @click="cancelEditStage">
                  <X class="size-3.5" />
                </Button>
              </template>
              <template v-else>
                <Button variant="ghost" size="icon-sm" :title="`Rename ${s.name}`" :aria-label="`Rename ${s.name}`" @click="startEditStage(s.id, s.name)">
                  <Pencil class="size-3.5" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :disabled="idx === 0"
                  :title="`Move ${s.name} up`"
                  :aria-label="`Move ${s.name} up`"
                  @click="moveStage(p, s.id, -1)"
                >
                  <ArrowUp class="size-3.5" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :disabled="idx === (p.stages?.length ?? 0) - 1"
                  :title="`Move ${s.name} down`"
                  :aria-label="`Move ${s.name} down`"
                  @click="moveStage(p, s.id, 1)"
                >
                  <ArrowDown class="size-3.5" />
                </Button>
                <Button variant="ghost" size="icon-sm" :title="`Delete ${s.name}`" :aria-label="`Delete ${s.name}`" @click="requestDeleteStage(s)">
                  <Trash2 class="size-3.5" />
                </Button>
              </template>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <ConfirmDialog
      :open="!!deletingPipeline"
      title="Delete pipeline"
      :description="`Delete the pipeline “${deletingPipeline?.name ?? ''}” and its stages? This cannot be undone.`"
      confirm-text="Delete"
      destructive
      @update:open="(v) => { if (!v) deletingPipeline = null }"
      @confirm="deletePipeline(deletingPipeline?.id ?? '')"
    />
    <ConfirmDialog
      :open="!!deletingStage"
      title="Delete stage"
      :description="`Delete the stage “${deletingStage?.name ?? ''}”? Leads on it must be moved first.`"
      confirm-text="Delete"
      destructive
      @update:open="(v) => { if (!v) deletingStage = null }"
      @confirm="confirmDeleteStage"
    />
  </div>
</template>
