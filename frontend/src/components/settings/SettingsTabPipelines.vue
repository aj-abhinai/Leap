<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/composables/useApi'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Plus } from '@lucide/vue'

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
  try {
    await apiClient.post('/api/pipelines', { name: newPipelineName.value, description: newPipelineDesc.value })
    newPipelineName.value = ''
    newPipelineDesc.value = ''
    loadPipelines()
  } catch (e: any) {
    newPipelineError.value = e.message
  }
}

async function deletePipeline(pipelineId: string) {
  try {
    await apiClient.delete(`/api/pipelines/${pipelineId}`)
    loadPipelines()
  } catch {}
}
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardHeader>
        <CardTitle>Create Pipeline</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex gap-2">
          <Input v-model="newPipelineName" placeholder="Pipeline name" class="flex-1" />
          <Input v-model="newPipelineDesc" placeholder="Description" class="flex-1" />
          <Button @click="createPipeline">
            <Plus class="mr-2 size-4" /> Add
          </Button>
        </div>
        <div v-if="newPipelineError" class="mt-2 text-sm text-destructive">{{ newPipelineError }}</div>
      </CardContent>
    </Card>
    <Card v-for="p in pipelines" :key="p.id">
      <CardHeader class="flex flex-row items-center justify-between">
        <CardTitle class="text-base">{{ p.name }}</CardTitle>
        <Button variant="ghost" size="sm" @click="deletePipeline(p.id)">Delete</Button>
      </CardHeader>
      <CardContent>
        <div class="text-sm text-muted-foreground mb-2">{{ p.description }}</div>
        <div class="flex flex-wrap gap-1">
          <Badge v-for="s in p.stages" :key="s.id" variant="outline">
            {{ s.name }}
          </Badge>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
