<script setup lang="ts">
import { shallowRef, watch, onMounted, onBeforeUnmount } from 'vue'
import { Bell } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useRemindersStore } from '@/stores/reminders'
import NotificationPanel from './NotificationPanel.vue'

const store = useRemindersStore()
const isOpen = shallowRef(false)

// The bell polls every 60 seconds so a nudge arrives at the due moment even
// when the user is not looking; the interval pauses while the tab is hidden
// to avoid background churn. Opening the panel also refreshes.
let timer: ReturnType<typeof setInterval> | null = null

function startPolling() {
  stopPolling()
  timer = setInterval(() => {
    if (!document.hidden) store.fetchReminders()
  }, 60_000)
}

function stopPolling() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

onMounted(() => {
  store.fetchReminders()
  startPolling()
  document.addEventListener('visibilitychange', onVisibility)
})

onBeforeUnmount(() => {
  stopPolling()
  document.removeEventListener('visibilitychange', onVisibility)
})

function onVisibility() {
  if (document.hidden) {
    stopPolling()
  } else {
    store.fetchReminders()
    startPolling()
  }
}

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
