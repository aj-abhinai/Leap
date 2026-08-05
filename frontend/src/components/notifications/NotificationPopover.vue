<script setup lang="ts">
import { shallowRef, watch, onMounted } from 'vue'
import { Bell } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useRemindersStore } from '@/stores/reminders'
import NotificationPanel from './NotificationPanel.vue'

const store = useRemindersStore()
const isOpen = shallowRef(false)

onMounted(() => store.fetchReminders())

watch(isOpen, (open) => {
  if (open) store.fetchReminders()
})
</script>

<template>
  <Popover v-model:open="isOpen">
    <PopoverTrigger as-child>
      <Button variant="ghost" size="icon" class="relative h-9 w-9" aria-label="Notifications">
        <Bell class="h-5 w-5" />
        <span
          v-if="store.pendingCount > 0"
          class="absolute top-0.5 right-0.5 inline-flex size-4 items-center justify-center rounded-full bg-destructive text-[10px] font-medium leading-none text-destructive-foreground"
        >
          {{ store.pendingCount > 99 ? '99' : store.pendingCount }}
        </span>
      </Button>
    </PopoverTrigger>
    <PopoverContent side="right" :side-offset="8" align="end" class="w-96 p-0">
      <NotificationPanel @close="isOpen = false" />
    </PopoverContent>
  </Popover>
</template>
