<script setup lang="ts">
import { computed, onMounted, shallowRef, watch } from 'vue'
import { useActivitiesStore, type ActivityListFilters } from '@/stores/activities'
import { useSettingsStore } from '@/stores/settings'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from '@/components/ui/dropdown-menu'
import { reminderIcon, snoozePresets, snoozeRemindAt } from '@/utils/reminders'
import { toast } from 'vue-sonner'
import { CheckCircle2, Trash2, MoreHorizontal, AlarmClockPlus, ClipboardList } from '@lucide/vue'
import { errorMessage } from '@/utils/errors'
import { useLeadDrawerGlobal } from '@/composables/useLeadDrawerGlobal'

const store = useActivitiesStore()
const settings = useSettingsStore()
const { openLeadDrawer } = useLeadDrawerGlobal()

const search = shallowRef('')
const typeFilter = shallowRef('')
const sortBy = shallowRef('due_at')
const sortOrder = shallowRef('desc')
const selected = shallowRef<Set<string>>(new Set())
const deleting = shallowRef(false)
const deletingIds = shallowRef<string[]>([])

interface ViewDef {
  id: string
  label: string
  status: string
  overdue?: boolean
  mine?: boolean
  today?: boolean
}

const views: ViewDef[] = [
  { id: 'all', label: 'All Activities', status: 'all' },
  { id: 'open', label: 'Open', status: 'open' },
  { id: 'done', label: 'Completed', status: 'done' },
  { id: 'cancelled', label: 'Canceled', status: 'cancelled' },
  { id: 'overdue', label: 'Overdue', status: 'open', overdue: true },
  { id: 'today_overdue', label: 'Today + Overdue', status: 'open', overdue: true, today: true },
  { id: 'today', label: "Today's Activities", status: 'all', today: true },
  { id: 'my_open', label: 'My Open', status: 'open', mine: true },
  { id: 'my_overdue', label: 'My Overdue', status: 'open', overdue: true, mine: true },
  { id: 'my_done', label: 'My Completed', status: 'done', mine: true },
]

const activeView = shallowRef<ViewDef>(views[0])

// todayBounds returns the current day as a [start, end) window in UTC ISO,
// used by the Today views.
function todayBounds(): { from: string; to: string } {
  const start = new Date()
  start.setHours(0, 0, 0, 0)
  const end = new Date(start)
  end.setDate(end.getDate() + 1)
  return { from: start.toISOString(), to: end.toISOString() }
}

// viewFilter maps the active view and filter controls to the store's query
// filters; Today views add the day window.
const viewFilter = computed<ActivityListFilters>(() => {
  const v = activeView.value
  const f: ActivityListFilters = { status: v.status === 'all' ? '' : v.status }
  if (v.overdue) f.overdue = 'true'
  if (v.mine) f.mine = 'true'
  if (v.today) {
    const { from, to } = todayBounds()
    f.from = from
    f.to = to
  }
  if (typeFilter.value) f.type = typeFilter.value
  if (search.value.trim()) f.q = search.value.trim()
  if (sortBy.value) f.sort = sortBy.value
  if (sortOrder.value) f.order = sortOrder.value
  return f
})

const totalPages = computed(() => Math.max(1, Math.ceil(store.total / store.perPage)))

function load() {
  store.fetchItems(viewFilter.value)
}

onMounted(() => {
  if (settings.activityTypes.length === 0) settings.fetchTags()
  load()
})

watch([search, typeFilter, sortBy, sortOrder, activeView], () => {
  // Any filter or view change restarts from page one.
  store.page = 1
  load()
})

function selectView(v: ViewDef) {
  activeView.value = v
  store.page = 1
  selected.value = new Set()
}

function isSelected(id: string): boolean {
  return selected.value.has(id)
}

function toggle(id: string) {
  const next = new Set(selected.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selected.value = next
}

function allSelected(): boolean {
  return store.items.length > 0 && store.items.every((i) => selected.value.has(i.id))
}

function toggleAll() {
  const next = new Set(selected.value)
  if (allSelected()) {
    for (const i of store.items) next.delete(i.id)
  } else {
    for (const i of store.items) next.add(i.id)
  }
  selected.value = next
}

function confirmDeleteOne(item: { id: string }) {
  deletingIds.value = [item.id]
  deleting.value = true
}

function confirmDeleteSelected() {
  deletingIds.value = [...selected.value]
  deleting.value = true
}

// doDelete removes the selected activities via their lead-scoped endpoints,
// clears the selection, and reloads; failures surface as a toast.
async function doDelete() {
  const ids = deletingIds.value
  deleting.value = false
  deletingIds.value = []
  let deleted = 0
  try {
    for (const id of ids) {
      const item = store.items.find((i) => i.id === id)
      if (item) {
        await store.deleteItem(item.lead_id, id)
        deleted++
      }
    }
    selected.value = new Set()
    toast.success(deleted > 1 ? `Deleted ${deleted} activities` : 'Activity deleted')
    load()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to delete'))
  }
}

async function doMarkDone(item: { id: string; lead_id: string }) {
  try {
    await store.markDone(item.lead_id, item.id)
    toast.success('Activity completed')
    load()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to complete'))
  }
}

async function doSnooze(item: { id: string; lead_id: string }, minutes: number) {
  try {
    await store.snooze(item.lead_id, item.id, snoozeRemindAt(minutes))
    toast.success('Reminder snoozed')
    load()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to snooze'))
  }
}

function isOverdue(item: { is_done: boolean; is_cancelled: boolean; remind_at?: string }): boolean {
  return (
    !item.is_done && !item.is_cancelled &&
    !!item.remind_at && new Date(item.remind_at).getTime() < Date.now()
  )
}

// statusLabel returns the display status by precedence: cancelled, done,
// reminded, overdue, else open.
function statusLabel(item: {
  is_done: boolean
  is_cancelled: boolean
  is_reminded: boolean
  remind_at?: string
}): string {
  if (item.is_cancelled) return 'Canceled'
  if (item.is_done) return 'Done'
  if (item.is_reminded) return 'Reminded'
  if (isOverdue(item)) return 'Overdue'
  return 'Open'
}

function statusVariant(label: string): 'default' | 'destructive' | 'secondary' | 'success' {
  if (label === 'Overdue') return 'destructive'
  if (label === 'Canceled') return 'secondary'
  if (label === 'Done') return 'success'
  if (label === 'Reminded') return 'secondary'
  return 'default'
}

// due returns the effective due time: remind_at, else scheduled_at, else created_at.
function due(item: { scheduled_at?: string; remind_at?: string; created_at: string }): string {
  if (item.remind_at) return item.remind_at
  if (item.scheduled_at) return item.scheduled_at
  return item.created_at
}

function dueLabel(item: { scheduled_at?: string; remind_at?: string; created_at: string }): string {
  const t = due(item)
  return new Date(t).toLocaleString()
}

function nextPage() {
  if (store.page < totalPages.value) {
    store.page++
    // Selection is page-scoped; leaving the page drops it.
    selected.value = new Set()
    load()
  }
}

function prevPage() {
  if (store.page > 1) {
    store.page--
    selected.value = new Set()
    load()
  }
}

function typeClass(type: string): string {
  return type || 'Task'
}
</script>

<template>
  <div class="p-6">
    <div class="mb-4">
      <h1 class="text-2xl font-semibold tracking-tight">Activities</h1>
      <p v-if="!store.loading && store.total" class="mt-0.5 text-sm text-muted-foreground">
        {{ store.total }} total &middot; {{ selected.size }} selected
      </p>
    </div>

    <div class="flex flex-col gap-4 lg:flex-row">
      <!-- Views rail -->
      <aside class="lg:w-52 shrink-0">
        <nav class="flex flex-row flex-wrap gap-1 lg:flex-col">
          <button
            v-for="v in views"
            :key="v.id"
            type="button"
            class="rounded-md px-3 py-1.5 text-left text-sm transition-colors"
            :class="activeView.id === v.id ? 'bg-primary/10 font-medium text-primary' : 'text-muted-foreground hover:bg-accent'"
            @click="selectView(v)"
          >
            {{ v.label }}
          </button>
        </nav>
      </aside>

      <!-- Main column -->
      <div class="min-w-0 flex-1 space-y-4">
        <!-- Filter bar -->
        <div class="flex flex-wrap items-center gap-2">
          <select
            v-model="typeFilter"
            class="h-8 rounded-md border bg-background px-2 text-sm"
          >
            <option value="">All types</option>
            <option v-for="t in settings.activityTypes" :key="t.id" :value="t.name">
              {{ t.name }}
            </option>
          </select>
          <Input
            v-model="search"
            class="h-8 w-48"
            placeholder="Search lead, notes…"
          />
          <select
            v-model="sortBy"
            class="h-8 rounded-md border bg-background px-2 text-sm"
          >
            <option value="due_at">Sort: Due date</option>
            <option value="type">Sort: Type</option>
            <option value="created_at">Sort: Created</option>
          </select>
          <select
            v-model="sortOrder"
            class="h-8 rounded-md border bg-background px-2 text-sm"
          >
            <option value="desc">Newest first</option>
            <option value="asc">Oldest first</option>
          </select>
        </div>

        <!-- Mass action bar -->
        <div v-if="selected.size > 0" class="flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2">
          <span class="text-sm text-muted-foreground">{{ selected.size }} selected</span>
          <Button size="sm" variant="destructive" @click="confirmDeleteSelected">
            <Trash2 class="mr-2 size-3.5" /> Delete
          </Button>
          <Button size="sm" variant="ghost" @click="selected = new Set()">Clear</Button>
        </div>

        <!-- Table -->
        <div v-if="store.loading" class="space-y-3">
          <Skeleton v-for="i in 6" :key="i" class="h-12 w-full" />
        </div>

        <div
          v-else-if="store.items.length === 0"
          class="flex flex-col items-center justify-center py-16 text-center"
        >
          <ClipboardList class="size-10 text-muted-foreground/40 mb-3" />
          <p class="text-sm font-medium text-muted-foreground">No activities here</p>
          <p class="text-xs text-muted-foreground/60 mt-1">Try a different view or clear the filters</p>
        </div>

        <Card v-else>
          <CardContent class="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead class="w-10">
                    <input
                      type="checkbox"
                      class="size-4"
                      :checked="allSelected()"
                      @change="toggleAll()"
                    />
                  </TableHead>
                  <TableHead>Lead</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Due</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Quick reply</TableHead>
                  <TableHead>Owner</TableHead>
                  <TableHead class="w-10" />
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="item in store.items" :key="item.id" class="group">
                  <TableCell>
                    <input
                      type="checkbox"
                      class="size-4"
                      :checked="isSelected(item.id)"
                      @change="toggle(item.id)"
                    />
                  </TableCell>
                  <TableCell>
                    <button
                      type="button"
                      class="cursor-pointer font-medium hover:text-primary"
                      @click="openLeadDrawer(item.lead_id)"
                    >
                      {{ item.lead_display_name }}
                    </button>
                  </TableCell>
                  <TableCell>
                    <span class="inline-flex items-center gap-1.5 text-sm">
                      <component :is="reminderIcon(item.type)" class="size-4 text-muted-foreground" />
                      {{ typeClass(item.type) }}
                    </span>
                  </TableCell>
                  <TableCell class="text-xs" :class="isOverdue(item) ? 'text-destructive' : 'text-muted-foreground'">
                    {{ dueLabel(item) }}
                  </TableCell>
                  <TableCell>
                    <Badge :variant="statusVariant(statusLabel(item))" class="text-xs">
                      {{ statusLabel(item) }}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <span v-if="item.quick_reply_name" class="text-xs px-1.5 py-0.5 rounded-full bg-secondary text-secondary-foreground">
                      {{ item.quick_reply_name }}
                    </span>
                    <span v-else class="text-xs text-muted-foreground/40">—</span>
                  </TableCell>
                  <TableCell class="text-xs text-muted-foreground">{{ item.user_name || 'System' }}</TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger as-child>
                        <Button variant="ghost" size="icon-sm" class="size-8" aria-label="Activity actions">
                          <MoreHorizontal class="size-3.5" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          v-if="!item.is_done && !item.is_cancelled"
                          class="cursor-pointer"
                          @click="doMarkDone(item)"
                        >
                          <CheckCircle2 class="size-3.5 mr-2" /> Mark done
                        </DropdownMenuItem>
                        <DropdownMenuSub v-if="item.remind_at && !item.is_done && !item.is_cancelled">
                          <DropdownMenuSubTrigger>
                            <AlarmClockPlus class="size-3.5 mr-2" /> Snooze
                          </DropdownMenuSubTrigger>
                          <DropdownMenuSubContent>
                            <DropdownMenuItem
                              v-for="preset in snoozePresets"
                              :key="preset.minutes"
                              class="cursor-pointer"
                              @select="doSnooze(item, preset.minutes)"
                            >
                              {{ preset.label }}
                            </DropdownMenuItem>
                          </DropdownMenuSubContent>
                        </DropdownMenuSub>
                        <DropdownMenuItem class="text-destructive cursor-pointer" @click="confirmDeleteOne(item)">
                          <Trash2 class="size-3.5 mr-2" /> Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <!-- Pagination -->
        <div v-if="store.total > 0" class="flex items-center justify-between">
          <span class="text-sm text-muted-foreground">
            Page {{ store.page }} of {{ totalPages }} &middot; {{ store.total }} total
          </span>
          <div class="flex items-center gap-1">
            <Button variant="outline" size="sm" :disabled="store.page <= 1" @click="prevPage()">
              Previous
            </Button>
            <Button variant="outline" size="sm" :disabled="store.page >= totalPages" @click="nextPage()">
              Next
            </Button>
          </div>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :open="deleting"
      title="Delete activities"
      :description="`Delete ${deletingIds.length > 1 ? deletingIds.length + ' activities' : 'this activity'}? This cannot be undone.`"
      confirm-text="Delete"
      destructive
      @update:open="(v) => { if (!v) deleting = false }"
      @confirm="doDelete"
    />
  </div>
</template>
