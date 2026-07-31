<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { apiClient } from '@/composables/useApi'
import { useContactsStore, type Contact } from '@/stores/contacts'
import { toast } from 'vue-sonner'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { Plus, Pencil, Trash2, Search, Users, ChevronLeft, ChevronRight, FolderKanban } from '@lucide/vue'
import { Badge } from '@/components/ui/badge'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import ContactForm from '@/components/contacts/ContactForm.vue'

const store = useContactsStore()
const router = useRouter()
const search = ref('')
const page = ref(1)
const perPage = 20

const drawerOpen = ref(false)
const editingContact = ref<Contact | null>(null)

const deletingId = ref<string | null>(null)
const deleteDialogOpen = ref(false)

const totalPages = computed(() => Math.ceil(store.total / perPage) || 1)

const deletingContactName = computed(() => {
  if (!deletingId.value) return ''
  const contact = store.contacts.find(c => c.id === deletingId.value)
  return contact?.name || ''
})

onMounted(() => {
  loadContacts().catch(() => {})
})

async function loadContacts() {
  await store.fetchContacts(page.value, perPage, search.value)
}

function onSearch() {
  page.value = 1
  loadContacts().catch(() => {})
}

function nextPage() {
  page.value++
  loadContacts().catch(() => {})
}

function prevPage() {
  page.value--
  loadContacts().catch(() => {})
}

function openCreate() {
  editingContact.value = null
  drawerOpen.value = true
}

function openEdit(contact: Contact) {
  editingContact.value = contact
  drawerOpen.value = true
}

async function handleSave(body: Record<string, any>) {
  try {
    if (editingContact.value) {
      await apiClient.patch(`/api/contacts/${editingContact.value.id}`, body)
      toast.success('Contact updated')
    } else {
      await apiClient.post('/api/contacts', body)
      toast.success('Contact created')
    }
    drawerOpen.value = false
    loadContacts().catch(() => {})
  } catch (e: any) {
    toast.error(e.message || 'Failed to save contact')
  }
}

function confirmDelete(id: string) {
  deletingId.value = id
  deleteDialogOpen.value = true
}

async function handleDelete() {
  if (!deletingId.value) return
  try {
    await apiClient.delete(`/api/contacts/${deletingId.value}`)
    toast.success('Contact deleted')
    deletingId.value = null
    deleteDialogOpen.value = false
    loadContacts().catch(() => {})
  } catch (e: any) {
    toast.error(e.message || 'Failed to delete contact')
  }
}

function getInitials(name: string): string {
  return name
    .split(' ')
    .map(n => n.charAt(0))
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

function getAvatarColor(name: string): string {
  const colors = [
    'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
    'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
    'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
    'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-300',
    'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300',
    'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300',
  ]
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return colors[Math.abs(hash) % colors.length]
}
</script>

<template>
  <LayoutShell>
    <div class="flex flex-1 flex-col gap-4 p-6 pt-2">
      <div class="flex items-center gap-2">
        <div class="relative flex-1 max-w-sm">
          <Search class="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            v-model="search"
            placeholder="Search contacts..."
            class="pl-8"
            @keyup.enter="onSearch"
          />
        </div>
        <Button variant="outline" size="sm" @click="onSearch">Search</Button>
        <Sheet v-model:open="drawerOpen">
          <SheetTrigger as-child>
            <Button @click="openCreate">
              <Plus class="mr-2 size-4" /> Add Contact
            </Button>
          </SheetTrigger>
          <SheetContent>
            <SheetHeader>
              <SheetTitle>{{ editingContact ? 'Edit Contact' : 'Add Contact' }}</SheetTitle>
              <SheetDescription>Fill in the contact details below.</SheetDescription>
            </SheetHeader>
            <ContactForm
              :key="editingContact?.id ?? 'create'"
              :editing-contact="editingContact"
              @save="handleSave"
            />
          </SheetContent>
        </Sheet>
      </div>

      <div class="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow class="hover:bg-transparent">
              <TableHead class="w-12" />
              <TableHead>Name</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Phone</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Tags</TableHead>
              <TableHead>Location</TableHead>
              <TableHead>Age</TableHead>
              <TableHead class="w-24">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <template v-if="store.loading">
              <TableRow v-for="i in 8" :key="i">
                <TableCell v-for="j in 9" :key="j">
                  <Skeleton class="h-5 w-full" />
                </TableCell>
              </TableRow>
            </template>
            <TableRow v-else-if="store.contacts.length === 0">
              <TableCell colspan="9">
                <div class="flex flex-col items-center justify-center py-12 text-center">
                  <Users class="size-10 text-muted-foreground/40 mb-3" />
                  <p class="text-sm font-medium text-muted-foreground">No contacts found</p>
                  <p class="text-xs text-muted-foreground/60 mt-1">
                    {{ search ? 'Try a different search term' : 'Add your first contact to get started' }}
                  </p>
                </div>
              </TableCell>
            </TableRow>
            <TableRow v-else v-for="c in store.contacts" :key="c.id" class="group">
              <TableCell>
                <div
                  class="flex size-8 items-center justify-center rounded-full text-xs font-medium"
                  :class="getAvatarColor(c.name)"
                >
                  {{ getInitials(c.name) }}
                </div>
              </TableCell>
              <TableCell class="font-medium">{{ c.name }}</TableCell>
              <TableCell class="text-muted-foreground">{{ c.email || '—' }}</TableCell>
              <TableCell class="text-muted-foreground">{{ c.phone || '—' }}</TableCell>
              <TableCell>
                <Badge v-if="c.status" variant="secondary">{{ c.status.name }}</Badge>
                <span v-else class="text-muted-foreground">—</span>
              </TableCell>
              <TableCell>
                <div class="flex flex-wrap gap-1">
                  <Badge v-for="t in (c.tags || [])" :key="t.id" variant="outline" class="text-xs">
                    {{ t.name }}
                  </Badge>
                  <span v-if="!c.tags?.length" class="text-muted-foreground">—</span>
                </div>
              </TableCell>
              <TableCell class="text-muted-foreground">{{ c.location || '—' }}</TableCell>
              <TableCell class="text-muted-foreground">{{ c.age || '—' }}</TableCell>
              <TableCell>
                <div class="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    @click="router.push({ name: 'Leads', query: { contact: c.id } })"
                    title="Create lead from contact"
                  >
                    <FolderKanban class="size-3.5" />
                  </Button>
                  <Button variant="ghost" size="icon-sm" @click="openEdit(c)">
                    <Pencil class="size-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    @click="confirmDelete(c.id)"
                  >
                    <Trash2 class="size-3.5" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <AlertDialog :open="deleteDialogOpen">
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Contact</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete <strong>{{ deletingContactName }}</strong>? This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel @click="deletingId = null; deleteDialogOpen = false">
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction variant="destructive" @click="handleDelete">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <div class="flex items-center justify-between">
        <span class="text-sm text-muted-foreground">
          Page {{ page }} of {{ totalPages }} &middot; {{ store.total }} total
        </span>
        <div class="flex items-center gap-1">
          <Button variant="outline" size="sm" :disabled="page <= 1" @click="prevPage">
            <ChevronLeft class="size-4" />
            Previous
          </Button>
          <Button variant="outline" size="sm" :disabled="page >= totalPages" @click="nextPage">
            Next
            <ChevronRight class="size-4" />
          </Button>
        </div>
      </div>
    </div>
  </LayoutShell>
</template>
