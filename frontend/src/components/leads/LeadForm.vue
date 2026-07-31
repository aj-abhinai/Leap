<script setup lang="ts">
import { ref } from 'vue'
import { type Lead } from '@/stores/leads'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Loader2 } from '@lucide/vue'
import type { Stage } from '@/stores/pipeline'

const props = defineProps<{
  editingLead: Lead | null
  stages: Stage[]
  pipelineId: string
  initialStageId?: string
}>()

const emit = defineEmits<{
  save: [body: Record<string, any>]
  delete: [leadId: string]
}>()

const formName = ref(props.editingLead?.name || '')
const formEmail = ref(props.editingLead?.email || '')
const formPhone = ref(props.editingLead?.phone || '')
const formValue = ref<number | undefined>(props.editingLead?.value)
const formNotes = ref(props.editingLead?.notes || '')
const formStageId = ref(props.editingLead?.stage_id || props.initialStageId || props.stages[0]?.id || '')
const formError = ref('')
const saving = ref(false)

async function handleSave() {
  formError.value = ''
  if (!formName.value) {
    formError.value = 'Name is required'
    return
  }
  saving.value = true
  try {
    emit('save', {
      name: formName.value,
      email: formEmail.value || null,
      phone: formPhone.value || null,
      value: formValue.value || null,
      notes: formNotes.value || null,
      pipeline_id: props.pipelineId,
      stage_id: formStageId.value,
    })
  } catch (e: any) {
    formError.value = e.message || 'Save failed'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="mt-4 space-y-4">
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
