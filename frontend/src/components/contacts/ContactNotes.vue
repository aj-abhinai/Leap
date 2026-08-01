<script setup lang="ts">
import { onMounted, ref, shallowRef } from 'vue'
import { apiClient } from '@/composables/useApi'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Card, CardHeader, CardContent } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel,
  AlertDialogContent, AlertDialogDescription, AlertDialogFooter,
  AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Plus, MoreHorizontal, Trash2 } from '@lucide/vue'
import { formatDateTime } from '@/utils/time'

interface Note {
  id: string
  contact_id: string
  user_id?: string
  user_name?: string
  note: string
  created_at: string
  updated_at: string
}

const props = defineProps<{ contactId: string }>()

const auth = useAuthStore()
const notes = shallowRef<Note[]>([])
const loading = shallowRef(false)
const isAdding = shallowRef(false)
const newNote = shallowRef('')
const saving = shallowRef(false)
const noteToDelete = shallowRef<string | null>(null)
const deleteDialogOpen = shallowRef(false)

onMounted(() => fetchNotes())

async function fetchNotes() {
  loading.value = true
  try {
    const res = await apiClient.get(`/api/contacts/${props.contactId}/notes`)
    notes.value = res.data
  } catch (e: any) {
    toast.error(e.message || 'Failed to load notes')
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!newNote.value.trim() || saving.value) return
  saving.value = true
  try {
    await apiClient.post(`/api/contacts/${props.contactId}/notes`, { note: newNote.value })
    newNote.value = ''
    isAdding.value = false
    await fetchNotes()
    toast.success('Note added')
  } catch (e: any) {
    toast.error(e.message || 'Failed to save note')
  } finally {
    saving.value = false
  }
}

function confirmDelete(noteId: string) {
  noteToDelete.value = noteId
  deleteDialogOpen.value = true
}

async function handleDelete() {
  if (!noteToDelete.value) return
  try {
    await apiClient.delete(`/api/contacts/${props.contactId}/notes/${noteToDelete.value}`)
    noteToDelete.value = null
    deleteDialogOpen.value = false
    await fetchNotes()
    toast.success('Note deleted')
  } catch (e: any) {
    toast.error(e.message || 'Failed to delete note')
  }
}

function canDelete(note: Note): boolean {
  return note.user_id === auth.user?.id
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold">Notes</h3>
      <Button v-if="!isAdding" variant="outline" size="sm" @click="isAdding = true">
        <Plus class="size-4 mr-1" /> Add Note
      </Button>
    </div>

    <div v-if="loading" class="space-y-3">
      <Skeleton v-for="i in 3" :key="i" class="h-20 w-full" />
    </div>

    <div v-if="isAdding" class="space-y-3 rounded-lg border p-3">
      <Textarea v-model="newNote" placeholder="Write a note..." class="min-h-24" />
      <div class="flex justify-end gap-2">
        <Button variant="outline" size="sm" @click="isAdding = false; newNote = ''">Cancel</Button>
        <Button size="sm" :disabled="!newNote.trim() || saving" @click="handleSave">
          {{ saving ? 'Saving...' : 'Save' }}
        </Button>
      </div>
    </div>

    <div v-if="!loading && !isAdding && notes.length === 0" class="rounded-lg border border-dashed p-8 text-center">
      <p class="text-sm text-muted-foreground">No notes yet. Add the first one.</p>
    </div>

    <div v-else-if="!loading" class="space-y-3">
      <Card v-for="note in notes" :key="note.id">
        <CardHeader class="pb-1 pt-3 px-4">
          <div class="flex items-center justify-between">
            <div class="text-sm">
              <span class="font-medium">{{ note.user_name || 'Unknown' }}</span>
              <span class="text-muted-foreground"> &middot; {{ formatDateTime(note.created_at) }}</span>
            </div>
            <DropdownMenu v-if="canDelete(note)">
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon-sm" class="h-6 w-6">
                  <MoreHorizontal class="size-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem class="text-destructive cursor-pointer" @click="confirmDelete(note.id)">
                  <Trash2 class="size-3.5 mr-2" /> Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </CardHeader>
        <CardContent class="pb-3 px-4">
          <p class="text-sm whitespace-pre-wrap">{{ note.note }}</p>
        </CardContent>
      </Card>
    </div>

    <AlertDialog :open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Note</AlertDialogTitle>
          <AlertDialogDescription>Are you sure? This cannot be undone.</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel @click="deleteDialogOpen = false; noteToDelete = null">Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" @click="handleDelete">Delete</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
