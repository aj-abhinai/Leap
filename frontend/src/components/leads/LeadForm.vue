<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { type Lead } from '@/stores/leads'
import { apiClient } from '@/composables/useApi'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Loader2, Link, Search } from '@lucide/vue'
import type { Stage } from '@/stores/pipeline'
import { formatCurrency } from '@/utils/format'

export interface PrefillContact {
  id: string
  name: string
  email?: string
  phone?: string
}

interface Program {
  id: string
  name: string
  price: number
}

interface ContactOption {
  id: string
  name: string
  phone?: string
  email?: string
}

const props = defineProps<{
  editingLead: Lead | null
  stages: Stage[]
  pipelineId: string
  initialStageId?: string
  prefillContact?: PrefillContact | null
}>()

const emit = defineEmits<{
  save: [body: Record<string, any>]
  delete: [leadId: string]
}>()

const programs = shallowRef<Program[]>([])
const formProgramId = shallowRef<string>(props.editingLead?.program_id || '__none__')
const linkedContactId = shallowRef(props.editingLead?.contact_id || props.prefillContact?.id || null)
const linkedContactName = shallowRef(props.editingLead?.contact_name || props.prefillContact?.name || '')
const formNickname = shallowRef(props.editingLead?.nickname || '')
const formNotes = shallowRef(props.editingLead?.notes || '')
const formStageId = shallowRef(props.editingLead?.stage_id || props.initialStageId || props.stages[0]?.id || '')
const formError = shallowRef('')
const saving = shallowRef(false)

// Contact picker state
const contactSearch = shallowRef('')
const contactResults = shallowRef<ContactOption[]>([])
const searchingContacts = shallowRef(false)
// "new contact" inline branch
const newContactMode = shallowRef(false)
const newContactName = shallowRef('')
const newContactPhone = shallowRef('')
const newContactEmail = shallowRef('')

const hasLinkedContact = computed(() => !!linkedContactId.value)
const isEditing = computed(() => !!props.editingLead)

const selectedProgram = computed(() => programs.value.find(p => p.id === formProgramId.value))

const snapshotValue = computed(() => {
  if (props.editingLead?.program_id === formProgramId.value) {
    return props.editingLead.value
  }
  return selectedProgram.value?.price
})

function formatPrice(price?: number) {
  return formatCurrency(price) || '—'
}

function programIdToSend(): string {
  return formProgramId.value === '__none__' ? '' : formProgramId.value
}

function displayName(): string {
  if (formNickname.value) return formNickname.value
  if (hasLinkedContact.value) return linkedContactName.value
  if (newContactMode.value) return newContactName.value
  return ''
}

async function searchContacts() {
  const q = contactSearch.value.trim()
  if (!q) {
    contactResults.value = []
    return
  }
  searchingContacts.value = true
  try {
    const res = await apiClient.get(`/api/contacts?q=${encodeURIComponent(q)}&per_page=10`)
    contactResults.value = res.data
  } catch {
    contactResults.value = []
  } finally {
    searchingContacts.value = false
  }
}

function selectContact(c: ContactOption) {
  linkedContactId.value = c.id
  linkedContactName.value = c.name
  newContactMode.value = false
  contactSearch.value = ''
  contactResults.value = []
}

function chooseNewContact() {
  newContactMode.value = true
  linkedContactId.value = null
  linkedContactName.value = ''
  contactSearch.value = ''
  contactResults.value = []
}

function chooseExisting() {
  newContactMode.value = false
}

onMounted(async () => {
  try {
    const res = await apiClient.get('/api/programs')
    programs.value = res.data
  } catch {}
})

async function handleSave() {
  formError.value = ''
  const body: Record<string, any> = {
    nickname: formNickname.value,
    pipeline_id: props.pipelineId,
    stage_id: formStageId.value,
    notes: formNotes.value,
    program_id: programIdToSend(),
  }

  if (isEditing.value) {
    // Editing keeps the existing linked contact; no resolve-or-create here.
    if (linkedContactId.value) {
      body.contact_id = linkedContactId.value
    }
  } else if (newContactMode.value) {
    // Resolve-or-create: lead service finds or creates the contact.
    if (!newContactName.value.trim()) {
      formError.value = 'Contact name is required'
      return
    }
    if (!newContactPhone.value.trim() && !newContactEmail.value.trim()) {
      formError.value = 'A phone or email is required for a new contact'
      return
    }
    body.new_contact = {
      name: newContactName.value.trim(),
      phone: newContactPhone.value.trim(),
      email: newContactEmail.value.trim(),
    }
  } else {
    if (!linkedContactId.value) {
      formError.value = 'Pick an existing contact or create a new one'
      return
    }
    body.contact_id = linkedContactId.value
  }

  saving.value = true
  try {
    await emit('save', body)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="mt-4 space-y-4">
    <!-- Linked contact banner -->
    <div v-if="hasLinkedContact" class="flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
      <Link class="size-3.5 text-muted-foreground" />
      <span class="text-muted-foreground">Linked to</span>
      <Badge variant="secondary" class="text-xs">{{ linkedContactName }}</Badge>
    </div>

    <!-- Contact selection (create only) -->
    <template v-if="!isEditing">
      <div v-if="!newContactMode && !hasLinkedContact" class="space-y-2">
        <Label>Contact</Label>
        <div class="flex gap-2">
          <div class="relative flex-1">
            <Search class="absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              v-model="contactSearch"
              placeholder="Search by name, phone or email"
              class="pl-8"
              @input="searchContacts"
            />
          </div>
          <Button variant="outline" @click="chooseNewContact">New contact</Button>
        </div>
        <div v-if="searchingContacts" class="text-sm text-muted-foreground">Searching…</div>
        <div v-else-if="contactResults.length" class="max-h-56 overflow-y-auto rounded-md border">
          <button
            v-for="c in contactResults"
            :key="c.id"
            class="flex w-full flex-col items-start gap-0.5 px-3 py-2 text-left hover:bg-muted/50"
            @click="selectContact(c)"
          >
            <span class="text-sm font-medium">{{ c.name }}</span>
            <span class="text-xs text-muted-foreground">
              {{ [c.phone, c.email].filter(Boolean).join(' · ') }}
            </span>
          </button>
        </div>
        <div v-else-if="contactSearch" class="text-sm text-muted-foreground">No contacts found</div>
      </div>

      <!-- New contact inline branch -->
      <div v-else-if="newContactMode && !hasLinkedContact" class="space-y-3 rounded-md border p-3">
        <div class="flex items-center justify-between">
          <Label class="text-sm font-medium">New contact</Label>
          <Button variant="ghost" size="sm" @click="chooseExisting">Pick existing</Button>
        </div>
        <div class="space-y-2">
          <Label for="nc-name">Name *</Label>
          <Input id="nc-name" v-model="newContactName" placeholder="Contact name" />
        </div>
        <div class="space-y-2">
          <Label for="nc-phone">Phone</Label>
          <Input id="nc-phone" v-model="newContactPhone" placeholder="Phone" />
        </div>
        <div class="space-y-2">
          <Label for="nc-email">Email</Label>
          <Input id="nc-email" v-model="newContactEmail" type="email" placeholder="Email" />
        </div>
        <p class="text-xs text-muted-foreground">
          A phone or email is required. If the phone matches an existing contact, the lead links to it.
        </p>
      </div>
    </template>

    <div class="space-y-2">
      <Label for="lnick">Nickname (optional)</Label>
      <Input id="lnick" v-model="formNickname" :placeholder="displayName() || 'Deal nickname'" />
      <p v-if="hasLinkedContact && !formNickname" class="text-xs text-muted-foreground">
        Displaying contact name: {{ linkedContactName }}
      </p>
    </div>
    <div class="space-y-2">
      <Label for="lprogram">Program</Label>
      <Select v-model="formProgramId">
        <SelectTrigger>
          <SelectValue placeholder="Select program" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__none__">No program</SelectItem>
          <SelectItem v-for="p in programs" :key="p.id" :value="p.id">
            {{ p.name }} — {{ formatPrice(p.price) }}
          </SelectItem>
        </SelectContent>
      </Select>
      <p class="text-xs text-muted-foreground">
        Value snapshot: {{ formatPrice(snapshotValue) }}
      </p>
    </div>
    <div class="space-y-2">
      <Label for="lnotes">Notes</Label>
      <Textarea id="lnotes" v-model="formNotes" placeholder="Notes..." rows="3" />
    </div>
    <div class="space-y-2">
      <Label>Stage</Label>
      <Select v-model="formStageId">
        <SelectTrigger>
          <SelectValue placeholder="Select stage" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="s in stages" :key="s.id" :value="s.id">
            {{ s.name }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>
    <div v-if="formError" class="text-sm text-destructive">{{ formError }}</div>
    <div class="flex gap-2">
      <Button @click="handleSave" :disabled="saving" class="flex-1">
        <Loader2 v-if="saving" class="mr-2 size-4 animate-spin" />
        {{ isEditing ? 'Update' : 'Create' }}
      </Button>
      <Button
        v-if="isEditing"
        variant="destructive"
        @click="editingLead && emit('delete', editingLead.id)"
      >
        Delete
      </Button>
    </div>
  </div>
</template>
