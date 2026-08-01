<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { ChevronLeft, ChevronRight } from '@lucide/vue'

defineProps<{
  page: number
  totalPages: number
  total: number
}>()

const emit = defineEmits<{
  prev: []
  next: []
}>()
</script>

<template>
  <div class="flex items-center justify-between">
    <span class="text-sm text-muted-foreground">
      Page {{ page }} of {{ totalPages }} &middot; {{ total }} total
    </span>
    <div class="flex items-center gap-1">
      <Button variant="outline" size="sm" :disabled="page <= 1" @click="emit('prev')">
        <ChevronLeft class="size-4" />
        Previous
      </Button>
      <Button variant="outline" size="sm" :disabled="page >= totalPages" @click="emit('next')">
        Next
        <ChevronRight class="size-4" />
      </Button>
    </div>
  </div>
</template>
