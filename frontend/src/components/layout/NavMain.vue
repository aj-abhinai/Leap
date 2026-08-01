<script setup lang="ts">
import type { Component } from 'vue'
import { useRoute } from 'vue-router'
import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'

defineProps<{
  items: { title: string; url: string; icon: Component }[]
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
              <component :is="item.icon" class="size-4 shrink-0" />
              <span>{{ item.title }}</span>
            </router-link>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
    </SidebarGroupContent>
  </SidebarGroup>
</template>
