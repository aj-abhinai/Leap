<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { profileSchema } from '@/lib/validation'
import LayoutShell from '@/components/layout/LayoutShell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Loader2, User } from '@lucide/vue'

const auth = useAuthStore()

const formName = ref(auth.user?.name || '')
const formEmail = ref(auth.user?.email || '')
const formPhone = ref(auth.user?.phone || '')
const error = ref('')
const saving = ref(false)

async function handleSave() {
  error.value = ''
  const result = profileSchema.safeParse({
    name: formName.value,
    phone: formPhone.value || undefined,
  })
  if (!result.success) {
    error.value = result.error.errors[0]?.message || 'Validation failed'
    return
  }
  saving.value = true
  try {
    await auth.updateProfile(result.data.name, result.data.phone || '')
    toast.success('Profile updated')
  } catch (e: any) {
    error.value = e.message || 'Failed to save profile'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <LayoutShell>
    <div class="flex flex-1 flex-col gap-4 p-6 pt-2">
      <Card class="max-w-md">
        <CardHeader>
          <div class="flex items-center gap-2">
            <div class="flex size-8 items-center justify-center rounded-md bg-muted">
              <User class="size-4" />
            </div>
            <CardTitle>Profile</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <form @submit.prevent="handleSave" class="space-y-4">
            <div class="space-y-2">
              <Label for="pname">Name</Label>
              <Input id="pname" v-model="formName" placeholder="Your name" />
            </div>
            <div class="space-y-2">
              <Label for="pemail">Email</Label>
              <Input id="pemail" :model-value="formEmail" disabled class="opacity-60" />
            </div>
            <div class="space-y-2">
              <Label for="pphone">Phone</Label>
              <Input id="pphone" v-model="formPhone" placeholder="Phone number" />
            </div>
            <div v-if="error" class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{{ error }}</div>
            <Button type="submit" :disabled="saving">
              <Loader2 v-if="saving" class="mr-2 size-4 animate-spin" />
              Save
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  </LayoutShell>
</template>
