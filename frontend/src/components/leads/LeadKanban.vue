<script setup lang="ts">
import { type Lead } from '@/stores/leads'
import { type Stage } from '@/stores/pipeline'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Plus, MoreHorizontal, ChevronRight, Link } from '@lucide/vue'

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

const stageColors: Record<number, string> = {
  0: 'border-l-blue-400',
  1: 'border-l-amber-400',
  2: 'border-l-emerald-400',
  3: 'border-l-violet-400',
  4: 'border-l-rose-400',
  5: 'border-l-cyan-400',
}

function getStageColor(index: number): string {
  return stageColors[index % Object.keys(stageColors).length]
}

function formatCurrency(value?: number) {
  if (!value) return ''
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value)
}
</script>

<template>
  <div class="flex gap-4 overflow-x-auto pb-4">
    <div
      v-for="(col, colIdx) in columns"
      :key="col.id"
      class="min-w-64 max-w-80 flex-1"
    >
      <div class="mb-2 flex items-center justify-between px-1">
        <div class="flex items-center gap-2">
          <div class="size-2 rounded-full" :class="getStageColor(colIdx).replace('border-l-', 'bg-')" />
          <span class="text-sm font-medium">{{ col.name }}</span>
          <Badge variant="secondary" class="text-xs px-1.5">{{ col.leads.length }}</Badge>
        </div>
        <Button variant="ghost" size="icon-sm" @click="emit('create', col.id)">
          <Plus class="size-3.5" />
        </Button>
      </div>
      <div class="space-y-2">
        <div
          v-for="lead in col.leads"
          :key="lead.id"
          class="group rounded-lg border bg-card p-3 text-sm shadow-sm cursor-pointer hover:shadow-md hover:border-primary/20 transition-all border-l-2"
          :class="getStageColor(colIdx)"
          @click="emit('edit', lead)"
        >
          <div class="flex items-start justify-between gap-2">
            <div class="font-medium truncate">{{ lead.name }}</div>
            <DropdownMenu>
              <DropdownMenuTrigger as-child @click.stop>
                <Button variant="ghost" size="icon-sm" class="size-6 -mr-1 -mt-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                  <MoreHorizontal class="size-3" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-40">
                <DropdownMenuItem
                  v-for="s in stages.filter(s => s.id !== col.id)"
                  :key="s.id"
                  @click.stop="emit('moveStage', lead.id!, s.id)"
                >
                  <ChevronRight class="mr-2 size-3.5" />
                  Move to {{ s.name }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
          <div v-if="lead.email" class="mt-0.5 text-xs text-muted-foreground truncate">{{ lead.email }}</div>
          <div v-if="lead.contact_name" class="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
            <Link class="size-3" />
            <span class="truncate">{{ lead.contact_name }}</span>
          </div>
          <div v-if="lead.value" class="mt-1.5 font-semibold text-primary">
            {{ formatCurrency(lead.value) }}
          </div>
        </div>
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
  </div>
</template>
