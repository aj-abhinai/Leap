<script setup lang="ts">
import { shallowRef, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import AuthCard from '@/components/auth/AuthCard.vue'
import { KeyRound } from '@lucide/vue'
import { PASSWORD_POLICY_HINT, isStrongPassword } from '@/lib/validation'
import { errorMessage } from '@/utils/errors'

const router = useRouter()
const auth = useAuthStore()

const currentPassword = shallowRef('')
const newPassword = shallowRef('')
const confirmPassword = shallowRef('')
const error = shallowRef('')
const loading = shallowRef(false)

const banner = computed(() =>
  auth.mustChangePassword
    ? 'You must change your password before continuing.'
    : '',
)

// handleSubmit validates the form, changes the password, and routes to the
// Dashboard on success; validation failures surface as an inline error.
async function handleSubmit() {
  error.value = ''
  if (!currentPassword.value || !newPassword.value || !confirmPassword.value) {
    error.value = 'All fields are required'
    return
  }
  if (!isStrongPassword(newPassword.value)) {
    error.value = PASSWORD_POLICY_HINT
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    error.value = 'Passwords do not match'
    return
  }
  loading.value = true
  try {
    await auth.changePassword(currentPassword.value, newPassword.value)
    toast.success('Password changed')
    router.push({ name: 'Dashboard' })
  } catch (e) {
    error.value = errorMessage(e, 'Failed to change password')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthCard
    title="Change Password"
    submit-label="Change Password"
    :error="error"
    :loading="loading"
    @submit="handleSubmit"
  >
    <template #emblem>
      <div class="mx-auto mb-3 flex size-12 items-center justify-center rounded-xl bg-linear-to-br from-primary to-primary/70 shadow-sm">
        <KeyRound class="size-6 text-primary-foreground" />
      </div>
    </template>
    <template #subtitle>
      <p v-if="banner" class="mt-2 text-sm text-muted-foreground">{{ banner }}</p>
    </template>
    <template #submit-icon>
      <KeyRound class="mr-2 size-4" />
    </template>

    <div class="space-y-2">
      <Label for="current">Current password</Label>
      <Input
        id="current"
        v-model="currentPassword"
        type="password"
        autocomplete="current-password"
        placeholder="Enter current password"
      />
    </div>
    <div class="space-y-2">
      <Label for="npw">New password</Label>
      <Input
        id="npw"
        v-model="newPassword"
        type="password"
        autocomplete="new-password"
        placeholder="10+ chars with upper, lower, digit, special"
      />
    </div>
    <div class="space-y-2">
      <Label for="cpw">Confirm new password</Label>
      <Input
        id="cpw"
        v-model="confirmPassword"
        type="password"
        autocomplete="new-password"
        placeholder="Re-enter new password"
      />
    </div>
  </AuthCard>
</template>
