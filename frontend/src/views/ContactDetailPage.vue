<script setup lang="ts">
import { ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useContactsStore, type Contact } from '@/stores/contacts'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import ContactForm, { type ContactSaveBody } from '@/components/contacts/ContactForm.vue'
import ContactLeadJourney from '@/components/contacts/ContactLeadJourney.vue'
import ContactNotes from '@/components/contacts/ContactNotes.vue'
import { Mail, Phone, MapPin, ArrowLeft, Pencil } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { getAvatarColor, getInitials } from '@/utils/avatar'
import { errorMessage } from '@/utils/errors'

const route = useRoute()
const router = useRouter()
const store = useContactsStore()

const contact = ref<Contact | null>(null)
const loading = shallowRef(true)
const loadError = shallowRef('')
const drawerOpen = shallowRef(false)
const saving = shallowRef(false)

// Watch the route param: navigating detail-to-detail reuses this instance,
// so onMounted-only loading would leave the previous contact on screen.
let loadSeq = 0
watch(
  () => route.params.id as string,
  (id) => loadContact(id),
  { immediate: true },
)

// loadContact fetches the contact for id; a newer request supersedes this
// one, so out-of-order responses are discarded.
async function loadContact(id: string) {
  const seq = ++loadSeq
  loading.value = true
  loadError.value = ''
  try {
    const loaded = await store.fetchContact(id)
    if (seq !== loadSeq) return
    contact.value = loaded
  } catch (e) {
    if (seq !== loadSeq) return
    contact.value = null
    if (errorMessage(e) === 'Contact not found') return
    loadError.value = errorMessage(e, 'Failed to load contact')
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

function goBack() {
  router.push({ name: 'Contacts' })
}

async function handleSave(body: ContactSaveBody) {
  const saved = contact.value
  if (!saved) return
  saving.value = true
  // Capture the id and the seq before awaiting the patch: navigating to
  // another contact mid-save bumps loadSeq, and this refetch of the saved
  // contact must then be discarded instead of overwriting the new contact.
  const seq = loadSeq
  try {
    await store.update(saved.id, body)
    toast.success('Contact updated')
    drawerOpen.value = false
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to update contact'))
    return
  } finally {
    saving.value = false
  }
  // The refetch is gated on loadSeq like loadContact so a route change that
  // starts a new load cannot be overwritten by this stale refresh.
  try {
    const loaded = await store.fetchContact(saved.id)
    if (seq === loadSeq) contact.value = loaded
  } catch {
    // Update succeeded; keep showing the previous data rather than blanking the page.
  }
}
</script>

<template>
  <div v-if="loading" class="p-6 space-y-4">
    <Skeleton class="h-6 w-32" />
    <div class="flex gap-6">
      <div class="w-1/3 space-y-3">
        <Skeleton class="h-8 w-3/4" />
        <Skeleton class="h-4 w-full" />
        <Skeleton class="h-4 w-2/3" />
      </div>
      <div class="w-2/3 space-y-3">
        <Skeleton class="h-6 w-32" />
        <Skeleton class="h-16 w-full" />
      </div>
    </div>
  </div>

  <div v-else-if="loadError" class="flex flex-col items-center justify-center py-16">
    <p class="text-destructive">{{ loadError }}</p>
    <Button variant="outline" class="mt-3" @click="loadContact(route.params.id as string)">Retry</Button>
    <Button variant="ghost" class="mt-2" @click="goBack">Back to Contacts</Button>
  </div>

  <div v-else-if="!contact" class="flex flex-col items-center justify-center py-16">
    <p class="text-muted-foreground">Contact not found</p>
    <Button variant="outline" class="mt-3" @click="goBack">Back to Contacts</Button>
  </div>

  <div v-else class="flex flex-col gap-6 p-6">
    <Button variant="ghost" class="w-fit -ml-2" @click="goBack">
      <ArrowLeft class="mr-2 size-4" /> Contacts
    </Button>

    <div class="flex flex-col gap-6 md:flex-row">
      <div class="w-full md:w-1/3 md:space-y-4">
        <div class="flex items-start gap-4">
          <div
            class="flex size-14 shrink-0 items-center justify-center rounded-full text-lg font-medium"
            :class="getAvatarColor(contact.name)"
          >
            {{ getInitials(contact.name) }}
          </div>
          <div>
            <h1 class="text-2xl font-semibold tracking-tight wrap-break-word">{{ contact.name }}</h1>
            <Badge v-if="contact.status" variant="secondary" class="mt-1.5">{{ contact.status.name }}</Badge>
          </div>
        </div>

        <div class="space-y-2">
          <div v-if="contact.nickname" class="flex items-center gap-2 text-sm">
            <Badge variant="secondary" class="text-xs">Nickname: {{ contact.nickname }}</Badge>
          </div>
          <div v-if="contact.emails?.length" class="flex items-start gap-2 text-sm text-muted-foreground">
            <Mail class="mt-0.5 size-4 shrink-0" />
            <div class="space-y-1">
              <div v-for="e in contact.emails" :key="e.id" class="flex items-center gap-2">
                <span>{{ e.value }}</span>
                <Badge v-if="e.is_primary" variant="outline" class="text-[10px] px-1.5">primary</Badge>
              </div>
            </div>
          </div>
          <div v-else-if="contact.email" class="flex items-center gap-2 text-sm text-muted-foreground">
            <Mail class="size-4" /> {{ contact.email }}
          </div>
          <div v-if="contact.phones?.length" class="flex items-start gap-2 text-sm text-muted-foreground">
            <Phone class="mt-0.5 size-4 shrink-0" />
            <div class="space-y-1">
              <div v-for="p in contact.phones" :key="p.id" class="flex items-center gap-2">
                <span>{{ p.value }}</span>
                <Badge v-if="p.is_primary" variant="outline" class="text-[10px] px-1.5">primary</Badge>
              </div>
            </div>
          </div>
          <div v-else-if="contact.phone" class="flex items-center gap-2 text-sm text-muted-foreground">
            <Phone class="size-4" /> {{ contact.phone }}
          </div>
          <div v-if="contact.location" class="flex items-center gap-2 text-sm text-muted-foreground">
            <MapPin class="size-4" /> {{ contact.location }}
          </div>
          <div v-if="contact.age" class="text-sm text-muted-foreground">
            Age: {{ contact.age }}
          </div>
        </div>

        <div v-if="contact.tags?.length" class="flex flex-wrap gap-1.5">
          <Badge v-for="t in contact.tags" :key="t.id" variant="outline">{{ t.name }}</Badge>
        </div>

        <Sheet v-model:open="drawerOpen">
          <SheetContent>
            <SheetHeader>
              <SheetTitle>Edit Contact</SheetTitle>
              <SheetDescription>Update contact details below.</SheetDescription>
            </SheetHeader>
            <ContactForm
              :key="contact.id"
              :editing-contact="contact"
              :saving="saving"
              @save="handleSave"
            />
          </SheetContent>
        </Sheet>

        <Button variant="outline" class="w-full" @click="drawerOpen = true">
          <Pencil class="mr-2 size-4" /> Edit
        </Button>
      </div>

      <div class="w-full md:w-2/3">
        <ContactLeadJourney :contact-id="contact.id" />
        <ContactNotes :contact-id="contact.id" class="mt-6" />
      </div>
    </div>
  </div>
</template>
