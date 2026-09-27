<script setup lang="ts">
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Loader2 } from '@lucide/vue'

// AuthCard is the shell every signed-out page shares: the centred card, its
// header, the form element, the inline error line, and the submit button with
// its spinner. The page owns the fields, the state, and the submit handler.
// The emblem, the subtitle, and the submit icon are slots because they are the
// only parts that differ between pages.
interface Props {
  title: string
  submitLabel: string
  description?: string
  error?: string
  loading?: boolean
}

withDefaults(defineProps<Props>(), {
  description: '',
  error: '',
  loading: false,
})

const emit = defineEmits<{
  (e: 'submit'): void
}>()
</script>

<template>
  <div class="relative flex min-h-screen items-center justify-center p-4">
    <Card class="relative w-full max-w-sm shadow-lg">
      <CardHeader class="text-center pb-2">
        <slot name="emblem" />
        <CardTitle class="text-2xl">{{ title }}</CardTitle>
        <CardDescription v-if="description">{{ description }}</CardDescription>
        <slot name="subtitle" />
      </CardHeader>
      <CardContent>
        <form class="space-y-4" @submit.prevent="emit('submit')">
          <slot />
          <div v-if="error" class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {{ error }}
          </div>
          <Button type="submit" class="w-full" :disabled="loading">
            <Loader2 v-if="loading" class="mr-2 size-4 animate-spin" />
            <slot v-if="!loading" name="submit-icon" />
            {{ submitLabel }}
          </Button>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
