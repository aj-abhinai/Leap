<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Sun, Moon, ChevronRight, Home } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { SidebarTrigger } from '@/components/ui/sidebar'
import { Separator } from '@/components/ui/separator'
import { useThemeStore } from '@/stores/theme'

const route = useRoute()
const theme = useThemeStore()
const title = computed(() => (route.meta.title as string) || 'CRM')

const breadcrumbs = computed(() => {
  const crumbs: { title: string; path?: string }[] = []
  const matched = route.matched

  for (let i = 0; i < matched.length; i++) {
    const meta = matched[i].meta
    if (meta.title) {
      crumbs.push({
        title: meta.title as string,
        path: i < matched.length - 1 ? matched[i].path : undefined,
      })
    }
  }

  if (crumbs.length === 0 && route.path === '/') {
    crumbs.push({ title: 'Dashboard' })
  }

  return crumbs
})
</script>

<template>
  <header class="flex h-12 shrink-0 items-center gap-2 border-b px-4">
    <SidebarTrigger class="h-9 w-9 md:h-7 md:w-7" />
    <Separator orientation="vertical" class="mr-2 h-4" />
    <nav class="flex min-w-0 flex-1 items-center gap-1 overflow-hidden text-sm text-muted-foreground">
      <Home class="size-3.5 shrink-0" />
      <template v-for="(crumb, i) in breadcrumbs" :key="i">
        <ChevronRight v-if="i > 0" class="size-3.5 shrink-0" />
        <span v-if="i === breadcrumbs.length - 1" class="truncate font-medium text-foreground">
          {{ crumb.title }}
        </span>
        <router-link v-else :to="crumb.path || '/'" class="truncate hover:text-foreground transition-colors">
          {{ crumb.title }}
        </router-link>
      </template>
    </nav>
    <Button
      class="ml-auto shrink-0"
      variant="ghost"
      size="icon"
      @click="theme.toggle()"
      :aria-label="theme.resolvedTheme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
    >
      <Sun v-if="theme.resolvedTheme === 'dark'" class="size-4" />
      <Moon v-else class="size-4" />
    </Button>
  </header>
</template>
