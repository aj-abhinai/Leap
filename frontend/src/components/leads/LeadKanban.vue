<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, shallowRef, watch } from 'vue'
import { type Lead } from '@/stores/leads'
import { type Stage } from '@/stores/pipeline'
import { useRBACStore } from '@/stores/rbac'
import { useUsersStore } from '@/stores/users'
import { addStage as apiAddStage } from '@/api/pipelines'
import { toast } from 'vue-sonner'
import draggable from 'vuedraggable'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Plus,
  MoreHorizontal,
  ChevronRight,
  ChevronDown,
  ChevronUp,
  ListChecks,
  BookOpen,
  Pencil,
  CheckCircle2,
  GripVertical,
  SlidersHorizontal,
  Check,
  User,
} from '@lucide/vue'
import { formatCurrency, formatContactDetail } from '@/utils/format'
import { timeAgo } from '@/utils/time'
import { errorMessage } from '@/utils/errors'

const rbac = useRBACStore()
const users = useUsersStore()

const props = defineProps<{
  columns: (Stage & { leads: Lead[]; count: number })[]
  stages: Stage[]
  pipelineId: string
}>()

const emit = defineEmits<{
  create: [stageId: string]
  edit: [lead: Lead]
  moveStage: [leadId: string, newStageId: string, previousStageId?: string]
  viewActivities: [lead: Lead]
  stageAdded: []
  bulkMove: [leadIds: string[], stageId: string]
}>()

// ---- per-pipeline kanban prefs (localStorage) ----
const collapsedKey = (pipelineId: string) => `crm:kanban:collapsed:${pipelineId}`
const widthsKey = (pipelineId: string) => `crm:kanban:widths:${pipelineId}`
const fieldsKey = (pipelineId: string) => `crm:kanban:fields:${pipelineId}`

const DEFAULT_WIDTH = 288
const MIN_WIDTH = 200
const MAX_WIDTH = 480

const collapsed = shallowRef<Record<string, boolean>>(loadJson(collapsedKey(props.pipelineId), {}))
const columnWidths = shallowRef<Record<string, number>>(loadJson(widthsKey(props.pipelineId), {}))
const cardFields = shallowRef<string[]>(loadJson(fieldsKey(props.pipelineId), ['contact', 'program', 'assignee', 'outcome', 'value', 'next_task', 'last_touch']))

function loadJson<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T) : fallback
  } catch {
    return fallback
  }
}

function saveJson(key: string, value: unknown) {
  localStorage.setItem(key, JSON.stringify(value))
}

watch(collapsed, (v) => saveJson(collapsedKey(props.pipelineId), v), { deep: true })
watch(columnWidths, (v) => saveJson(widthsKey(props.pipelineId), v), { deep: true })
watch(cardFields, (v) => saveJson(fieldsKey(props.pipelineId), v), { deep: true })

onMounted(() => users.fetchOptions())

// Resolve the assignee's name from the users store; falls back to a neutral
// label when the id is unknown (e.g. a deleted user).
function assigneeName(assignedTo: string): string {
  return users.options.find((u) => u.id === assignedTo)?.name || 'Assigned'
}

// Reload per-pipeline prefs when the pipeline selector changes; without this,
// switching pipelines would render with the previous pipeline's saved prefs
// and then overwrite the new pipeline's stored values with stale ones.
watch(
  () => props.pipelineId,
  (id) => {
    collapsed.value = loadJson(collapsedKey(id), {})
    columnWidths.value = loadJson(widthsKey(id), {})
    cardFields.value = loadJson(fieldsKey(id), ['contact', 'program', 'assignee', 'outcome', 'value', 'next_task', 'last_touch'])
    selectedCardFields.value = new Set(cardFields.value)
    clearSelection()
    bulkTargetStageId.value = ''
  },
)

function isCollapsed(stageId: string): boolean {
  return !!collapsed.value[stageId]
}

function toggleCollapsed(stageId: string) {
  collapsed.value = { ...collapsed.value, [stageId]: !collapsed.value[stageId] }
}

function columnWidth(stageId: string): number {
  return columnWidths.value[stageId] ?? DEFAULT_WIDTH
}

// ---- column resize ----
const resizingStage = shallowRef('')
const resizeStartX = shallowRef(0)
const resizeStartWidth = shallowRef(0)

function startResize(stageId: string, event: MouseEvent) {
  resizingStage.value = stageId
  resizeStartX.value = event.clientX
  resizeStartWidth.value = columnWidth(stageId)
  window.addEventListener('mousemove', onResizeMove)
  window.addEventListener('mouseup', stopResize)
}

function onResizeMove(event: MouseEvent) {
  if (!resizingStage.value) return
  const delta = event.clientX - resizeStartX.value
  const width = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, resizeStartWidth.value + delta))
  columnWidths.value = { ...columnWidths.value, [resizingStage.value]: width }
}

function stopResize() {
  resizingStage.value = ''
  window.removeEventListener('mousemove', onResizeMove)
  window.removeEventListener('mouseup', stopResize)
}

onBeforeUnmount(stopResize)

// ---- inline add stage ----
const addingStage = shallowRef(false)
const newStageName = shallowRef('')

async function addStage() {
  const name = newStageName.value.trim()
  if (!name) {
    toast.error('Stage name is required')
    return
  }
  try {
    const maxOrder = props.stages.reduce((m, s) => Math.max(m, s.order), -1)
    await apiAddStage(props.pipelineId, { name, order: maxOrder + 1 })
    toast.success('Stage added')
    newStageName.value = ''
    addingStage.value = false
    emit('stageAdded')
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to add stage'))
  }
}

// ---- customize cards (per-pipeline field picker) ----
const CARD_FIELD_OPTIONS = [
  { key: 'contact', label: 'Contact detail' },
  { key: 'program', label: 'Program' },
  { key: 'assignee', label: 'Assignee' },
  { key: 'outcome', label: 'Outcome / Lost reason' },
  { key: 'value', label: 'Value' },
  { key: 'next_task', label: 'Next task' },
  { key: 'last_touch', label: 'Last touch' },
] as const

const showFieldPicker = shallowRef(false)
const selectedCardFields = shallowRef(new Set<string>(cardFields.value))

function toggleCardField(key: string) {
  const next = new Set(selectedCardFields.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  selectedCardFields.value = next
}

function applyCardFields() {
  cardFields.value = [...selectedCardFields.value]
  showFieldPicker.value = false
}

// ---- bulk move ----
const selectedIds = shallowRef<Set<string>>(new Set())

// A closed lead (one in a closing stage) is terminal and cannot be bulk-moved:
// bulk move is for open work; dragging a closed card spawns a new cycle instead.
function isClosedLead(lead: Lead): boolean {
  return lead.stage_outcome === 'won' || lead.stage_outcome === 'lost'
}

// selectableLeads is the subset of a column's cards that bulk selection
// applies to — open leads only. All selection logic derives from this one
// collection so the closed-lead rule is defined once.
function selectableLeads(col: { leads: Lead[] }): Lead[] {
  return col.leads.filter((l) => !isClosedLead(l))
}

function isSelected(id: string): boolean {
  return selectedIds.value.has(id)
}

function toggleSelect(id: string) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedIds.value = next
}

function columnAllSelected(col: Stage & { leads: Lead[]; count: number }): boolean {
  const selectable = selectableLeads(col)
  return selectable.length > 0 && selectable.every((l) => selectedIds.value.has(l.id!))
}

function columnSomeSelected(col: Stage & { leads: Lead[]; count: number }): boolean {
  return selectableLeads(col).some((l) => selectedIds.value.has(l.id!))
}

function columnCheckState(col: Stage & { leads: Lead[]; count: number }): boolean | 'indeterminate' {
  if (columnAllSelected(col)) return true
  if (columnSomeSelected(col)) return 'indeterminate'
  return false
}

function toggleColumnSelect(col: Stage & { leads: Lead[]; count: number }) {
  const next = new Set(selectedIds.value)
  const selectable = selectableLeads(col)
  if (columnAllSelected(col)) {
    for (const l of selectable) next.delete(l.id!)
  } else {
    for (const l of selectable) next.add(l.id!)
  }
  selectedIds.value = next
}

function clearSelection() {
  selectedIds.value = new Set()
}

const bulkTargetStageId = shallowRef('')

async function doBulkMove() {
  const target = bulkTargetStageId.value
  if (!target || selectedIds.value.size === 0) return
  const ids = [...selectedIds.value]
  emit('bulkMove', ids, target)
  clearSelection()
  bulkTargetStageId.value = ''
}

const moveTargets = computed<Record<string, Stage[]>>(() => {
  const map: Record<string, Stage[]> = {}
  for (const col of props.columns) {
    map[col.id] = props.stages.filter((s) => s.id !== col.id)
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
function isNextTaskOverdue(lead: Lead): boolean {
  return !!lead.next_task_at && new Date(lead.next_task_at).getTime() < Date.now()
}

// stage_outcome is the authoritative won/lost signal for a lead; 'open'
// means the lead is in play, so no outcome badge is shown.
function cardOutcome(lead: Lead): string {
  return lead.stage_outcome === 'won' || lead.stage_outcome === 'lost'
    ? lead.stage_outcome
    : ''
}

function formatNextTaskAt(lead: Lead): string {
  return new Date(lead.next_task_at!).toLocaleString([], {
    weekday: 'short',
    hour: 'numeric',
    minute: '2-digit',
  })
}

function showField(key: string): boolean {
  return cardFields.value.includes(key)
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <!-- Bulk move action bar -->
    <div
      v-if="selectedIds.size > 0"
      class="sticky top-0 z-20 flex flex-wrap items-center gap-2 rounded-lg border bg-card/95 px-3 py-2 shadow-sm backdrop-blur"
    >
      <span class="text-sm font-medium tabular-nums">{{ selectedIds.size }} selected</span>
      <Select v-model="bulkTargetStageId" class="w-48">
        <SelectTrigger class="h-8 w-48">
          <SelectValue placeholder="Move to stage…" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="s in stages" :key="s.id" :value="s.id">
            {{ s.name }}
          </SelectItem>
        </SelectContent>
      </Select>
      <Button size="sm" :disabled="!bulkTargetStageId" @click="doBulkMove">
        <CheckCircle2 class="mr-2 size-3.5" /> Move
      </Button>
      <Button size="sm" variant="ghost" @click="clearSelection">Clear</Button>
    </div>

    <!-- Kanban row -->
    <div class="flex w-full min-w-0 gap-2 overflow-x-auto pb-4">
      <div
        v-for="col in columns"
        :key="col.id"
        class="group relative shrink-0"
        :style="{ width: `${columnWidth(col.id)}px` }"
      >
        <div class="mb-2 flex items-center gap-1.5 px-1">
          <Checkbox
            v-if="rbac.can('lead:write') && selectableLeads(col).length > 0"
            :model-value="columnCheckState(col)"
            class="size-4"
            :aria-label="`Select all in ${col.name}`"
            @click.stop
            @update:model-value="toggleColumnSelect(col)"
          />
          <button
            type="button"
            class="flex min-w-0 flex-1 items-center gap-1.5 text-left"
            :aria-label="`${isCollapsed(col.id) ? 'Expand' : 'Collapse'} ${col.name}`"
            @click="toggleCollapsed(col.id)"
          >
            <ChevronDown v-if="isCollapsed(col.id)" class="size-3.5 shrink-0 text-muted-foreground" />
            <ChevronUp v-else class="size-3.5 shrink-0 text-muted-foreground" />
            <span class="truncate text-sm font-medium">{{ col.name }}</span>
            <Badge variant="secondary" class="text-xs px-1.5" :title="col.count > col.leads.length ? `${col.count} total in this stage` : undefined">
              {{ col.count }}
            </Badge>
          </button>
          <Button
            v-if="rbac.can('lead:write')"
            variant="ghost"
            size="icon-sm"
            :aria-label="`Add lead to ${col.name}`"
            @click="emit('create', col.id)"
          >
            <Plus class="size-3.5" />
          </Button>
        </div>

        <div v-if="!isCollapsed(col.id)">
          <draggable
            :list="col.leads"
            :group="{ name: 'leads', pull: true, put: true }"
            item-key="id"
            class="space-y-2 min-h-12"
            ghost-class="opacity-40"
            drag-class="lead-kanban-dragging"
            :animation="200"
            :sort="false"
            :disabled="!rbac.can('lead:write')"
            @change="(evt: { added?: { element: Lead } }) => handleDragChange(evt, col.id)"
          >
            <template #item="{ element: lead }">
              <div
                :key="lead.id"
                class="group relative rounded-lg border bg-card p-3 text-sm shadow-sm transition-all hover:border-primary/20 hover:shadow-md cursor-pointer"
                @click="emit('viewActivities', lead)"
              >
                <div class="absolute left-2 top-2 z-10">
                  <Checkbox
                    v-if="rbac.can('lead:write') && !isClosedLead(lead)"
                    :model-value="isSelected(lead.id!)"
                    class="size-4 opacity-0 transition-opacity group-hover:opacity-100"
                    :class="{ 'opacity-100': isSelected(lead.id!) }"
                    :aria-label="`Select ${lead.display_name}`"
                    @click.stop
                    @update:model-value="toggleSelect(lead.id!)"
                  />
                </div>
                <div class="flex items-start justify-between gap-2 pl-0" :class="{ 'pl-6': rbac.can('lead:write') }">
                  <div class="min-w-0 font-medium truncate">{{ lead.display_name }}</div>
                  <div class="flex shrink-0 items-center gap-0.5 text-muted-foreground">
                    <Button
                      v-if="rbac.can('lead:write')"
                      variant="ghost"
                      size="icon-sm"
                      class="size-8"
                      @click.stop="emit('edit', lead)"
                      title="Edit lead"
                      aria-label="Edit lead"
                    >
                      <Pencil class="size-3.5" />
                    </Button>
                    <DropdownMenu>
                      <DropdownMenuTrigger as-child @click.stop>
                        <Button variant="ghost" size="icon-sm" class="size-8" aria-label="Lead actions">
                          <MoreHorizontal class="size-3.5" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" class="w-40">
                        <template v-for="s in moveTargets[col.id]" :key="s.id">
                          <DropdownMenuItem
                            v-if="rbac.can('lead:write')"
                            @click.stop="emit('moveStage', lead.id!, s.id)"
                          >
                            <ChevronRight class="mr-2 size-3.5" />
                            Move to {{ s.name }}
                          </DropdownMenuItem>
                        </template>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </div>
                <div v-if="showField('contact') && (lead.contact_phone || lead.contact_email)" class="mt-0.5 text-xs text-muted-foreground truncate">
                  {{ formatContactDetail(lead.contact_phone, lead.contact_email) }}
                </div>
                <div v-if="showField('program') && lead.program_name" class="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                  <BookOpen class="size-3" />
                  <span class="truncate">{{ lead.program_name }}</span>
                </div>
                <div v-if="showField('assignee') && lead.assigned_to" class="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                  <User class="size-3" />
                  <span class="truncate">{{ assigneeName(lead.assigned_to) }}</span>
                </div>
                <div v-if="showField('outcome') && cardOutcome(lead)" class="mt-1.5 flex items-center gap-1.5">
                  <Badge
                    :variant="cardOutcome(lead) === 'won' ? 'default' : 'destructive'"
                    class="text-xs px-1.5"
                  >
                    {{ cardOutcome(lead) === 'won' ? 'Won' : 'Lost' }}
                  </Badge>
                  <span v-if="cardOutcome(lead) === 'lost' && lead.lost_reason" class="text-xs text-muted-foreground truncate">
                    {{ lead.lost_reason }}
                  </span>
                </div>
                <div v-if="showField('value') && lead.value" class="mt-1.5 text-sm font-semibold text-primary tabular-nums">
                  {{ formatCurrency(lead.value) }}
                </div>
                <div v-if="showField('next_task') || showField('last_touch')" class="mt-1.5 space-y-0.5">
                  <div
                    v-if="showField('next_task') && lead.next_task_type"
                    class="flex items-center gap-1 text-xs"
                    :class="isNextTaskOverdue(lead) ? 'text-destructive' : 'text-muted-foreground'"
                  >
                    <ListChecks class="size-3 shrink-0" />
                    <span class="truncate">
                      Next: {{ lead.next_task_type }}
                      <template v-if="lead.next_task_at">
                        · {{ formatNextTaskAt(lead) }}
                      </template>
                    </span>
                  </div>
                  <div v-if="showField('last_touch') && lead.last_touch_type && lead.last_touch_at" class="flex items-center gap-1 text-xs text-muted-foreground">
                    <CheckCircle2 class="size-3 shrink-0" />
                    <span class="truncate">
                      Last: {{ lead.last_touch_type }} · {{ timeAgo(lead.last_touch_at) }}
                    </span>
                  </div>
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
              v-if="rbac.can('lead:write')"
              variant="ghost"
              size="sm"
              class="mt-1 text-xs"
              @click="emit('create', col.id)"
            >
              <Plus class="mr-1 size-3" /> Add one
            </Button>
          </div>
        </div>

        <!-- resize handle -->
        <div
          class="absolute right-0 top-0 z-10 flex h-full w-1.5 cursor-col-resize items-center justify-center"
          :class="resizingStage === col.id ? 'bg-primary/30' : 'opacity-0 group-hover:opacity-100'"
          @mousedown.prevent.stop="startResize(col.id, $event)"
        >
          <GripVertical class="size-3 text-muted-foreground" />
        </div>
      </div>

      <!-- Inline add stage (settings:manage only) -->
      <div v-if="rbac.can('settings:manage')" class="shrink-0 w-64">
        <div v-if="addingStage" class="rounded-lg border border-dashed p-3 space-y-2">
          <Input
            v-model="newStageName"
            placeholder="Stage name"
            autofocus
            @keyup.enter="addStage"
            @keyup.esc="addingStage = false"
          />
          <div class="flex gap-1.5">
            <Button size="sm" @click="addStage"><Check class="mr-1 size-3.5" /> Add</Button>
            <Button size="sm" variant="ghost" @click="addingStage = false">Cancel</Button>
          </div>
        </div>
        <Button
          v-else
          variant="ghost"
          size="sm"
          class="w-full justify-start text-muted-foreground border border-dashed"
          @click="addingStage = true"
        >
          <Plus class="mr-1 size-3.5" /> Add stage
        </Button>
      </div>
    </div>

    <!-- Customize cards (per-pipeline) -->
    <DropdownMenu v-model:open="showFieldPicker">
      <DropdownMenuTrigger as-child>
        <Button variant="outline" size="sm" class="fixed bottom-6 right-6 z-30 shadow-lg" title="Customize cards">
          <SlidersHorizontal class="mr-2 size-3.5" /> Customize cards
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" class="w-56">
        <p class="px-2 py-1.5 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Card fields</p>
        <label
          v-for="opt in CARD_FIELD_OPTIONS"
          :key="opt.key"
          class="flex cursor-pointer items-center gap-2 px-2 py-1.5 text-sm hover:bg-accent"
        >
          <Checkbox
            :model-value="selectedCardFields.has(opt.key)"
            class="size-4"
            @update:model-value="toggleCardField(opt.key)"
          />
          {{ opt.label }}
        </label>
        <div class="flex justify-end gap-1.5 px-2 py-2">
          <Button size="sm" @click="applyCardFields">Apply</Button>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  </div>
</template>

<style scoped>
:global(.lead-kanban-dragging) {
  transform: rotate(1deg);
  z-index: 50;
  box-shadow: var(--shadow-xl);
}
</style>
