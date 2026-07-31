<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Loader2, LogIn } from '@lucide/vue'

const router = useRouter()
const auth = useAuthStore()

const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleSubmit() {
  error.value = ''
  if (!email.value || !password.value) {
    error.value = 'Email and password are required'
    return
  }
  loading.value = true
  try {
    await auth.login(email.value, password.value)
    router.push('/')
  } catch (e: any) {
    error.value = e.message || 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="relative flex min-h-screen items-center justify-center p-4">
    <div class="absolute inset-0 bg-linear-to-br from-primary/5 via-background to-primary/5 animate-gradient" />
    <div class="absolute inset-0 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-primary/5 via-transparent to-transparent" />
    <Card class="relative w-full max-w-sm shadow-lg animate-fade-in-up">
      <CardHeader class="text-center pb-2">
        <div class="mx-auto mb-3 flex size-12 items-center justify-center rounded-xl bg-linear-to-br from-primary to-primary/70 shadow-sm">
          <span class="text-xl font-bold text-primary-foreground">C</span>
        </div>
        <CardTitle class="text-2xl">Prayaan CRM</CardTitle>
        <CardDescription>Sign in to your account</CardDescription>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="handleSubmit" class="space-y-4">
          <div class="space-y-2">
            <Label for="email">Email</Label>
            <Input id="email" v-model="email" type="email" placeholder="admin@crm.local" autocomplete="email" />
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
