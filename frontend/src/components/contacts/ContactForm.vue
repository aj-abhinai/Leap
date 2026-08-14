<script setup lang="ts">
import { onMounted, ref, shallowRef, watch } from 'vue'
import { type Contact, type PhoneValue, type EmailValue } from '@/stores/contacts'
import { useSettingsStore } from '@/stores/settings'
import { contactSchema } from '@/lib/validation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Loader2, Plus, X, Star } from '@lucide/vue'

const props = defineProps<{
  editingContact: Contact | null
  saving?: boolean
}>()

const emit = defineEmits<{
  save: [body: Record<string, any>]
}>()

const settings = useSettingsStore()

const formName = shallowRef(props.editingContact?.name || '')
const formNickname = shallowRef(props.editingContact?.nickname || '')
const formLocation = shallowRef(props.editingContact?.location || '')
const formAge = shallowRef<number | undefined>(props.editingContact?.age)
const formStatusId = shallowRef(props.editingContact?.status?.id || '__none__')
const selectedTags = ref<string[]>(props.editingContact?.tags?.map(t => t.id) || [])
const phones = ref<{ value: string; is_primary: boolean }[]>(
  (props.editingContact?.phones?.length ? props.editingContact.phones : props.editingContact?.phone ? [{ value: props.editingContact.phone, is_primary: true }] : [{ value: '', is_primary: true }])
)
const emails = ref<{ value: string; is_primary: boolean }[]>(
  (props.editingContact?.emails?.length ? props.editingContact.emails : props.editingContact?.email ? [{ value: props.editingContact.email, is_primary: true }] : [{ value: '', is_primary: true }])
)
const formError = shallowRef('')

onMounted(() => {
  settings.fetchTags()
})

watch(() => props.editingContact, (c) => {
  formName.value = c?.name || ''
  formNickname.value = c?.nickname || ''
  formLocation.value = c?.location || ''
  formAge.value = c?.age
  formStatusId.value = c?.status?.id || '__none__'
  selectedTags.value = c?.tags?.map(t => t.id) || []
  phones.value = c?.phones?.length
    ? c.phones.map(p => ({ value: p.value, is_primary: p.is_primary }))
    : c?.phone
      ? [{ value: c.phone, is_primary: true }]
      : [{ value: '', is_primary: true }]
  emails.value = c?.emails?.length
    ? c.emails.map(e => ({ value: e.value, is_primary: e.is_primary }))
    : c?.email
      ? [{ value: c.email, is_primary: true }]
      : [{ value: '', is_primary: true }]
  formError.value = ''
})

function addPhone() { phones.value.push({ value: '', is_primary: false }) }
function addEmail() { emails.value.push({ value: '', is_primary: false }) }

function removePhone(idx: number) {
  const removed = phones.value.splice(idx, 1)[0]
  // auto-promote the next value if the primary was removed
  if (removed?.is_primary && phones.value.length) {
    phones.value[0].is_primary = true
  }
}
function removeEmail(idx: number) {
  const removed = emails.value.splice(idx, 1)[0]
  if (removed?.is_primary && emails.value.length) {
    emails.value[0].is_primary = true
  }
}

function setPhonePrimary(idx: number) {
  phones.value.forEach((p, i) => (p.is_primary = i === idx))
}
function setEmailPrimary(idx: number) {
  emails.value.forEach((e, i) => (e.is_primary = i === idx))
}

function toggleTag(tagId: string) {
  const idx = selectedTags.value.indexOf(tagId)
  if (idx >= 0) {
    selectedTags.value.splice(idx, 1)
  } else {
    selectedTags.value.push(tagId)
  }
}

async function handleSave() {
  formError.value = ''
  const phoneVals = phones.value.map(p => p.value.trim()).filter(Boolean)
  const emailVals = emails.value.map(e => e.value.trim()).filter(Boolean)
  const result = contactSchema.safeParse({
    name: formName.value,
    nickname: formNickname.value || undefined,
    email: emailVals[0] ?? '',
    phone: phoneVals[0] ?? '',
    location: formLocation.value || undefined,
    age: formAge.value || undefined,
  })
  if (!result.success) {
    formError.value = result.error.issues[0]?.message || 'Validation failed'
    return
  }
  // ensure exactly one primary per type
  const phoneList = phones.value.map(p => ({ value: p.value.trim(), is_primary: p.is_primary })).filter(p => p.value)
  const emailList = emails.value.map(e => ({ value: e.value.trim(), is_primary: e.is_primary })).filter(e => e.value)
  if (phoneList.length && !phoneList.some(p => p.is_primary)) phoneList[0].is_primary = true
  if (emailList.length && !emailList.some(e => e.is_primary)) emailList[0].is_primary = true

  await emit('save', {
    name: result.data.name,
    nickname: formNickname.value,
    location: result.data.location ?? '',
    age: result.data.age ?? null,
    tag_ids: selectedTags.value,
    status_id: formStatusId.value && formStatusId.value !== '__none__' ? formStatusId.value : '',
    phones: phoneList,
    emails: emailList,
    phone: phoneList.find(p => p.is_primary)?.value ?? phoneList[0]?.value ?? '',
    email: emailList.find(e => e.is_primary)?.value ?? emailList[0]?.value ?? '',
  })
}
</script>

<template>
  <div class="flex flex-1 min-h-0 flex-col">
    <div class="flex-1 min-h-0 overflow-y-auto space-y-4 px-4 pb-4">
    <div class="space-y-2">
      <Label for="cname">Name *</Label>
      <Input id="cname" v-model="formName" placeholder="Full name" />
    </div>
    <div class="space-y-2">
      <Label for="cnick">Nickname</Label>
      <Input id="cnick" v-model="formNickname" placeholder="Nickname" />
    </div>
    <div class="space-y-2">
      <Label>Phones</Label>
      <div class="space-y-1.5">
        <div v-for="(p, idx) in phones" :key="idx" class="flex items-center gap-2">
          <Input v-model="p.value" type="tel" :placeholder="`Phone ${idx + 1}`" />
          <Button
            variant="ghost"
            size="icon-sm"
            :title="p.is_primary ? 'Primary phone' : 'Set as primary'"
            :aria-label="p.is_primary ? 'Primary phone' : 'Set as primary'"
            :class="p.is_primary ? 'text-primary' : 'text-muted-foreground'"
            @click="setPhonePrimary(idx)"
          >
            <Star class="size-3.5" :fill="p.is_primary ? 'currentColor' : 'none'" />
          </Button>
          <Button variant="ghost" size="icon-sm" title="Remove" aria-label="Remove phone" @click="removePhone(idx)">
            <X class="size-3.5" />
          </Button>
        </div>
        <Button variant="outline" size="sm" @click="addPhone">
          <Plus class="mr-1 size-3.5" /> Add phone
        </Button>
      </div>
    </div>
    <div class="space-y-2">
      <Label>Emails</Label>
      <div class="space-y-1.5">
        <div v-for="(e, idx) in emails" :key="idx" class="flex items-center gap-2">
          <Input v-model="e.value" type="email" :placeholder="`Email ${idx + 1}`" />
          <Button
            variant="ghost"
            size="icon-sm"
            :title="e.is_primary ? 'Primary email' : 'Set as primary'"
            :aria-label="e.is_primary ? 'Primary email' : 'Set as primary'"
            :class="e.is_primary ? 'text-primary' : 'text-muted-foreground'"
            @click="setEmailPrimary(idx)"
          >
            <Star class="size-3.5" :fill="e.is_primary ? 'currentColor' : 'none'" />
          </Button>
          <Button variant="ghost" size="icon-sm" title="Remove" aria-label="Remove email" @click="removeEmail(idx)">
            <X class="size-3.5" />
          </Button>
        </div>
        <Button variant="outline" size="sm" @click="addEmail">
          <Plus class="mr-1 size-3.5" /> Add email
        </Button>
      </div>
    </div>
    <div class="space-y-2">
      <Label for="clocation">Location</Label>
      <Input id="clocation" v-model="formLocation" placeholder="Location" />
    </div>
    <div class="space-y-2">
      <Label for="cage">Age</Label>
      <Input id="cage" v-model.number="formAge" type="number" placeholder="Age" />
    </div>
    <div class="space-y-2">
      <Label>Status</Label>
      <Select v-model="formStatusId">
        <SelectTrigger>
          <SelectValue placeholder="Select status" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__none__">None</SelectItem>
          <SelectItem v-for="s in settings.statuses" :key="s.id" :value="s.id">
            {{ s.name }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>
    <div class="space-y-2">
      <Label>Tags</Label>
      <div class="flex flex-wrap gap-2 max-h-32 overflow-y-auto rounded-md border p-3">
        <div v-for="t in settings.tags" :key="t.id" class="flex items-center gap-1.5">
          <Checkbox :id="'tag-' + t.id" :checked="selectedTags.includes(t.id)" @update:checked="toggleTag(t.id)" />
          <Label :for="'tag-' + t.id" class="text-sm cursor-pointer">{{ t.name }}</Label>
        </div>
        <p v-if="settings.tags.length === 0" class="text-sm text-muted-foreground">No tags available. Create them in Settings.</p>
      </div>
    </div>
    <div v-if="formError" class="text-sm text-destructive">{{ formError }}</div>
    </div>
    <div class="border-t p-4">
      <Button @click="handleSave" :disabled="saving" class="w-full">
        <Loader2 v-if="saving" class="mr-2 size-4 animate-spin" />
        {{ editingContact ? 'Update' : 'Create' }}
      </Button>
    </div>
  </div>
</template>
