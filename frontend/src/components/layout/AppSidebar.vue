<script setup lang="ts">
import type { Component } from 'vue'
import { Settings, LayoutDashboard, Users, Folder, CalendarCheck } from '@lucide/vue'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
} from '@/components/ui/sidebar'
import NavMain from './NavMain.vue'
import NavUser from './NavUser.vue'
import NotificationPopover from '@/components/notifications/NotificationPopover.vue'

const navItems: { title: string; url: string; icon: Component }[] = [
  {
    title: 'Dashboard',
    url: '/',
    icon: LayoutDashboard,
  },
  {
    title: 'Contacts',
    url: '/contacts',
    icon: Users,
  },
  {
    title: 'Leads',
    url: '/leads',
    icon: Folder,
  },
  {
    title: 'Activities',
    url: '/activities',
    icon: CalendarCheck,
  },
]
</script>

<template>
  <Sidebar collapsible="icon">
    <SidebarHeader>
      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton size="lg">
            <div
              class="aspect-square size-8 rounded-md bg-cover bg-center group-data-[collapsible=icon]:size-5 shadow-sm"
              style="background-image: url('/logo.png')"
              role="img"
              aria-label="Leap logo"
            ></div>
            <div class="grid flex-1 text-left text-sm leading-tight group-data-[collapsible=icon]:hidden">
              <span class="truncate font-semibold">Leap</span>
              <span class="truncate text-xs">CRM</span>
            </div>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
    </SidebarHeader>
    <SidebarContent>
      <NavMain :items="navItems" />
    </SidebarContent>
    <SidebarFooter>
      <SidebarSeparator />
      <SidebarMenu>
        <SidebarMenuItem>
          <div class="flex items-center justify-center">
            <NotificationPopover />
          </div>
        </SidebarMenuItem>
        <SidebarMenuItem>
          <SidebarMenuButton as-child tooltip="Settings">
            <router-link to="/settings" class="flex items-center gap-2">
              <Settings class="size-4" />
              <span>Settings</span>
            </router-link>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
      <NavUser />
    </SidebarFooter>
  </Sidebar>
</template>
