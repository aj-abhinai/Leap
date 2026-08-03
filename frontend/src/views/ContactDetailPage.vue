<script setup lang="ts">
import { ref, onMounted, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useContactsStore, type Contact } from '@/stores/contacts'
import LayoutShell from '@/components/layout/LayoutShell.vue'
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
import ContactForm from '@/components/contacts/ContactForm.vue'
import ContactLeadJourney from '@/components/contacts/ContactLeadJourney.vue'
import ContactNotes from '@/components/contacts/ContactNotes.vue'
import { Mail, Phone, MapPin, ArrowLeft, Pencil } from '@lucide/vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import { getAvatarColor, getInitials } from '@/utils/avatar'

const route = useRoute()
const router = useRouter()
const store = useContactsStore()

const contact = ref<Contact | null>(null)
const loading = shallowRef(true)
const drawerOpen = shallowRef(false)

onMounted(async () => {
  const id = route.params.id as string
  contact.value = await store.fetchContact(id)
  loading.value = false
})

function goBack() {
  router.push({ name: 'Contacts' })
}

async function handleSave(body: Record<string, any>) {
  if (!contact.value) return
  try {
    await apiClient.patch(`/api/contacts/${contact.value.id}`, body)
    toast.success('Contact updated')
    drawerOpen.value = false
    contact.value = await store.fetchContact(contact.value.id)
  } catch (e: any) {
    toast.error(e.message || 'Failed to update contact')
  }
}
</script>

<template>
  <LayoutShell>
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

    <div v-else-if="!contact" class="flex flex-col items-center justify-center py-16">
      <p class="text-muted-foreground">Contact not found</p>
      <Button variant="outline" class="mt-3" @click="goBack">Back to Contacts</Button>
    </div>

    <div v-else class="flex flex-col gap-6 p-6">
      <Button variant="ghost" class="w-fit -ml-2" @click="goBack">
        <ArrowLeft class="mr-2 size-4" /> Contacts
      </Button>

      <div class="flex gap-6">
        <div class="w-1/3 space-y-4">
          <div class="flex items-start gap-4">
            <div
              class="flex size-14 shrink-0 items-center justify-center rounded-full text-lg font-medium"
              :class="getAvatarColor(contact.name)"
            >
              {{ getInitials(contact.name) }}
            </div>
            <div>
              <h2 class="text-xl font-bold">{{ contact.name }}</h2>
              <Badge v-if="contact.status" variant="secondary" class="mt-1">{{ contact.status.name }}</Badge>
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
                @save="handleSave"
              />
            </SheetContent>
          </Sheet>

          <Button variant="outline" class="w-full" @click="drawerOpen = true">
            <Pencil class="mr-2 size-4" /> Edit
          </Button>
        </div>

        <div class="w-2/3">
          <ContactLeadJourney :contact-id="contact.id" />
          <ContactNotes :contact-id="contact.id" class="mt-6" />
        </div>
      </div>
    </div>
  </LayoutShell>
</template>
