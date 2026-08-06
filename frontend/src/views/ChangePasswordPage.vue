<script setup lang="ts">
import { shallowRef, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Loader2, KeyRound } from '@lucide/vue'

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

async function handleSubmit() {
  error.value = ''
  if (!currentPassword.value || !newPassword.value || !confirmPassword.value) {
    error.value = 'All fields are required'
    return
  }
  if (newPassword.value.length < 8) {
    error.value = 'New password must be at least 8 characters'
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
  } catch (e: any) {
    error.value = e.message || 'Failed to change password'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="relative flex min-h-screen items-center justify-center p-4">
    <Card class="relative w-full max-w-sm shadow-lg">
      <CardHeader class="text-center pb-2">
        <div class="mx-auto mb-3 flex size-12 items-center justify-center rounded-xl bg-linear-to-br from-primary to-primary/70 shadow-sm">
          <KeyRound class="size-6 text-primary-foreground" />
        </div>
        <CardTitle class="text-2xl">Change Password</CardTitle>
        <p v-if="banner" class="mt-2 text-sm text-muted-foreground">{{ banner }}</p>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="handleSubmit" class="space-y-4">
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
              placeholder="At least 8 characters"
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
          <div v-if="error" class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{{ error }}</div>
          <Button type="submit" class="w-full" :disabled="loading">
            <Loader2 v-if="loading" class="mr-2 size-4 animate-spin" />
            <KeyRound v-else class="mr-2 size-4" />
            Change Password
          </Button>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
