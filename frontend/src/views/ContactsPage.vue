<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/composables/useApi'
import { useContactsStore, type Contact } from '@/stores/contacts'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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
import { Label } from '@/components/ui/label'
import { Plus, Pencil, Trash2, Loader2 } from '@lucide/vue'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'

const store = useContactsStore()
const search = ref('')
const page = ref(1)
const perPage = 20

const drawerOpen = ref(false)
const editingContact = ref<Contact | null>(null)
const formName = ref('')
const formEmail = ref('')
const formPhone = ref('')
const formLocation = ref('')
const formAge = ref<number | undefined>(undefined)
const formError = ref('')
const saving = ref(false)

const deletingId = ref<string | null>(null)
const deleteDialogOpen = ref(false)

onMounted(() => loadContacts())

async function loadContacts() {
  await store.fetchContacts(page.value, perPage, search.value)
}

function onSearch() {
  page.value = 1
  loadContacts()
}

function openCreate() {
  editingContact.value = null
  formName.value = ''
  formEmail.value = ''
  formPhone.value = ''
  formLocation.value = ''
  formAge.value = undefined
  formError.value = ''
  drawerOpen.value = true
}

function openEdit(contact: Contact) {
  editingContact.value = contact
  formName.value = contact.name
  formEmail.value = contact.email || ''
  formPhone.value = contact.phone || ''
  formLocation.value = contact.location || ''
  formAge.value = contact.age
  formError.value = ''
  drawerOpen.value = true
}

async function handleSave() {
  formError.value = ''
  if (!formName.value) {
    formError.value = 'Name is required'
    return
  }
  saving.value = true
  try {
    if (editingContact.value) {
      await apiClient.patch(`/api/contacts/${editingContact.value.id}`, {
        name: formName.value,
        email: formEmail.value || null,
        phone: formPhone.value || null,
        location: formLocation.value || null,
        age: formAge.value || null,
      })
    } else {
      await apiClient.post('/api/contacts', {
        name: formName.value,
        email: formEmail.value || null,
        phone: formPhone.value || null,
        location: formLocation.value || null,
        age: formAge.value || null,
      })
    }
    drawerOpen.value = false
    loadContacts()
  } catch (e: any) {
    formError.value = e.message || 'Save failed'
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  if (!deletingId.value) return
  try {
    await apiClient.delete(`/api/contacts/${deletingId.value}`)
    deletingId.value = null
    deleteDialogOpen.value = false
    loadContacts()
  } catch {}
}

function totalPages() {
  return Math.ceil(store.total / perPage) || 1
}
</script>

<template>
  <LayoutShell>
    <div class="flex flex-1 flex-col gap-4 p-4 pt-0">
      <div class="flex items-center gap-2">
        <Input
          v-model="search"
          placeholder="Search contacts..."
          class="max-w-sm"
          @keyup.enter="onSearch"
        />
        <Button variant="outline" @click="onSearch">Search</Button>
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
            <div class="mt-4 space-y-4">
              <div class="space-y-2">
                <Label for="name">Name *</Label>
                <Input id="name" v-model="formName" placeholder="Full name" />
              </div>
              <div class="space-y-2">
                <Label for="email">Email</Label>
                <Input id="email" v-model="formEmail" type="email" placeholder="Email address" />
              </div>
              <div class="space-y-2">
                <Label for="phone">Phone</Label>
                <Input id="phone" v-model="formPhone" placeholder="Phone number" />
              </div>
              <div class="space-y-2">
                <Label for="location">Location</Label>
                <Input id="location" v-model="formLocation" placeholder="Location" />
              </div>
              <div class="space-y-2">
                <Label for="age">Age</Label>
                <Input id="age" v-model.number="formAge" type="number" placeholder="Age" />
              </div>
              <div v-if="formError" class="text-sm text-destructive">{{ formError }}</div>
              <Button @click="handleSave" :disabled="saving" class="w-full">
                <Loader2 v-if="saving" class="mr-2 size-4 animate-spin" />
                {{ editingContact ? 'Update' : 'Create' }}
              </Button>
            </div>
          </SheetContent>
        </Sheet>
      </div>
      <div class="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Phone</TableHead>
              <TableHead>Location</TableHead>
              <TableHead>Age</TableHead>
              <TableHead class="w-20">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-if="store.loading">
              <TableCell colspan="6" class="text-center text-muted-foreground">
                Loading...
              </TableCell>
            </TableRow>
            <TableRow v-else-if="store.contacts.length === 0">
              <TableCell colspan="6" class="text-center text-muted-foreground">
                No contacts found
              </TableCell>
            </TableRow>
            <TableRow v-for="c in store.contacts" :key="c.id">
              <TableCell class="font-medium">{{ c.name }}</TableCell>
              <TableCell>{{ c.email }}</TableCell>
              <TableCell>{{ c.phone }}</TableCell>
              <TableCell>{{ c.location }}</TableCell>
              <TableCell>{{ c.age }}</TableCell>
              <TableCell>
                <div class="flex gap-1">
                  <Button variant="ghost" size="icon" @click="openEdit(c)">
                    <Pencil class="size-4" />
                  </Button>
                  <AlertDialog :open="deleteDialogOpen">
                    <AlertDialogTrigger as-child>
                      <Button
                        variant="ghost"
                        size="icon"
                        @click="deletingId = c.id; deleteDialogOpen = true"
                      >
                        <Trash2 class="size-4 text-destructive" />
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>Delete Contact</AlertDialogTitle>
                        <AlertDialogDescription>
                          Are you sure you want to delete this contact? This action cannot be undone.
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel @click="deletingId = null; deleteDialogOpen = false">
                          Cancel
                        </AlertDialogCancel>
                        <AlertDialogAction @click="handleDelete">Delete</AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
      <div class="flex items-center justify-between">
        <span class="text-sm text-muted-foreground">
          Page {{ page }} of {{ totalPages() }} ({{ store.total }} total)
        </span>
        <div class="flex gap-2">
          <Button variant="outline" :disabled="page <= 1" @click="page--; loadContacts()">
            Previous
          </Button>
          <Button variant="outline" :disabled="page >= totalPages()" @click="page++; loadContacts()">
            Next
          </Button>
        </div>
      </div>
    </div>
  </LayoutShell>
</template>
