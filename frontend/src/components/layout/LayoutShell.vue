<script setup lang="ts">
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import AppSidebar from './AppSidebar.vue'
import SiteHeader from './SiteHeader.vue'
import LeadActivityDrawer from '@/components/leads/LeadActivityDrawer.vue'
</script>

<template>
  <SidebarProvider>
    <AppSidebar />
    <SidebarInset>
      <SiteHeader />
      <!-- Keyed by path so page-to-page navigation cross-fades while the
           shell stays put; query/filter changes keep the same key and do not
           remount the page. -->
      <div class="relative flex min-w-0 flex-1 flex-col">
        <RouterView v-slot="{ Component, route }">
          <Transition name="page">
            <div :key="route.path" class="flex min-w-0 flex-1 flex-col">
              <component :is="Component" />
            </div>
          </Transition>
        </RouterView>
      </div>
    </SidebarInset>
    <!-- App-level lead drawer: opens over any page with full lead context. -->
    <LeadActivityDrawer />
  </SidebarProvider>
</template>
