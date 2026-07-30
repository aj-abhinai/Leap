<script setup lang="ts">
import { useRoute } from 'vue-router'
import { LayoutDashboard, Users, Folder, ListChecks } from '@lucide/vue'
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'

const iconMap: Record<string, any> = {
  LayoutDashboard,
  Users,
  Folder,
  ListChecks,
}

defineProps<{
  items: { title: string; url: string; icon: string }[]
}>()

const route = useRoute()
</script>

<template>
  <SidebarMenu>
    <SidebarMenuItem v-for="item in items" :key="item.url">
      <SidebarMenuButton
        as-child
        :is-active="route?.path === item.url"
        :tooltip="item.title"
      >
        <router-link :to="item.url" class="flex items-center gap-2 px-2.5 md:px-2">
          <component :is="iconMap[item.icon]" class="size-4" />
          <span>{{ item.title }}</span>
        </router-link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  </SidebarMenu>
</template>
