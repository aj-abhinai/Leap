<script setup lang="ts">
import { useRoute } from 'vue-router'
import { LayoutDashboard, Users, Folder } from '@lucide/vue'
import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'

const iconMap: Record<string, any> = {
  LayoutDashboard,
  Users,
  Folder,
}

defineProps<{
  items: { title: string; url: string; icon: string }[]
}>()

const route = useRoute()
</script>

<template>
  <SidebarGroup>
    <SidebarGroupLabel>Main</SidebarGroupLabel>
    <SidebarGroupContent>
      <SidebarMenu>
        <SidebarMenuItem v-for="item in items" :key="item.url">
          <SidebarMenuButton
            as-child
            :is-active="route?.path === item.url"
            :tooltip="item.title"
          >
            <router-link :to="item.url" class="flex items-center gap-3 px-2.5 md:px-2">
              <component :is="iconMap[item.icon]" class="size-4 shrink-0" />
              <span>{{ item.title }}</span>
            </router-link>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
    </SidebarGroupContent>
  </SidebarGroup>
</template>
