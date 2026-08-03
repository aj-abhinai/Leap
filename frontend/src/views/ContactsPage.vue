<script setup lang="ts">
import { computed, onMounted, ref, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import { apiClient } from '@/composables/useApi'
import { useContactsStore, type Contact } from '@/stores/contacts'
import { useRBACStore } from '@/stores/rbac'
import { toast } from 'vue-sonner'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Button } from '@/components/ui/button'
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
} from '@/components/ui/sheet'
import { FolderKanban, Pencil, Trash2, Users } from '@lucide/vue'
import { Badge } from '@/components/ui/badge'
import ContactForm from '@/components/contacts/ContactForm.vue'
import ContactCompactCard from '@/components/contacts/ContactCompactCard.vue'
import ContactSpreadsheet from '@/components/contacts/ContactSpreadsheet.vue'
import CsvImport from '@/components/contacts/CsvImport.vue'
import ContactsToolbar, { type ContactViewMode } from '@/components/contacts/ContactsToolbar.vue'
import ContactsPagination from '@/components/contacts/ContactsPagination.vue'
import ContactDeleteDialog from '@/components/contacts/ContactDeleteDialog.vue'
import { getAvatarColor, getInitials } from '@/utils/avatar'

const store = useContactsStore()
const router = useRouter()
const rbac = useRBACStore()
const search = shallowRef('')
const page = shallowRef(1)
const perPage = 20

const drawerOpen = shallowRef(false)
const editingContact = ref<Contact | null>(null)

const deletingId = shallowRef<string | null>(null)
const deleteDialogOpen = shallowRef(false)

const totalPages = computed(() => Math.ceil(store.total / perPage) || 1)

const deletingContactName = computed(() => {
  if (!deletingId.value) return ''
  const contact = store.contacts.find(c => c.id === deletingId.value)
  return contact?.name || ''
})

const viewMode = shallowRef<ContactViewMode>('table')

onMounted(() => {
  const saved = localStorage.getItem('crm-contact-view')
  if (saved === 'table' || saved === 'compact' || saved === 'spreadsheet') {
    viewMode.value = saved
  }
  loadContacts().catch(() => {})
})

watch(viewMode, (v) => localStorage.setItem('crm-contact-view', v))

const importOpen = shallowRef(false)

async function loadContacts() {
  await store.fetchContacts(page.value, perPage, search.value)
  if (store.contacts.length === 0 && page.value > 1 && store.total > 0) {
    page.value = Math.max(1, Math.ceil(store.total / perPage))
    await store.fetchContacts(page.value, perPage, search.value)
  }
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
      const res = await apiClient.post('/api/contacts', body)
      toast.success('Contact created')
      if (res.data?.warnings?.length) {
        toast.warning(res.data.warnings.join('; '))
      }
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
</script>

<template>
  <LayoutShell>
    <div class="flex flex-1 flex-col gap-4 p-6 pt-2">
      <ContactsToolbar
        v-model:search="search"
        v-model:view-mode="viewMode"
        :can-write="rbac.can('contact:write')"
        @search="onSearch"
        @import="importOpen = true"
        @create="openCreate"
      />

      <template v-if="viewMode === 'table'">
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
                  <div class="flex gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      @click="router.push({ name: 'Leads', query: { contact: c.id } })"
                      title="Create lead from contact"
                    >
                      <FolderKanban class="size-3.5" />
                    </Button>
                    <Button variant="ghost" size="icon-sm" @click="openEdit(c)" v-if="rbac.can('contact:write')">
                      <Pencil class="size-3.5" />
                    </Button>
                    <Button
                      v-if="rbac.can('contact:delete')"
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
      </template>

      <div v-else-if="viewMode === 'compact'" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <div v-if="store.loading">
          <div v-for="i in 8" :key="i" class="rounded-lg border p-4 space-y-3">
            <Skeleton class="h-5 w-3/4" />
            <Skeleton class="h-4 w-1/2" />
            <Skeleton class="h-4 w-full" />
          </div>
        </div>
        <div v-else-if="store.contacts.length === 0" class="col-span-full flex flex-col items-center justify-center py-12 text-center">
          <Users class="size-10 text-muted-foreground/40 mb-3" />
          <p class="text-sm font-medium text-muted-foreground">No contacts found</p>
        </div>
        <ContactCompactCard
          v-for="c in store.contacts"
          :key="c.id"
          :contact="c"
          @click="router.push({ name: 'ContactDetail', params: { id: c.id } })"
        />
      </div>

      <ContactSpreadsheet
        v-else
        :contacts="store.contacts"
        @row-click="(id) => router.push({ name: 'ContactDetail', params: { id } })"
      />

      <Sheet v-model:open="drawerOpen">
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

      <ContactDeleteDialog
        :open="deleteDialogOpen"
        :contact-name="deletingContactName"
        @update:open="deleteDialogOpen = $event"
        @confirm="handleDelete"
      />

      <ContactsPagination
        :page="page"
        :total-pages="totalPages"
        :total="store.total"
        @prev="prevPage"
        @next="nextPage"
      />
    </div>
    <CsvImport :open="importOpen" @close="importOpen = false; loadContacts()" />
  </LayoutShell>
</template>
