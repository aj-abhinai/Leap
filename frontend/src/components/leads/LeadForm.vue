<script setup lang="ts">
import { computed, onMounted, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { type Lead } from '@/stores/leads'
import { useSettingsStore } from '@/stores/settings'
import { useRBACStore } from '@/stores/rbac'
import { useUsersStore } from '@/stores/users'
import { useLeadDrawerGlobal } from '@/composables/useLeadDrawerGlobal'
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
import { formatCurrency, formatContactDetail } from '@/utils/format'
import { debounce } from '@/utils/debounce'
import { errorMessage } from '@/utils/errors'
import { listContacts, resolveContactByPhone, type ResolveMatch } from '@/api/contacts'
import { listPrograms, type Program } from '@/api/programs'
import { createLeadActivity, type OpenLeadRef } from '@/api/leads'

export interface PrefillContact {
  id: string
  name: string
  email?: string
  phone?: string
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
  saving?: boolean
  // openLeadConflict is the existing open lead from a refused create (409);
  // it renders the same resolve-or-log banner as the resolve picker path.
  openLeadConflict?: OpenLeadRef | null
}>()

export interface LeadSaveBody {
  nickname: string
  pipeline_id: string
  stage_id: string
  notes: string
  program_id: string
  lost_reason?: string
  assigned_to?: string
  contact_id?: string
  new_contact?: { name: string; phone: string; email: string }
}

const emit = defineEmits<{
  save: [body: LeadSaveBody]
  delete: [leadId: string]
  // enquiry-logged fires after the one-tap [Log enquiry] wrote the touchpoint
  // on the existing open lead; the drawer closes and the board refreshes.
  'enquiry-logged': []
}>()

const settings = useSettingsStore()
const users = useUsersStore()

const UNASSIGNED = '__unassigned__'
const programs = shallowRef<Program[]>([])
const formProgramId = shallowRef<string>(props.editingLead?.program_id || '__none__')
const linkedContactId = shallowRef(props.editingLead?.contact_id || props.prefillContact?.id || null)
const linkedContactName = shallowRef(props.editingLead?.contact_name || props.prefillContact?.name || '')
const formNickname = shallowRef(props.editingLead?.nickname || '')
const formNotes = shallowRef(props.editingLead?.notes || '')
const formLostReason = shallowRef(props.editingLead?.lost_reason || '')
const formAssignedTo = shallowRef(props.editingLead?.assigned_to || UNASSIGNED)
// defaultStageId picks the form's starting stage. In create mode a closing
// stage is unreachable (ErrClosingStageAtCreate), so the kanban "+" column's
// stage is only honored when it is open — otherwise the first open stage.
function defaultStageId(): string {
  if (props.editingLead?.stage_id) return props.editingLead.stage_id
  if (props.initialStageId) {
    const s = props.stages.find((x) => x.id === props.initialStageId)
    if (s && (props.editingLead || !s.is_closing)) return s.id
  }
  return props.stages.find((s) => props.editingLead || !s.is_closing)?.id || props.stages[0]?.id || ''
}

const formStageId = shallowRef(defaultStageId())
const formError = shallowRef('')

// Contact picker state
const contactSearch = shallowRef('')
const contactResults = shallowRef<ContactOption[]>([])
const searchingContacts = shallowRef(false)
// "new contact" inline branch
const newContactMode = shallowRef(false)
const newContactName = shallowRef('')
const newContactPhone = shallowRef('')
const newContactEmail = shallowRef('')
// phone resolve picker: matches are offered before the lead submits
const resolveMatches = shallowRef<ResolveMatch[]>([])
const resolvedOnce = shallowRef(false)

// Resolve-or-log banner: open leads already held by the picked contact. One
// lead → one-tap [Log enquiry]; several → the select identifies the match.
const bannerLeads = shallowRef<OpenLeadRef[]>([])
const selectedBannerLeadId = shallowRef('')
const loggingEnquiry = shallowRef(false)

const router = useRouter()
const rbac = useRBACStore()
const { openLeadDrawer } = useLeadDrawerGlobal()

const hasLinkedContact = computed(() => !!linkedContactId.value)
const isEditing = computed(() => !!props.editingLead)

// Closing stages are unreachable at create: the form hides them in
// create mode, and the backend rejects them (ErrClosingStageAtCreate).
const createStages = computed(() =>
  isEditing.value ? props.stages : props.stages.filter(s => !s.is_closing),
)

// A "+" click on a different column while the create drawer is open re-seeds
// the stage default (the form instance is reused for create mode).
watch(() => props.initialStageId, () => {
  if (!isEditing.value) formStageId.value = defaultStageId()
})

const selectedStage = computed(() => props.stages.find(s => s.id === formStageId.value))
const isClosingStage = computed(() => !!selectedStage.value?.is_closing)

const lossReasonPresets = computed(() => settings.lossReasons.map(t => t.name))

const selectedProgram = computed(() => programs.value.find(p => p.id === formProgramId.value))

// If the lead is assigned to a user who has since been deleted, that id is not
// in the options; sending it would fail validation (ErrInvalidAssignee). Reset
// to Unassigned once the option list loads so the form can still be saved.
// Only reset on a successful fetch: a transient fetch failure empties the
// options too, and must not silently unassign a valid assignee.
watch(
  () => users.options,
  () => {
    if (users.error) return
    if (formAssignedTo.value !== UNASSIGNED && !users.options.some((u) => u.id === formAssignedTo.value)) {
      formAssignedTo.value = UNASSIGNED
    }
  },
)

// snapshotValue returns the lead's saved value while the program is unchanged,
// else the newly selected program's price — the price snapshot contract.
const snapshotValue = computed(() => {
  if (props.editingLead?.program_id === formProgramId.value) {
    return props.editingLead.value
  }
  return selectedProgram.value?.price
})

function formatPrice(price?: number) {
  return formatCurrency(price) || '–'
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

// showBanner renders the resolve-or-log banner for the given open leads and
// defaults the match select to the first (oldest) lead.
function showBanner(leads: OpenLeadRef[]) {
  if (!leads.length) return
  bannerLeads.value = leads
  selectedBannerLeadId.value = leads[0].id
}

function dismissBanner() {
  bannerLeads.value = []
  selectedBannerLeadId.value = ''
}

// A create refused with an open-lead conflict (409) renders the same
// resolve-or-log banner as the resolve picker path — the backstop when the
// form picked a contact without open-lead data. The banner clears when the
// conflict is reset (drawer close, next create).
watch(
  () => props.openLeadConflict,
  (lead) => {
    if (lead) {
      showBanner([lead])
    } else {
      dismissBanner()
    }
  },
  { immediate: true },
)

// The select identifies the match when the contact holds several open leads;
// the single-lead case falls back to the only lead.
const selectedBannerLead = computed(() =>
  bannerLeads.value.find((l) => l.id === selectedBannerLeadId.value) ?? bannerLeads.value[0],
)

// logEnquiry writes exactly one done Enquiry activity on the existing open
// lead — the repeat enquiry is a touchpoint, not a new opportunity. The
// server stamps occurred_at for done activities. The type is resolved from
// the task-type catalog (a seeded preset) so a rename keeps working, with the
// literal name as the fallback.
async function logEnquiry() {
  const lead = selectedBannerLead.value
  if (!lead) return
  loggingEnquiry.value = true
  try {
    let enquiryType = 'Enquiry'
    try {
      if (settings.activityTypes.length === 0) await settings.fetchTags()
      // Prefer the catalog's entry when the workspace customizes task types;
      // an absent entry falls back to the seeded literal.
      enquiryType = settings.activityTypes.find((t) => t.name === 'Enquiry')?.name ?? 'Enquiry'
    } catch {
      // The catalog is a convenience; the literal preset still logs the enquiry.
    }
    await createLeadActivity(lead.id, {
      type: enquiryType,
      is_done: true,
    })
    toast.success('Enquiry logged')
    emit('enquiry-logged')
    dismissBanner()
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to log enquiry'))
  } finally {
    loggingEnquiry.value = false
  }
}

// viewBannerLead opens the existing lead's drawer over the current page.
function viewBannerLead() {
  const lead = selectedBannerLead.value
  if (!lead) return
  openLeadDrawer(lead.id)
}

// searchContacts debounces the contact picker search (300ms) and discards
// out-of-order responses so a slow reply can never overwrite a newer query.
let searchSeq = 0
const searchContacts = debounce(runContactSearch, 300)

async function runContactSearch() {
  const q = contactSearch.value.trim()
  if (!q) {
    contactResults.value = []
    return
  }
  const seq = ++searchSeq
  searchingContacts.value = true
  try {
    const res = await listContacts({ page: 1, perPage: 10, q })
    if (seq !== searchSeq) return
    contactResults.value = res.data
  } catch {
    if (seq !== searchSeq) return
    contactResults.value = []
  } finally {
    if (seq === searchSeq) searchingContacts.value = false
  }
}

function selectContact(c: ContactOption) {
  linkedContactId.value = c.id
  linkedContactName.value = c.name
  newContactMode.value = false
  contactSearch.value = ''
  contactResults.value = []
  // The search list does not carry open leads; a collision surfaces as the
  // create-409 backstop instead.
  dismissBanner()
}

function chooseNewContact() {
  newContactMode.value = true
  linkedContactId.value = null
  linkedContactName.value = ''
  contactSearch.value = ''
  contactResults.value = []
  dismissBanner()
}

function chooseExisting() {
  newContactMode.value = false
}

onMounted(async () => {
  if (settings.lossReasons.length === 0) settings.fetchTags()
  users.fetchOptions()
  try {
    const res = await listPrograms()
    programs.value = res.data
  } catch {}
})

async function handleSave() {
  formError.value = ''
  const body: LeadSaveBody = {
    nickname: formNickname.value,
    pipeline_id: props.pipelineId,
    stage_id: formStageId.value,
    notes: formNotes.value,
    program_id: programIdToSend(),
    assigned_to: formAssignedTo.value === UNASSIGNED ? '' : formAssignedTo.value,
  }
  if (isClosingStage.value) {
    body.lost_reason = formLostReason.value.trim()
  }

  if (isEditing.value) {
    // Editing keeps the existing linked contact; no resolve-or-create here.
    if (linkedContactId.value) {
      body.contact_id = linkedContactId.value
    }
  } else if (newContactMode.value) {
    // Resolve-or-create: ask on phone match, then submit.
    if (!newContactName.value.trim()) {
      formError.value = 'Contact name is required'
      return
    }
    if (!newContactPhone.value.trim() && !newContactEmail.value.trim()) {
      formError.value = 'A phone or email is required for a new contact'
      return
    }
    if (!resolvedOnce.value && newContactPhone.value.trim()) {
      try {
        const res = await resolveContactByPhone(newContactPhone.value.trim())
        const matches = res.data ?? []
        if (matches.length > 0) {
          resolveMatches.value = matches
          resolvedOnce.value = true
          return // show the picker; do not submit yet
        }
      } catch {
        // Resolve is best-effort: fall through to the new-contact submission.
      }
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

  emit('save', body)
}

function linkResolvedMatch(m: ResolveMatch) {
  linkedContactId.value = m.id
  linkedContactName.value = m.name
  newContactMode.value = false
  resolveMatches.value = []
  resolvedOnce.value = false
  // The resolve response carries the contact's open leads, so the banner
  // renders immediately without a second fetch.
  if (m.open_leads?.length) {
    showBanner(m.open_leads)
  } else {
    dismissBanner()
  }
}

function createNewPersonInstead() {
  resolveMatches.value = []
  resolvedOnce.value = true
  router.push('/contacts')
}
</script>

<template>
  <div class="flex flex-1 min-h-0 flex-col">
    <div class="flex-1 min-h-0 overflow-y-auto space-y-4 px-4 pb-4">
    <!-- Linked contact banner -->
    <div v-if="hasLinkedContact" class="flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
      <Link class="size-3.5 text-muted-foreground" />
      <span class="text-muted-foreground">Linked to</span>
      <Badge variant="secondary" class="text-xs">{{ linkedContactName }}</Badge>
    </div>

    <!-- Resolve-or-log banner: the contact already holds an open lead in
         this slot; a repeat enquiry is a touchpoint on the existing deal,
         not a new opportunity. -->
    <div v-if="bannerLeads.length" class="space-y-2 rounded-md border px-3 py-2 text-sm">
      <div class="flex items-center justify-between gap-2">
        <p class="font-medium">
          {{
            bannerLeads.length > 1
              ? 'Open deals exist for this contact'
              : `${selectedBannerLead?.display_name || 'This contact'} has an open deal`
          }}
        </p>
        <button
          type="button"
          class="text-xs text-muted-foreground hover:text-foreground"
          @click="dismissBanner"
        >
          Dismiss
        </button>
      </div>
      <Select v-if="bannerLeads.length > 1" v-model="selectedBannerLeadId">
        <SelectTrigger>
          <SelectValue placeholder="Select deal" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="l in bannerLeads" :key="l.id" :value="l.id">
            {{ l.display_name }} · {{ l.program_name || 'No program' }}
          </SelectItem>
        </SelectContent>
      </Select>
      <p v-if="selectedBannerLead" class="text-xs text-muted-foreground">
        {{
          [selectedBannerLead.stage_name, selectedBannerLead.program_name || 'No program', selectedBannerLead.pipeline_name]
            .filter(Boolean)
            .join(' · ')
        }}
      </p>
      <p class="text-xs text-muted-foreground">
        Log this enquiry on the open deal instead of creating a duplicate.
      </p>
      <div class="flex gap-2">
        <Button size="sm" :disabled="loggingEnquiry" @click="logEnquiry">
          <Loader2 v-if="loggingEnquiry" class="mr-2 size-3.5 animate-spin" />
          Log enquiry
        </Button>
        <Button size="sm" variant="outline" @click="viewBannerLead">View lead</Button>
      </div>
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
              aria-label="Contact"
              placeholder="Search by name, phone or email"
              class="pl-8"
              :disabled="!rbac.can('contact:read')"
              @input="searchContacts"
            />
          </div>
          <Button variant="outline" @click="chooseNewContact">New contact</Button>
        </div>
        <p v-if="!rbac.can('contact:read')" class="text-xs text-muted-foreground">
          contact:read permission required to search contacts
        </p>
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
              {{ formatContactDetail(c.phone, c.email) }}
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
          <Input id="nc-phone" v-model="newContactPhone" placeholder="98765 43210 or +971 50 123 4567" />
        </div>
        <div class="space-y-2">
          <Label for="nc-email">Email</Label>
          <Input id="nc-email" v-model="newContactEmail" type="email" placeholder="Email" />
        </div>
        <p class="text-xs text-muted-foreground">
          A phone or email is required. If the phone matches an existing contact, the lead links to it.
        </p>

        <!-- Phone-match picker: ask before linking -->
        <div v-if="resolveMatches.length" class="space-y-2">
          <Label class="text-sm font-medium">Matching contacts</Label>
          <div class="max-h-48 overflow-y-auto rounded-md border">
            <button
              v-for="m in resolveMatches"
              :key="m.id"
              class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left hover:bg-muted/50"
              @click="linkResolvedMatch(m)"
            >
              <span class="min-w-0">
                <span class="block truncate text-sm font-medium">{{ m.name }}</span>
                <span class="block truncate text-xs text-muted-foreground">
                  {{ formatContactDetail(m.phone, m.email) }}
                </span>
              </span>
              <span class="shrink-0 text-xs text-primary">Link to {{ m.name }}</span>
            </button>
          </div>
          <Button variant="outline" size="sm" class="w-full" @click="createNewPersonInstead">
            Create new person on Contacts page
          </Button>
        </div>
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
        <SelectTrigger id="lprogram">
          <SelectValue placeholder="Select program" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__none__">No program</SelectItem>
          <SelectItem v-for="p in programs" :key="p.id" :value="p.id">
            {{ p.name }} · {{ formatPrice(p.price) }}
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
      <Label>Assignee</Label>
      <Select v-model="formAssignedTo">
        <SelectTrigger aria-label="Assignee">
          <SelectValue placeholder="Unassigned" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem :value="UNASSIGNED">Unassigned</SelectItem>
          <SelectItem v-for="u in users.options" :key="u.id" :value="u.id">
            {{ u.name }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>
    <div class="space-y-2">
      <Label>Stage</Label>
      <Select v-model="formStageId">
        <SelectTrigger aria-label="Stage">
          <SelectValue placeholder="Select stage" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="s in createStages" :key="s.id" :value="s.id">
            {{ s.name }}
          </SelectItem>
        </SelectContent>
      </Select>
      <p v-if="!isEditing && createStages.length < stages.length" class="text-xs text-muted-foreground">
        Closing stages are only reachable by moving an existing lead
      </p>
    </div>
    <div v-if="isClosingStage" class="space-y-2">
      <Label>Loss reason</Label>
      <div class="flex flex-wrap gap-1.5">
        <Button
          v-for="r in lossReasonPresets"
          :key="r"
          variant="outline"
          size="sm"
          :class="{ 'ring-1 ring-primary': formLostReason === r }"
          @click="formLostReason = formLostReason === r ? '' : r"
        >
          {{ r }}
        </Button>
      </div>
      <Input v-model="formLostReason" aria-label="Loss reason" placeholder="Or type a reason…" />
    </div>
    <div v-if="formError" class="text-sm text-destructive">{{ formError }}</div>
    </div>
    <div class="border-t p-4">
      <div class="flex gap-2">
        <Button @click="handleSave" :disabled="saving" class="flex-1">
          <Loader2 v-if="saving" class="mr-2 size-4 animate-spin" />
          {{ saving ? 'Saving...' : (isEditing ? 'Update' : 'Create') }}
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
  </div>
</template>
