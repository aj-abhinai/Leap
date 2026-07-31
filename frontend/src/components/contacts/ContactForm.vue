<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { type Contact } from '@/stores/contacts'
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
import { Loader2 } from '@lucide/vue'

const props = defineProps<{
  editingContact: Contact | null
}>()

const emit = defineEmits<{
  save: [body: Record<string, any>]
}>()

const settings = useSettingsStore()

const formName = ref(props.editingContact?.name || '')
const formEmail = ref(props.editingContact?.email || '')
const formPhone = ref(props.editingContact?.phone || '')
const formLocation = ref(props.editingContact?.location || '')
const formAge = ref<number | undefined>(props.editingContact?.age)
const formStatusId = ref(props.editingContact?.status?.id || '__none__')
const selectedTags = ref<string[]>(props.editingContact?.tags?.map(t => t.id) || [])
const formError = ref('')
const saving = ref(false)

onMounted(() => {
  settings.fetchTags()
})

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
  const result = contactSchema.safeParse({
    name: formName.value,
    email: formEmail.value || undefined,
    phone: formPhone.value || undefined,
    location: formLocation.value || undefined,
    age: formAge.value || undefined,
  })
  if (!result.success) {
    formError.value = result.error.errors[0]?.message || 'Validation failed'
    return
  }
  saving.value = true
  try {
    emit('save', {
      name: result.data.name,
      email: result.data.email || null,
      phone: result.data.phone || null,
      location: result.data.location || null,
      age: result.data.age || null,
      tag_ids: selectedTags.value,
      status_id: formStatusId.value && formStatusId.value !== '__none__' ? formStatusId.value : null,
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
    <Button @click="handleSave" :disabled="saving" class="w-full">
      <Loader2 v-if="saving" class="mr-2 size-4 animate-spin" />
      {{ editingContact ? 'Update' : 'Create' }}
    </Button>
  </div>
</template>
