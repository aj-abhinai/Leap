<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { List, LayoutGrid, Table2, Plus, Search } from '@lucide/vue'

export type ContactViewMode = 'table' | 'compact' | 'spreadsheet'

const props = withDefaults(defineProps<{
  canWrite?: boolean
}>(), {
  canWrite: true,
})

const search = defineModel<string>('search', { default: '' })
const viewMode = defineModel<ContactViewMode>('viewMode', { default: 'table' })

const emit = defineEmits<{
  search: []
  import: []
  create: []
}>()
</script>

<template>
  <div class="flex items-center gap-2">
    <div class="relative max-w-sm flex-1">
      <Search class="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        v-model="search"
        placeholder="Search contacts..."
        class="pl-8"
        @keyup.enter="emit('search')"
      />
    </div>
    <Button variant="outline" size="sm" @click="emit('search')">Search</Button>
    <Button v-if="props.canWrite" variant="outline" size="sm" @click="emit('import')">
      Import CSV
    </Button>
    <div class="flex items-center gap-1">
      <Button
        variant="ghost"
        size="icon-sm"
        :class="{ 'bg-muted': viewMode === 'table' }"
        @click="viewMode = 'table'"
        title="Table view"
      >
        <List class="size-4" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        :class="{ 'bg-muted': viewMode === 'compact' }"
        @click="viewMode = 'compact'"
        title="Compact cards"
      >
        <LayoutGrid class="size-4" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        :class="{ 'bg-muted': viewMode === 'spreadsheet' }"
        @click="viewMode = 'spreadsheet'"
        title="Spreadsheet view"
      >
        <Table2 class="size-4" />
      </Button>
    </div>
    <Button v-if="props.canWrite" @click="emit('create')">
      <Plus class="mr-2 size-4" /> Add Contact
    </Button>
  </div>
</template>
