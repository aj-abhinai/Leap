<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'
import { Plus, Trash2, Pencil } from '@lucide/vue'

const behaviorLabels: Record<string, string> = {
  log: 'Log only',
  next: 'Schedule next',
  close_lost: 'Close lost',
}

const props = defineProps<{
  kind: 'tag' | 'status' | 'activity_type' | 'loss_reason'
  title: string
  placeholder: string
}>()

const store = useSettingsStore()

const newName = shallowRef('')
const error = shallowRef('')
const deletingId = shallowRef<string | null>(null)
const editing = shallowRef<{ id: string; groupName: string; sortOrder: number; behavior: string } | null>(null)
const editError = shallowRef('')
const savingEdit = shallowRef(false)

const items = computed(() => {
  switch (props.kind) {
    case 'status': return store.statuses
    case 'activity_type': return store.activityTypes
    case 'loss_reason': return store.lossReasons
    default: return store.tags
  }
})

const kindLabel = computed(() => {
  switch (props.kind) {
    case 'status': return 'Status'
    case 'activity_type': return 'Activity type'
    case 'loss_reason': return 'Loss reason'
    default: return 'Tag'
  }
})

async function add() {
  error.value = ''
  if (!newName.value.trim()) {
    error.value = 'Name is required'
    return
  }
  try {
    await store.createTag(newName.value.trim(), props.kind)
    toast.success(`${kindLabel.value} created`)
    newName.value = ''
  } catch (e: any) {
    error.value = e.message || `Failed to create ${kindLabel.value.toLowerCase()}`
  }
}

function confirmDelete(id: string) {
  deletingId.value = id
}

async function remove() {
  if (!deletingId.value) return
  try {
    await store.deleteTag(deletingId.value)
    toast.success('Deleted')
  } catch (e: any) {
    toast.error(e.message || 'Failed to delete')
  } finally {
    deletingId.value = null
  }
}

function openEdit(id: string) {
  const item = items.value.find((i) => i.id === id)
  if (!item) return
  editing.value = {
    id,
    groupName: item.group_name || '',
    sortOrder: item.sort_order,
    behavior: item.behavior,
  }
  editError.value = ''
}

async function saveEdit() {
  if (!editing.value) return
  savingEdit.value = true
  editError.value = ''
  try {
    await store.updateTag(editing.value.id, {
      group_name: editing.value.groupName.trim(),
      sort_order: editing.value.sortOrder,
      behavior: editing.value.behavior,
    })
    toast.success('Status updated')
    editing.value = null
  } catch (e: any) {
    editError.value = e.message || 'Failed to update'
  } finally {
    savingEdit.value = false
  }
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle class="text-base">{{ title }}</CardTitle>
    </CardHeader>
    <CardContent>
      <div class="flex gap-2 mb-4">
        <Input v-model="newName" :placeholder="placeholder" @keyup.enter="add" />
        <Button @click="add">
          <Plus class="mr-2 size-4" /> Add
        </Button>
      </div>
      <div v-if="error" class="mb-2 text-sm text-destructive">{{ error }}</div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead v-if="props.kind === 'status'">Group</TableHead>
            <TableHead v-if="props.kind === 'status'">Behavior</TableHead>
            <TableHead class="w-24" />
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-if="items.length === 0">
            <TableCell :colspan="props.kind === 'status' ? 4 : 2" class="text-center text-muted-foreground text-sm py-4">
              No {{ title.toLowerCase() }} yet
            </TableCell>
          </TableRow>
          <TableRow v-for="item in items" :key="item.id">
            <TableCell>{{ item.name }}</TableCell>
            <TableCell v-if="props.kind === 'status'">
              <span v-if="item.group_name" class="text-xs text-muted-foreground">{{ item.group_name }}</span>
              <span v-else class="text-xs text-muted-foreground/50">—</span>
            </TableCell>
            <TableCell v-if="props.kind === 'status'">
              <span v-if="item.behavior" class="text-xs">{{ behaviorLabels[item.behavior] || item.behavior }}</span>
              <span v-else class="text-xs text-muted-foreground/50">—</span>
            </TableCell>
            <TableCell class="text-right">
              <Button
                v-if="props.kind === 'status'"
                variant="ghost"
                size="icon-sm"
                :title="`Edit ${item.name}`"
                :aria-label="`Edit ${item.name}`"
                @click="openEdit(item.id)"
              >
                <Pencil class="size-3.5" />
              </Button>
              <Button variant="ghost" size="icon-sm" :title="`Delete ${item.name}`" :aria-label="`Delete ${item.name}`" @click="confirmDelete(item.id)">
                <Trash2 class="size-3.5" />
              </Button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>

      <ConfirmDialog
        :open="!!deletingId"
        title="Delete"
        :description="`Delete ${props.title.toLowerCase()} &ldquo;${items.find((i) => i.id === deletingId)?.name ?? ''}&rdquo;? This cannot be undone.`"
        confirm-text="Delete"
        destructive
        @update:open="(v) => { if (!v) deletingId = null }"
        @confirm="remove"
      />

      <Dialog :open="!!editing" @update:open="(v) => { if (!v) editing = null }">
        <DialogContent class="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Edit status</DialogTitle>
            <DialogDescription>Configure the outcome group and follow-up behavior.</DialogDescription>
          </DialogHeader>
          <div v-if="editing" class="space-y-4 py-2">
            <div class="space-y-2">
              <Label class="text-xs">Group</Label>
              <Input v-model="editing.groupName" placeholder="e.g. Connected, Not Connected, Heard Details" />
              <p class="text-xs text-muted-foreground">
                Chips are shown grouped under this label in the activity form.
              </p>
            </div>
            <div class="space-y-2">
              <Label class="text-xs">Sort order</Label>
              <Input v-model.number="editing.sortOrder" type="number" />
              <p class="text-xs text-muted-foreground">Lower numbers appear first within the palette.</p>
            </div>
            <div class="space-y-2">
              <Label class="text-xs">Behavior</Label>
              <Select v-model="editing.behavior">
                <SelectTrigger>
                  <SelectValue placeholder="Select behavior" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="log">Log only — record the outcome</SelectItem>
                  <SelectItem value="next">Schedule next — log + create the next task</SelectItem>
                  <SelectItem value="close_lost">Close lost — log + move lead to Closed Lost</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <p v-if="editError" class="text-sm text-destructive">{{ editError }}</p>
          </div>
          <DialogFooter>
            <Button variant="ghost" @click="editing = null">Cancel</Button>
            <Button :disabled="savingEdit" @click="saveEdit">
              {{ savingEdit ? 'Saving…' : 'Save' }}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </CardContent>
  </Card>
</template>
