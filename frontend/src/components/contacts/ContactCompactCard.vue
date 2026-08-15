<script setup lang="ts">
import { computed } from 'vue'
import { type Contact } from '@/stores/contacts'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Phone } from '@lucide/vue'
import { getAvatarColor, getInitials } from '@/utils/avatar'

const props = defineProps<{
  contact: Contact
}>()

const emit = defineEmits<{
  click: []
}>()

const avatarColor = computed(() => getAvatarColor(props.contact.name))
const avatarInitials = computed(() => getInitials(props.contact.name))

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    emit('click')
  }
}
</script>

<template>
  <Card
    role="button"
    tabindex="0"
    :aria-label="`Open ${contact.name}`"
    class="cursor-pointer hover:shadow-md hover:border-primary/20 transition-all active:scale-[0.99] focus-visible:outline-2 focus-visible:outline-primary"
    @click="emit('click')"
    @keydown="onKeydown"
  >
    <CardContent class="p-4">
      <div class="flex items-start gap-3">
        <div
          class="flex size-10 shrink-0 items-center justify-center rounded-full text-xs font-medium"
          :class="avatarColor"
        >
          {{ avatarInitials }}
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
