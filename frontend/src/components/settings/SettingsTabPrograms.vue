<script setup lang="ts">
import { onMounted, ref, shallowRef } from 'vue'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Plus, Archive, RotateCcw, BookOpen } from '@lucide/vue'
import { formatCurrency } from '@/utils/format'
import { errorMessage } from '@/utils/errors'
import { listProgramsManage, createProgram as apiCreateProgram, updateProgram, archiveProgram as apiArchiveProgram, restoreProgram as apiRestoreProgram, type Program } from '@/api/programs'

const programs = shallowRef<Program[]>([])
const newName = shallowRef('')
const newDesc = shallowRef('')
const newPrice = shallowRef<number | undefined>(undefined)
const newError = shallowRef('')
const creating = shallowRef(false)
// Deep-reactive so v-model on nested fields (name/desc/price) updates the UI.
const editing = ref<{ id: string; name: string; desc: string; price: number } | null>(null)

onMounted(() => loadPrograms())

async function loadPrograms() {
  try {
    const res = await listProgramsManage()
    programs.value = res.data
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to load programs'))
  }
}

async function createProgram() {
  newError.value = ''
  if (!newName.value) {
    newError.value = 'Name is required'
    return
  }
  if (newPrice.value === undefined || newPrice.value < 0) {
    newError.value = 'Price is required and cannot be negative'
    return
  }
  creating.value = true
  try {
    await apiCreateProgram({
      name: newName.value,
      description: newDesc.value,
      price: newPrice.value,
    })
    toast.success('Program created')
    newName.value = ''
    newDesc.value = ''
    newPrice.value = undefined
    loadPrograms()
  } catch (e) {
    newError.value = errorMessage(e, 'Failed to create program')
  } finally {
    creating.value = false
  }
}

function startEdit(p: Program) {
  editing.value = { id: p.id, name: p.name, desc: p.description || '', price: p.price }
}

async function saveEdit() {
  if (!editing.value) return
  if (!editing.value.name) {
    toast.error('Name is required')
    return
  }
  try {
    await updateProgram(editing.value.id, {
      name: editing.value.name,
      description: editing.value.desc,
      price: editing.value.price,
    })
    toast.success('Program updated')
    editing.value = null
    loadPrograms()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to update program'))
  }
}

async function archiveProgram(id: string) {
  try {
    await apiArchiveProgram(id)
    toast.success('Program archived')
    loadPrograms()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to archive program'))
  }
}

async function restoreProgram(id: string) {
  try {
    await apiRestoreProgram(id)
    toast.success('Program restored')
    loadPrograms()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to restore program'))
  }
}
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardHeader>
        <CardTitle class="text-base">Add Program</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex flex-wrap items-end gap-2">
          <div class="space-y-1">
            <Label for="pname" class="text-xs">Name *</Label>
            <Input id="pname" v-model="newName" placeholder="Program name" class="min-w-40" />
          </div>
          <div class="space-y-1">
            <Label for="pdesc" class="text-xs">Description</Label>
            <Input id="pdesc" v-model="newDesc" placeholder="Description" class="min-w-40" />
          </div>
          <div class="space-y-1">
            <Label for="pprice" class="text-xs">Price *</Label>
            <Input id="pprice" v-model.number="newPrice" type="number" min="0" placeholder="0" class="w-32" />
          </div>
          <Button @click="createProgram" :disabled="creating">
            <Plus class="mr-2 size-4" /> Add Program
          </Button>
        </div>
        <div v-if="newError" class="mt-2 text-sm text-destructive">{{ newError }}</div>
        <p class="mt-2 text-xs text-muted-foreground">
          Lead values are snapshots of the catalog price at creation time; changing a price
          never rewrites existing leads.
        </p>
      </CardContent>
    </Card>
    <div v-if="programs.length === 0" class="flex flex-col items-center justify-center py-12 text-center">
      <BookOpen class="size-10 text-muted-foreground/40 mb-3" />
      <p class="text-sm text-muted-foreground">No programs configured</p>
    </div>
    <Card v-for="p in programs" :key="p.id">
      <CardHeader class="flex flex-row items-center justify-between pb-2">
        <div>
          <CardTitle class="text-base flex items-center gap-2">
            {{ p.name }}
            <Badge v-if="p.archived" variant="secondary" class="text-xs">Archived</Badge>
          </CardTitle>
          <p v-if="p.description" class="text-sm text-muted-foreground mt-0.5">{{ p.description }}</p>
          <p class="text-sm font-medium mt-1">{{ formatCurrency(p.price) }}</p>
        </div>
        <div v-if="!editing || editing.id !== p.id" class="flex gap-1">
          <Button variant="outline" size="sm" @click="startEdit(p)">Edit</Button>
          <Button v-if="!p.archived" variant="ghost" size="sm" @click="archiveProgram(p.id)">
            <Archive class="mr-1 size-3.5" /> Archive
          </Button>
          <Button v-else variant="ghost" size="sm" @click="restoreProgram(p.id)">
            <RotateCcw class="mr-1 size-3.5" /> Restore
          </Button>
        </div>
      </CardHeader>
      <CardContent v-if="editing && editing.id === p.id" class="flex flex-wrap items-end gap-2">
        <div class="space-y-1">
          <Label class="text-xs">Name</Label>
          <Input v-model="editing.name" class="min-w-40" />
        </div>
        <div class="space-y-1">
          <Label class="text-xs">Description</Label>
          <Input v-model="editing.desc" class="min-w-40" />
        </div>
        <div class="space-y-1">
          <Label class="text-xs">Price</Label>
          <Input v-model.number="editing.price" type="number" min="0" class="w-32" />
        </div>
        <Button @click="saveEdit">Save</Button>
        <Button variant="ghost" @click="editing = null">Cancel</Button>
      </CardContent>
    </Card>
  </div>
</template>
