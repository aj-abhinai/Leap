<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { type Lead } from '@/stores/leads'
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
import { Loader2, Link } from '@lucide/vue'
import type { Stage } from '@/stores/pipeline'

export interface PrefillContact {
  id: string
  name: string
  email?: string
  phone?: string
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

const linkedContactId = shallowRef(props.editingLead?.contact_id || props.prefillContact?.id || null)
const linkedContactName = shallowRef(props.editingLead?.contact_name || props.prefillContact?.name || '')

const formName = shallowRef(props.editingLead?.name || props.prefillContact?.name || '')
const formEmail = shallowRef(props.editingLead?.email || props.prefillContact?.email || '')
const formPhone = shallowRef(props.editingLead?.phone || props.prefillContact?.phone || '')
const formValue = shallowRef<number | undefined>(props.editingLead?.value)
const formNotes = shallowRef(props.editingLead?.notes || '')
const formStageId = shallowRef(props.editingLead?.stage_id || props.initialStageId || props.stages[0]?.id || '')
const formError = shallowRef('')
const saving = shallowRef(false)

const hasLinkedContact = computed(() => !!linkedContactId.value)

watch(() => props.editingLead, (lead) => {
  linkedContactId.value = lead?.contact_id || props.prefillContact?.id || null
  linkedContactName.value = lead?.contact_name || props.prefillContact?.name || ''
  formName.value = lead?.name || props.prefillContact?.name || ''
  formEmail.value = lead?.email || props.prefillContact?.email || ''
  formPhone.value = lead?.phone || props.prefillContact?.phone || ''
  formValue.value = lead?.value
  formNotes.value = lead?.notes || ''
  formStageId.value = lead?.stage_id || props.initialStageId || props.stages[0]?.id || ''
  formError.value = ''
})

async function handleSave() {
  formError.value = ''
  if (!formName.value) {
    formError.value = 'Name is required'
    return
  }
  saving.value = true
  emit('save', {
    name: formName.value,
    email: formEmail.value || null,
    phone: formPhone.value || null,
    value: formValue.value || null,
    notes: formNotes.value || null,
    pipeline_id: props.pipelineId,
    stage_id: formStageId.value,
    contact_id: linkedContactId.value || null,
  })
  saving.value = false
}
</script>

<template>
  <div class="mt-4 space-y-4">
    <div v-if="hasLinkedContact" class="flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
      <Link class="size-3.5 text-muted-foreground" />
      <span class="text-muted-foreground">Linked to</span>
      <Badge variant="secondary" class="text-xs">{{ linkedContactName }}</Badge>
    </div>
    <div class="space-y-2">
      <Label for="lname">Name *</Label>
      <Input id="lname" v-model="formName" placeholder="Lead name" />
    </div>
    <div class="space-y-2">
      <Label for="lemail">Email</Label>
      <Input id="lemail" v-model="formEmail" type="email" placeholder="Email" />
    </div>
    <div class="space-y-2">
      <Label for="lphone">Phone</Label>
      <Input id="lphone" v-model="formPhone" placeholder="Phone" />
    </div>
    <div class="space-y-2">
      <Label for="lvalue">Value</Label>
      <Input id="lvalue" v-model.number="formValue" type="number" placeholder="Deal value" />
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
        {{ editingLead ? 'Update' : 'Create' }}
      </Button>
      <Button
        v-if="editingLead"
        variant="destructive"
        @click="emit('delete', editingLead.id!)"
      >
        Delete
      </Button>
    </div>
  </div>
</template>
