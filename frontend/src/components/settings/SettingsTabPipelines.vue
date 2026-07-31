<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Plus, Trash2, Layers } from '@lucide/vue'

interface Pipeline {
  id: string
  name: string
  description?: string
  stages?: { id: string; name: string; order: number }[]
}

const pipelines = ref<Pipeline[]>([])
const newPipelineName = ref('')
const newPipelineDesc = ref('')
const newPipelineError = ref('')
const creatingPipeline = ref(false)

onMounted(() => loadPipelines())

async function loadPipelines() {
  try {
    const res = await apiClient.get('/api/pipelines')
    pipelines.value = res.data
  } catch {}
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
  } catch (e: any) {
    newPipelineError.value = e.message || 'Failed to create pipeline'
  } finally {
    creatingPipeline.value = false
  }
}

async function deletePipeline(pipelineId: string) {
  try {
    await apiClient.delete(`/api/pipelines/${pipelineId}`)
    toast.success('Pipeline deleted')
    loadPipelines()
  } catch (e: any) {
    toast.error(e.message || 'Failed to delete pipeline')
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
      <CardContent>
        <div class="flex flex-wrap gap-1.5">
          <Badge v-for="s in p.stages" :key="s.id" variant="secondary">
            {{ s.name }}
          </Badge>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
