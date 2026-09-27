<script setup lang="ts">
import { shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import AuthCard from '@/components/auth/AuthCard.vue'
import { LogIn } from '@lucide/vue'
import { errorMessage } from '@/utils/errors'
import { useSplash } from '@/composables/useSplash'

const router = useRouter()
const auth = useAuthStore()
const { showSplash, hideSplash } = useSplash()

const email = shallowRef('')
const password = shallowRef('')
const error = shallowRef('')
const loading = shallowRef(false)

// handleSubmit logs in, then routes to Change Password when the server
// requires it, otherwise to the Dashboard; failures surface as an inline error.
async function handleSubmit() {
  error.value = ''
  if (!email.value || !password.value) {
    error.value = 'Email and password are required'
    return
  }
  loading.value = true
  try {
    await auth.login(email.value, password.value)
    showSplash()
    try {
      if (auth.mustChangePassword) {
        await router.push({ name: 'ChangePassword' })
      } else {
        await router.push({ name: 'Dashboard' })
      }
    } finally {
      hideSplash()
    }
  } catch (e) {
    error.value = errorMessage(e, 'Login failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthCard
    title="Leap"
    description="Sign in to your account"
    submit-label="Sign In"
    :error="error"
    :loading="loading"
    @submit="handleSubmit"
  >
    <template #emblem>
      <div
        class="mx-auto mb-3 size-12 rounded-xl bg-cover bg-center shadow-sm"
        style="background-image: url('/logo.png')"
        role="img"
        aria-label="Leap logo"
      ></div>
    </template>
    <template #submit-icon>
      <LogIn class="mr-2 size-4" />
    </template>

    <div class="space-y-2">
      <Label for="email">Email</Label>
      <Input id="email" v-model="email" type="email" placeholder="admin@admin.com" autocomplete="email" />
    </div>
    <div class="space-y-2">
      <Label for="password">Password</Label>
      <Input
        id="password"
        v-model="password"
        type="password"
        placeholder="Enter your password"
        autocomplete="current-password"
      />
    </div>
  </AuthCard>
</template>
