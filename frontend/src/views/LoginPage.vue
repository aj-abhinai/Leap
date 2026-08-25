<script setup lang="ts">
import { shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Loader2, LogIn } from '@lucide/vue'
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
  <div class="relative flex min-h-screen items-center justify-center p-4">
    <Card class="relative w-full max-w-sm shadow-lg">
      <CardHeader class="text-center pb-2">
        <div
          class="mx-auto mb-3 size-12 rounded-xl bg-cover bg-center shadow-sm"
          style="background-image: url('/logo.png')"
          role="img"
          aria-label="Leap logo"
        ></div>
        <CardTitle class="text-2xl">Leap</CardTitle>
        <CardDescription>Sign in to your account</CardDescription>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="handleSubmit" class="space-y-4">
          <div class="space-y-2">
            <Label for="email">Email</Label>
            <Input id="email" v-model="email" type="email" placeholder="admin@admin.com" autocomplete="email" />
          </div>
          <div class="space-y-2">
            <Label for="password">Password</Label>
            <Input id="password" v-model="password" type="password" placeholder="Enter your password" autocomplete="current-password" />
          </div>
          <div v-if="error" class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{{ error }}</div>
          <Button type="submit" class="w-full" :disabled="loading">
            <Loader2 v-if="loading" class="mr-2 size-4 animate-spin" />
            <LogIn v-else class="mr-2 size-4" />
            Sign In
          </Button>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
