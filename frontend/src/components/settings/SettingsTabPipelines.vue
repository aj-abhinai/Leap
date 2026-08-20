<script setup lang="ts">
import { onMounted, ref, shallowRef } from 'vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
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
import { errorMessage } from '@/utils/errors'

interface Stage {
  id: string
  name: string
  order: number
  is_closing?: boolean
  outcome?: string
}

interface Pipeline {
  id: string
  name: string
  description?: string
  stages?: Stage[]
}

const pipelines = shallowRef<Pipeline[]>([])
const newPipelineName = shallowRef('')
const newPipelineDesc = shallowRef('')
const newPipelineError = shallowRef('')
const creatingPipeline = shallowRef(false)
const newStageNames = ref<Record<string, string>>({})
const editingStageId = shallowRef('')
const editingStageName = shallowRef('')

// Remember the last won/lost choice per stage: unchecking "Closing" forces the
// stage to 'open' server-side, so without this the win/loss would be lost and
// re-checking would silently default to 'lost'.
const rememberedOutcome = shallowRef<Record<string, string>>({})

onMounted(() => loadPipelines())

async function loadPipelines() {
  try {
    const res = await apiClient.get('/api/pipelines')
    pipelines.value = res.data
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
    await apiClient.post('/api/pipelines', { name: newPipelineName.value, description: newPipelineDesc.value })
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
    await apiClient.delete(`/api/pipelines/${pipelineId}`)
    toast.success('Pipeline deleted')
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to delete pipeline'))
  }
}

async function createStage(pipelineId: string) {
  const name = newStageNames.value[pipelineId]?.trim()
  if (!name) return
  try {
    await apiClient.post(`/api/pipelines/${pipelineId}/stages`, { name })
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
    await apiClient.patch(`/api/stages/${stageId}`, { name })
    toast.success('Stage renamed')
    cancelEditStage()
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to rename stage'))
  }
}

async function reorderStage(stageId: string, order: number) {
  try {
    await apiClient.patch(`/api/stages/${stageId}`, { order })
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to reorder stage'))
  }
}

async function deleteStage(stageId: string) {
  try {
    await apiClient.delete(`/api/stages/${stageId}`)
    toast.success('Stage deleted')
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to delete stage'))
  }
}

// Closing stages resolve the deal (won/lost) and cancel open tasks; non-closing
// stages stay 'open'. Outcome is chosen explicitly so close-lost never has to
// guess by stage name.
async function setClosing(stage: Stage, isClosing: boolean) {
  const current = stage.outcome === 'won' || stage.outcome === 'lost' ? stage.outcome : ''
  try {
    if (isClosing) {
      const outcome = rememberedOutcome.value[stage.id] || current || 'lost'
      delete rememberedOutcome.value[stage.id]
      await apiClient.patch(`/api/stages/${stage.id}`, { is_closing: true, outcome })
      toast.success('Stage marked as closing')
    } else {
      if (current) rememberedOutcome.value[stage.id] = current
      await apiClient.patch(`/api/stages/${stage.id}`, { is_closing: false, outcome: 'open' })
      toast.success('Stage is now open')
    }
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to update stage'))
  }
}

async function setStageOutcome(stage: Stage, outcome: string) {
  delete rememberedOutcome.value[stage.id]
  try {
    await apiClient.patch(`/api/stages/${stage.id}`, { outcome })
    toast.success('Stage outcome updated')
    loadPipelines()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to update outcome'))
  }
}
</script>

<template>
  <div class="space-y-4">
    <Card>
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
        <Button variant="ghost" size="sm" @click="deletePipeline(p.id)">
          <Trash2 class="mr-1 size-3.5" /> Delete
        </Button>
      </CardHeader>
      <CardContent class="space-y-3">
        <div class="flex flex-wrap gap-2">
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
            <label class="flex cursor-pointer items-center gap-1 text-xs text-muted-foreground" :title="`Mark ${s.name} as a closing stage`">
              <Checkbox
                :model-value="!!s.is_closing"
                class="size-3.5"
                :aria-label="`Mark ${s.name} as closing`"
                @update:model-value="(v) => setClosing(s, v === true)"
              />
              Closing
            </label>
            <Select
              v-if="s.is_closing"
              :model-value="s.outcome === 'won' ? 'won' : 'lost'"
              @update:model-value="(v) => setStageOutcome(s, String(v ?? 'lost'))"
            >
              <SelectTrigger class="h-7 w-24 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="lost">Lost</SelectItem>
                <SelectItem value="won">Won</SelectItem>
              </SelectContent>
            </Select>
            <div class="ml-auto flex items-center gap-1">
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
                  @click="reorderStage(s.id, s.order - 1)"
                >
                  <ArrowUp class="size-3.5" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :disabled="idx === (p.stages?.length ?? 0) - 1"
                  :title="`Move ${s.name} down`"
                  :aria-label="`Move ${s.name} down`"
                  @click="reorderStage(s.id, s.order + 1)"
                >
                  <ArrowDown class="size-3.5" />
                </Button>
                <Button variant="ghost" size="icon-sm" :title="`Delete ${s.name}`" :aria-label="`Delete ${s.name}`" @click="deleteStage(s.id)">
                  <Trash2 class="size-3.5" />
                </Button>
              </template>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
