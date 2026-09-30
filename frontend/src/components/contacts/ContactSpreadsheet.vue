<script setup lang="ts">
import { type Contact } from '@/stores/contacts'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

// Selection is opt-in: a host with a bulk action passes the selected id set
// (and the header check state) and receives toggle events; without them the
// sheet renders read-only.
defineProps<{
  contacts: Contact[]
  selectedIds?: Set<string>
  checkState?: boolean | 'indeterminate'
}>()

const emit = defineEmits<{
  rowClick: [id: string]
  toggle: [id: string]
  togglePage: []
}>()
</script>

<template>
  <div class="w-full min-w-0 overflow-x-auto rounded-lg border">
    <Table>
      <TableHeader>
        <TableRow class="hover:bg-transparent">
          <TableHead v-if="selectedIds" class="w-10">
            <Checkbox
              :model-value="checkState ?? false"
              class="size-4"
              aria-label="Select all on this page"
              @click.stop
              @update:model-value="emit('togglePage')"
            />
          </TableHead>
          <TableHead>Name</TableHead>
          <TableHead>Phone</TableHead>
          <TableHead>Email</TableHead>
          <TableHead>Location</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Tags</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-if="contacts.length === 0">
          <TableCell :colspan="selectedIds ? 7 : 6" class="text-center text-muted-foreground py-8">
            No contacts to display
          </TableCell>
        </TableRow>
        <TableRow
          v-for="c in contacts"
          :key="c.id"
          class="cursor-pointer hover:bg-muted/50"
          @click="emit('rowClick', c.id)"
        >
          <TableCell v-if="selectedIds" @click.stop>
            <Checkbox
              :model-value="selectedIds.has(c.id)"
              class="size-4"
              :aria-label="`Select ${c.name}`"
              @update:model-value="emit('toggle', c.id)"
            />
          </TableCell>
          <TableCell class="font-medium whitespace-nowrap">{{ c.name }}</TableCell>
          <TableCell class="text-muted-foreground whitespace-nowrap">{{ c.phone || '–' }}</TableCell>
          <TableCell class="text-muted-foreground whitespace-nowrap">{{ c.email || '–' }}</TableCell>
          <TableCell class="text-muted-foreground">{{ c.location || '–' }}</TableCell>
          <TableCell>
            <Badge v-if="c.status" variant="secondary" class="text-xs">{{ c.status.name }}</Badge>
            <span v-else class="text-muted-foreground">–</span>
          </TableCell>
          <TableCell>
            <div class="flex flex-wrap gap-1">
              <Badge v-for="t in (c.tags || [])" :key="t.id" variant="outline" class="text-xs whitespace-nowrap">
                {{ t.name }}
              </Badge>
              <span v-if="!c.tags?.length" class="text-muted-foreground">–</span>
            </div>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </div>
</template>
