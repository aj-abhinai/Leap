<script setup lang="ts">
import type { Component } from 'vue'
import { computed, onMounted, shallowRef } from 'vue'
import { Settings, LayoutDashboard, Users, Folder, CalendarCheck, PanelLeftClose } from '@lucide/vue'
import { useRBACStore } from '@/stores/rbac'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
  useSidebar,
} from '@/components/ui/sidebar'
import NavMain from './NavMain.vue'
import NavUser from './NavUser.vue'
import NotificationPopover from '@/components/notifications/NotificationPopover.vue'

const rbac = useRBACStore()
const permissionsLoaded = shallowRef(false)
const { isMobile, toggleSidebar } = useSidebar()

// The Settings nav item appears when any Settings tab is visible; it stays
// shown while permissions load so a fast viewer never sees it flash away.
const settingsVisible = computed(() => {
  if (!permissionsLoaded.value) return true
  return (
    rbac.can('contact:read') ||
    rbac.can('lead:read') ||
    rbac.can('settings:manage') ||
    rbac.can('data:export')
  )
})

onMounted(async () => {
  try {
    await rbac.fetchPermissions()
  } finally {
    permissionsLoaded.value = true
  }
})

const navItems: { title: string; url: string; icon: Component }[] = [
  {
    title: 'Dashboard',
    url: '/',
    icon: LayoutDashboard,
  },
  {
    title: 'Leads',
    url: '/leads',
    icon: Folder,
  },
  {
    title: 'Tasks',
    url: '/activities',
    icon: CalendarCheck,
  },
  {
    title: 'Contacts',
    url: '/contacts',
    icon: Users,
  },
]
</script>

<template>
  <Sidebar collapsible="icon">
    <SidebarHeader>
      <SidebarMenu>
        <SidebarMenuItem class="flex items-center gap-1">
          <SidebarMenuButton size="lg" class="min-w-0 group-data-[collapsible=icon]:shrink-0">
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
          <Tooltip v-if="!isMobile">
            <TooltipTrigger as-child>
              <Button
                variant="ghost"
                size="icon-sm"
                class="shrink-0 text-muted-foreground group-data-[collapsible=icon]:hidden"
                aria-label="Collapse sidebar"
                @click="toggleSidebar"
              >
                <PanelLeftClose class="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">Collapse sidebar</TooltipContent>
          </Tooltip>
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
        <SidebarMenuItem v-if="settingsVisible">
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
