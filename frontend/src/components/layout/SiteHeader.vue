<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Sun, Moon } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { SidebarTrigger } from '@/components/ui/sidebar'
import { Separator } from '@/components/ui/separator'
import { useThemeStore } from '@/stores/theme'

const route = useRoute()
const theme = useThemeStore()
const title = computed(() => (route.meta.title as string) || 'CRM')
</script>

<template>
  <header class="flex h-12 shrink-0 items-center gap-2 px-4">
    <SidebarTrigger class="-ml-1" />
    <Separator orientation="vertical" class="mr-2 h-4" />
    <h1 class="flex-1 text-sm font-semibold">{{ title }}</h1>
    <Button variant="ghost" size="icon" @click="theme.toggle()" :aria-label="theme.resolvedTheme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'">
      <Sun v-if="theme.resolvedTheme === 'dark'" class="size-4" />
      <Moon v-else class="size-4" />
    </Button>
  </header>
</template>
