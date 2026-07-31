<script setup lang="ts">
import { ref } from 'vue'
import { type Contact } from '@/stores/contacts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Loader2 } from '@lucide/vue'

const props = defineProps<{
  editingContact: Contact | null
}>()

const emit = defineEmits<{
  save: [body: Record<string, any>]
}>()

const formName = ref(props.editingContact?.name || '')
const formEmail = ref(props.editingContact?.email || '')
const formPhone = ref(props.editingContact?.phone || '')
const formLocation = ref(props.editingContact?.location || '')
const formAge = ref<number | undefined>(props.editingContact?.age)
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
      location: formLocation.value || null,
      age: formAge.value || null,
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
      <Label for="cname">Name *</Label>
      <Input id="cname" v-model="formName" placeholder="Full name" />
    </div>
    <div class="space-y-2">
      <Label for="cemail">Email</Label>
      <Input id="cemail" v-model="formEmail" type="email" placeholder="Email address" />
    </div>
    <div class="space-y-2">
      <Label for="cphone">Phone</Label>
      <Input id="cphone" v-model="formPhone" placeholder="Phone number" />
    </div>
    <div class="space-y-2">
      <Label for="clocation">Location</Label>
      <Input id="clocation" v-model="formLocation" placeholder="Location" />
    </div>
    <div class="space-y-2">
      <Label for="cage">Age</Label>
      <Input id="cage" v-model.number="formAge" type="number" placeholder="Age" />
    </div>
    <div v-if="formError" class="text-sm text-destructive">{{ formError }}</div>
    <Button @click="handleSave" :disabled="saving" class="w-full">
      <Loader2 v-if="saving" class="mr-2 size-4 animate-spin" />
      {{ editingContact ? 'Update' : 'Create' }}
    </Button>
  </div>
</template>
