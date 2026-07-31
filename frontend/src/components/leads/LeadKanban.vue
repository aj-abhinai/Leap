<script setup lang="ts">
import { type Lead } from '@/stores/leads'
import { type Stage } from '@/stores/pipeline'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Plus } from '@lucide/vue'

const props = defineProps<{
  columns: (Stage & { leads: Lead[] })[]
  stages: Stage[]
  pipelineId: string
}>()

const emit = defineEmits<{
  create: [stageId: string]
  edit: [lead: Lead]
  moveStage: [leadId: string, newStageId: string]
}>()

function formatCurrency(value?: number) {
  if (!value) return ''
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value)
}
</script>

<template>
  <div class="flex gap-4 overflow-x-auto pb-4">
    <Card
      v-for="col in columns"
      :key="col.id"
      class="min-w-64 flex-1 bg-muted/30"
    >
      <CardHeader class="flex flex-row items-center justify-between pb-2">
        <CardTitle class="text-sm font-medium">
          {{ col.name }}
          <Badge variant="secondary" class="ml-2">{{ col.leads.length }}</Badge>
        </CardTitle>
        <Button variant="ghost" size="icon" @click="emit('create', col.id)">
          <Plus class="size-4" />
        </Button>
      </CardHeader>
      <CardContent class="space-y-2">
        <div
          v-for="lead in col.leads"
          :key="lead.id"
          class="rounded-lg border bg-card p-3 text-sm shadow-sm cursor-pointer hover:shadow-md transition-shadow"
          @click="emit('edit', lead)"
        >
          <div class="font-medium">{{ lead.name }}</div>
          <div v-if="lead.email" class="text-xs text-muted-foreground">{{ lead.email }}</div>
          <div v-if="lead.value" class="mt-1 font-medium text-primary">
            {{ formatCurrency(lead.value) }}
          </div>
          <div class="mt-2 flex gap-1">
            <Button
              v-for="s in stages.filter(s => s.id !== col.id)"
              :key="s.id"
              variant="ghost"
              size="sm"
              class="h-6 text-xs"
              @click.stop="emit('moveStage', lead.id!, s.id)"
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
</template>
