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
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'
import { Plus, Trash2 } from '@lucide/vue'

const props = defineProps<{
  kind: 'tag' | 'status' | 'activity_type' | 'loss_reason'
  title: string
  placeholder: string
}>()

const store = useSettingsStore()

const newName = shallowRef('')
const error = shallowRef('')
const deletingId = shallowRef<string | null>(null)

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
            <TableHead class="w-16" />
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-if="items.length === 0">
            <TableCell colspan="2" class="text-center text-muted-foreground text-sm py-4">
              No {{ title.toLowerCase() }} yet
            </TableCell>
          </TableRow>
          <TableRow v-for="item in items" :key="item.id">
            <TableCell>{{ item.name }}</TableCell>
            <TableCell>
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
    </CardContent>
  </Card>
</template>
