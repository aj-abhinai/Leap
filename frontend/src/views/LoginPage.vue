<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Loader2 } from '@lucide/vue'

const router = useRouter()
const auth = useAuthStore()

const email = ref('admin@crm.local')
const password = ref('admin')
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
  <div class="flex min-h-screen items-center justify-center bg-muted/50 p-4">
    <Card class="w-full max-w-sm">
      <CardHeader class="text-center">
        <CardTitle class="text-2xl">CRM</CardTitle>
        <CardDescription>Sign in to your account</CardDescription>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="handleSubmit" class="space-y-4">
          <div class="space-y-2">
            <Label for="email">Email</Label>
            <Input id="email" v-model="email" type="email" placeholder="admin@crm.local" />
          </div>
          <div class="space-y-2">
            <Label for="password">Password</Label>
            <Input id="password" v-model="password" type="password" placeholder="admin" />
          </div>
          <div v-if="error" class="text-sm text-destructive">{{ error }}</div>
          <Button type="submit" class="w-full" :disabled="loading">
            <Loader2 v-if="loading" class="mr-2 size-4 animate-spin" />
            Sign In
          </Button>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
