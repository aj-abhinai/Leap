<script setup lang="ts">
import { ChevronDown, LogOut, User } from '@lucide/vue'
import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'
import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'

const auth = useAuthStore()
const router = useRouter()

function handleLogout() {
  auth.logout()
  router.push('/login')
}
</script>

<template>
  <SidebarMenu>
    <SidebarMenuItem>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <SidebarMenuButton
            size="lg"
            :tooltip="auth.user?.name || 'User'"
          >
            <div class="relative">
              <Avatar class="size-8 rounded-md group-data-[collapsible=icon]:size-5">
                <AvatarImage :src="auth.user?.avatar_url || ''" />
                <AvatarFallback class="rounded-md">
                  {{ auth.user?.name?.charAt(0)?.toUpperCase() || 'U' }}
                </AvatarFallback>
              </Avatar>
              <span class="absolute -bottom-0.5 -right-0.5 size-2 rounded-full border-2 border-sidebar bg-emerald-500 ring-2 ring-sidebar group-data-[collapsible=icon]:hidden" />
            </div>
            <div class="grid flex-1 text-left text-sm leading-tight">
              <span class="truncate font-semibold">{{ auth.user?.name || 'User' }}</span>
              <span class="truncate text-xs text-muted-foreground">{{ auth.user?.email || '' }}</span>
            </div>
            <ChevronDown class="ml-auto size-4 opacity-50" />
          </SidebarMenuButton>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          class="w-56"
          side="right"
          align="end"
          :side-offset="4"
        >
          <DropdownMenuLabel>My Account</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem as-child>
            <router-link to="/profile">
              <User class="mr-2 size-4" />
              <span>Profile</span>
            </router-link>
          </DropdownMenuItem>
          <DropdownMenuItem @click="handleLogout">
            <LogOut class="mr-2 size-4" />
            <span>Log out</span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </SidebarMenuItem>
  </SidebarMenu>
</template>
