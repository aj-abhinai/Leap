<script setup lang="ts">
import { ref, shallowRef, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { listNotes, addNote, deleteNote, type ContactNote } from '@/api/contacts'
import { Button } from '@/components/ui/button'
import { Card, CardHeader, CardContent } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import PageState from '@/components/PageState.vue'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel,
  AlertDialogContent, AlertDialogDescription, AlertDialogFooter,
  AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Plus, MoreHorizontal, Trash2, Loader2 } from '@lucide/vue'
import { formatDateTime } from '@/utils/time'
import { errorMessage } from '@/utils/errors'

interface Note extends ContactNote {}

const props = defineProps<{ contactId: string }>()

const auth = useAuthStore()
const notes = shallowRef<Note[]>([])
const loading = shallowRef(false)
const isAdding = shallowRef(false)
const newNote = shallowRef('')
const saving = shallowRef(false)
const noteToDelete = shallowRef<string | null>(null)
const deleteDialogOpen = shallowRef(false)
const total = shallowRef(0)
const page = shallowRef(1)
const loadingMore = shallowRef(false)
const PAGE_SIZE = 50

// Watch the prop: the component instance is reused when the route param
// changes, so onMounted-only fetching would leave stale notes behind.
let fetchSeq = 0
watch(() => props.contactId, () => fetchNotes(), { immediate: true })

async function fetchNotes() {
  const seq = ++fetchSeq
  loading.value = true
  page.value = 1
  try {
    const res = await listNotes(props.contactId, { page: 1, perPage: PAGE_SIZE })
    if (seq !== fetchSeq) return
    notes.value = res.data
    total.value = res.meta?.total ?? res.data.length
  } catch (e) {
    if (seq !== fetchSeq) return
    toast.error(errorMessage(e, 'Failed to load notes'))
  } finally {
    if (seq === fetchSeq) loading.value = false
  }
}

// loadMore fetches the next page and appends it; the server caps each page at
// 50 so the feed grows in bounded chunks.
async function loadMore() {
  if (loadingMore.value) return
  loadingMore.value = true
  const seq = fetchSeq
  try {
    const res = await listNotes(props.contactId, { page: page.value + 1, perPage: PAGE_SIZE })
    if (seq !== fetchSeq) return
    notes.value = [...notes.value, ...res.data]
    total.value = res.meta?.total ?? total.value
    page.value++
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to load more notes'))
  } finally {
    loadingMore.value = false
  }
}

async function handleSave() {
  if (!newNote.value.trim() || saving.value) return
  saving.value = true
  try {
    await addNote(props.contactId, newNote.value)
    newNote.value = ''
    isAdding.value = false
    await fetchNotes()
    toast.success('Note added')
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to save note'))
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
    await deleteNote(props.contactId, noteToDelete.value)
    noteToDelete.value = null
    deleteDialogOpen.value = false
    await fetchNotes()
    toast.success('Note deleted')
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to delete note'))
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

    <div v-if="isAdding" class="space-y-3 rounded-lg border p-3">
      <Textarea v-model="newNote" placeholder="Write a note..." class="min-h-24" />
      <div class="flex justify-end gap-2">
        <Button variant="outline" size="sm" @click="isAdding = false; newNote = ''">Cancel</Button>
        <Button size="sm" :disabled="!newNote.trim() || saving" @click="handleSave">
          {{ saving ? 'Saving...' : 'Save' }}
        </Button>
      </div>
    </div>

    <PageState
      :loading="loading"
      :empty="!isAdding && notes.length === 0"
      empty-title="No notes yet. Add the first one."
      :skeleton-count="3"
      skeleton-class="h-20 w-full"
    >
      <div class="space-y-3">
        <Card v-for="note in notes" :key="note.id">
        <CardHeader class="pb-1 pt-3 px-4">
          <div class="flex items-center justify-between">
            <div class="text-sm">
              <span class="font-medium">{{ note.user_name || 'Unknown' }}</span>
              <span class="text-muted-foreground"> &middot; {{ formatDateTime(note.created_at) }}</span>
            </div>
            <DropdownMenu v-if="canDelete(note)">
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon-sm" class="size-8" aria-label="Note actions">
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
      <div v-if="notes.length < total" class="flex justify-center">
        <Button variant="outline" size="sm" :disabled="loadingMore" @click="loadMore">
          <Loader2 v-if="loadingMore" class="size-3.5 mr-1 animate-spin" />
          Load more
        </Button>
      </div>
      </div>
    </PageState>

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
