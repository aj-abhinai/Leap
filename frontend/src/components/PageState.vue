<script setup lang="ts">
import { Skeleton } from '@/components/ui/skeleton'
import ErrorAlert from '@/components/ui/ErrorAlert.vue'
import { ClipboardList } from '@lucide/vue'

// PageState renders the standard page states — loading skeletons, an error
// banner with retry, or an empty message — so views stop re-inlining the
// trio. Only one state renders at a time, in this order: loading > error >
// empty. The default slot is the content shown in the normal state.
withDefaults(
  defineProps<{
    loading?: boolean
    error?: string
    empty?: boolean
    emptyTitle?: string
    emptyHint?: string
    skeletonCount?: number
    skeletonClass?: string
  }>(),
  {
    loading: false,
    error: '',
    empty: false,
    emptyTitle: '',
    emptyHint: '',
    skeletonCount: 5,
    skeletonClass: 'h-12 w-full',
  },
)

const emit = defineEmits<{ retry: [] }>()
</script>

<template>
  <div v-if="loading" class="space-y-3">
    <Skeleton v-for="i in skeletonCount" :key="i" class="w-full" :class="skeletonClass" />
  </div>
  <ErrorAlert v-else-if="error" :error="error" title="Failed to load" @retry="emit('retry')" />
  <div v-else-if="empty" class="flex flex-col items-center justify-center py-16 text-center">
    <slot name="empty-icon">
      <ClipboardList class="mb-3 size-10 text-muted-foreground/40" />
    </slot>
    <p class="text-sm font-medium text-muted-foreground">{{ emptyTitle }}</p>
    <p v-if="emptyHint" class="mt-1 text-xs text-muted-foreground/60">{{ emptyHint }}</p>
  </div>
  <slot v-else />
</template>