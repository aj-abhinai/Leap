<script setup lang="ts">
import { onMounted, ref } from 'vue'
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
import { Plus, Trash2, Tag, Flag } from '@lucide/vue'

const store = useSettingsStore()

const newTagName = ref('')
const newStatusName = ref('')
const tagError = ref('')
const statusError = ref('')

onMounted(() => {
  store.fetchTags()
})

async function addTag() {
  tagError.value = ''
  if (!newTagName.value.trim()) {
    tagError.value = 'Name is required'
    return
  }
  try {
    await store.createTag(newTagName.value.trim(), 'tag')
    toast.success('Tag created')
    newTagName.value = ''
  } catch (e: any) {
    tagError.value = e.message || 'Failed to create tag'
  }
}

async function addStatus() {
  statusError.value = ''
  if (!newStatusName.value.trim()) {
    statusError.value = 'Name is required'
    return
  }
  try {
    await store.createTag(newStatusName.value.trim(), 'status')
    toast.success('Status created')
    newStatusName.value = ''
  } catch (e: any) {
    statusError.value = e.message || 'Failed to create status'
  }
}

async function removeTag(id: string) {
  try {
    await store.deleteTag(id)
    toast.success('Deleted')
  } catch (e: any) {
    toast.error(e.message || 'Failed to delete')
  }
}
</script>

<template>
  <div class="grid gap-6 md:grid-cols-2">
    <Card>
      <CardHeader>
        <CardTitle class="text-base flex items-center gap-2">
          <Tag class="size-4" /> Tags
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex gap-2 mb-4">
          <Input v-model="newTagName" placeholder="Tag name" @keyup.enter="addTag" />
          <Button @click="addTag">
            <Plus class="mr-2 size-4" /> Add
          </Button>
        </div>
        <div v-if="tagError" class="mb-2 text-sm text-destructive">{{ tagError }}</div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead class="w-16" />
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-if="store.tags.length === 0">
              <TableCell colspan="2" class="text-center text-muted-foreground text-sm py-4">
                No tags yet
              </TableCell>
            </TableRow>
            <TableRow v-for="t in store.tags" :key="t.id">
              <TableCell>{{ t.name }}</TableCell>
              <TableCell>
                <Button variant="ghost" size="icon-sm" @click="removeTag(t.id)">
                  <Trash2 class="size-3.5" />
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle class="text-base flex items-center gap-2">
          <Flag class="size-4" /> Statuses
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex gap-2 mb-4">
          <Input v-model="newStatusName" placeholder="Status name" @keyup.enter="addStatus" />
          <Button @click="addStatus">
            <Plus class="mr-2 size-4" /> Add
          </Button>
        </div>
        <div v-if="statusError" class="mb-2 text-sm text-destructive">{{ statusError }}</div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead class="w-16" />
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-if="store.statuses.length === 0">
              <TableCell colspan="2" class="text-center text-muted-foreground text-sm py-4">
                No statuses yet
              </TableCell>
            </TableRow>
            <TableRow v-for="s in store.statuses" :key="s.id">
              <TableCell>{{ s.name }}</TableCell>
              <TableCell>
                <Button variant="ghost" size="icon-sm" @click="removeTag(s.id)">
                  <Trash2 class="size-3.5" />
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  </div>
</template>
