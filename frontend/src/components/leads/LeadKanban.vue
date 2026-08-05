<script setup lang="ts">
import { computed } from 'vue'
import { type Lead } from '@/stores/leads'
import { type Stage } from '@/stores/pipeline'
import draggable from 'vuedraggable'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Plus, MoreHorizontal, ChevronRight, ListChecks, BookOpen } from '@lucide/vue'
import { formatCurrency, formatContactDetail } from '@/utils/format'

const props = defineProps<{
  columns: (Stage & { leads: Lead[] })[]
  stages: Stage[]
  pipelineId: string
}>()

const emit = defineEmits<{
  create: [stageId: string]
  edit: [lead: Lead]
  moveStage: [leadId: string, newStageId: string, previousStageId?: string]
  viewActivities: [lead: Lead]
}>()

const moveTargets = computed<Record<string, Stage[]>>(() => {
  const map: Record<string, Stage[]> = {}
  for (const col of props.columns) {
    map[col.id] = props.stages.filter(s => s.id !== col.id)
  }
  return map
})

function handleDragChange(evt: { added?: { element: Lead } }, newStageId: string) {
  if (evt.added) {
    const previousStageId = evt.added.element.stage_id
    if (previousStageId !== newStageId) {
      emit('moveStage', evt.added.element.id!, newStageId, previousStageId)
    }
  }
}
</script>

<template>
  <div class="flex w-full min-w-0 gap-4 overflow-x-auto pb-4">
    <div
      v-for="col in columns"
      :key="col.id"
      class="w-64 shrink-0 sm:w-72"
    >
      <div class="mb-2 flex items-center justify-between px-1">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium">{{ col.name }}</span>
          <Badge variant="secondary" class="text-xs px-1.5">{{ col.leads.length }}</Badge>
        </div>
        <Button variant="ghost" size="icon-sm" @click="emit('create', col.id)">
          <Plus class="size-3.5" />
        </Button>
      </div>
      <draggable
        :list="col.leads"
        :group="{ name: 'leads', pull: true, put: true }"
        item-key="id"
        class="space-y-2 min-h-12"
        ghost-class="opacity-40"
        drag-class="shadow-xl rotate-1 z-50"
        :animation="200"
        :sort="false"
        @change="(evt: any) => handleDragChange(evt, col.id)"
      >
        <template #item="{ element: lead }">
          <div
            :key="lead.id"
            class="group rounded-lg border bg-card p-3 text-sm shadow-sm cursor-grab active:cursor-grabbing hover:shadow-md hover:border-primary/20 transition-all"
            @click="emit('edit', lead)"
          >
            <div class="flex items-start justify-between gap-2">
              <div class="font-medium truncate">{{ lead.display_name }}</div>
              <div class="flex items-center gap-0.5 text-muted-foreground">
                <Button variant="ghost" size="icon-sm" class="size-6" @click.stop="emit('viewActivities', lead)" title="Activities" aria-label="View activities">
                  <ListChecks class="size-3" />
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child @click.stop>
                    <Button variant="ghost" size="icon-sm" class="size-6 -mr-1 -mt-0.5" aria-label="Lead actions">
                      <MoreHorizontal class="size-3" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="w-40">
                    <DropdownMenuItem
                      v-for="s in moveTargets[col.id]"
                      :key="s.id"
                      @click.stop="emit('moveStage', lead.id!, s.id)"
                    >
                      <ChevronRight class="mr-2 size-3.5" />
                      Move to {{ s.name }}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
            <div v-if="lead.contact_phone || lead.contact_email" class="mt-0.5 text-xs text-muted-foreground truncate">
              {{ formatContactDetail(lead.contact_phone, lead.contact_email) }}
            </div>
            <div v-if="lead.program_name" class="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
              <BookOpen class="size-3" />
              <span class="truncate">{{ lead.program_name }}</span>
            </div>
            <div v-if="lead.value" class="mt-1.5 font-semibold text-primary">
              {{ formatCurrency(lead.value) }}
            </div>
          </div>
        </template>
      </draggable>
        <div
          v-if="col.leads.length === 0"
          class="rounded-lg border border-dashed p-6 text-center"
        >
          <p class="text-xs text-muted-foreground">No leads</p>
          <Button
            variant="ghost"
            size="sm"
            class="mt-1 text-xs"
            @click="emit('create', col.id)"
          >
            <Plus class="mr-1 size-3" /> Add one
          </Button>
        </div>
    </div>
  </div>
</template>
