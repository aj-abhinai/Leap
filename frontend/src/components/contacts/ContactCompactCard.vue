<script setup lang="ts">
import { type Contact } from '@/stores/contacts'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Phone } from '@lucide/vue'

defineProps<{
  contact: Contact
}>()

const emit = defineEmits<{
  click: []
}>()

function getInitials(name: string): string {
  return name
    .split(' ')
    .map(n => n.charAt(0))
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

function getAvatarColor(name: string): string {
  const colors = [
    'bg-blue-100 text-blue-700',
    'bg-emerald-100 text-emerald-700',
    'bg-amber-100 text-amber-700',
    'bg-violet-100 text-violet-700',
    'bg-rose-100 text-rose-700',
    'bg-cyan-100 text-cyan-700',
  ]
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return colors[Math.abs(hash) % colors.length]
}
</script>

<template>
  <Card class="cursor-pointer hover:shadow-md hover:border-primary/20 transition-all" @click="emit('click')">
    <CardContent class="p-4">
      <div class="flex items-start gap-3">
        <div
          class="flex size-10 shrink-0 items-center justify-center rounded-full text-xs font-medium"
          :class="getAvatarColor(contact.name)"
        >
          {{ getInitials(contact.name) }}
        </div>
        <div class="min-w-0 flex-1">
          <div class="font-semibold truncate">{{ contact.name }}</div>
          <div v-if="contact.phone" class="flex items-center gap-1 mt-0.5 text-xs text-muted-foreground">
            <Phone class="size-3" />
            {{ contact.phone }}
          </div>
          <div v-if="contact.email" class="text-xs text-muted-foreground truncate mt-0.5">
            {{ contact.email }}
          </div>
          <div class="flex flex-wrap gap-1 mt-2">
            <Badge v-if="contact.status" variant="secondary" class="text-xs">{{ contact.status.name }}</Badge>
            <Badge v-for="t in (contact.tags || [])" :key="t.id" variant="outline" class="text-xs">
              {{ t.name }}
            </Badge>
          </div>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
