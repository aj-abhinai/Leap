<script setup lang="ts">
import { Settings } from '@lucide/vue'
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

const navItems = [
  {
    title: 'Dashboard',
    url: '/',
    icon: 'LayoutDashboard',
  },
  {
    title: 'Contacts',
    url: '/contacts',
    icon: 'Users',
  },
  {
    title: 'Leads',
    url: '/leads',
    icon: 'Folder',
  },
]
</script>

<template>
  <Sidebar collapsible="icon">
    <SidebarHeader>
      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton size="lg">
            <div class="flex aspect-square size-8 items-center justify-center rounded-md group-data-[collapsible=icon]:size-5 bg-linear-to-br from-primary to-primary/70 text-primary-foreground shadow-sm">
              <span class="text-sm font-bold">C</span>
            </div>
            <div class="grid flex-1 text-left text-sm leading-tight group-data-[collapsible=icon]:hidden">
              <span class="truncate font-semibold">CRM</span>
              <span class="truncate text-xs">Prayaan OS</span>
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
