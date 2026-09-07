<script setup lang="ts">
import { shallowRef, computed, onMounted, watch, type Component } from 'vue'
import { useRBACStore } from '@/stores/rbac'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Card, CardContent } from '@/components/ui/card'
import SettingsTabContacts from '@/components/settings/SettingsTabContacts.vue'
import SettingsTabSales from '@/components/settings/SettingsTabSales.vue'
import SettingsTabUsers from '@/components/settings/SettingsTabUsers.vue'
import SettingsTabRoles from '@/components/settings/SettingsTabRoles.vue'
import SettingsTabGeneral from '@/components/settings/SettingsTabGeneral.vue'
import SettingsTabAudit from '@/components/settings/SettingsTabAudit.vue'
import { Tags, Briefcase, User, Settings, ScrollText, Lock } from '@lucide/vue'

const rbac = useRBACStore()
const permissionsLoaded = shallowRef(false)
const activeTab = shallowRef('')

interface SettingsTab {
  value: string
  label: string
  icon: Component
  // visible lists the permissions that unlock the tab; any one suffices.
  permissions: string[]
}

// The visibility rule: a Settings tab is visible to anyone who can read its
// domain (Contacts ← contact:read, Sales ← lead:read); the admin surfaces
// (Team, General, Audit log) stay settings:manage, with the audit tab also
// reachable through data:export for its export card.
const tabs: SettingsTab[] = [
  { value: 'contacts', label: 'Contacts', icon: Tags, permissions: ['contact:read'] },
  { value: 'sales', label: 'Sales', icon: Briefcase, permissions: ['lead:read'] },
  { value: 'team', label: 'Team', icon: User, permissions: ['settings:manage'] },
  { value: 'general', label: 'General', icon: Settings, permissions: ['settings:manage'] },
  { value: 'audit', label: 'Audit log', icon: ScrollText, permissions: ['settings:manage', 'data:export'] },
]

const visibleTabs = computed(() => {
  if (!permissionsLoaded.value) return tabs
  return tabs.filter((t) => t.permissions.some((p) => rbac.can(p)))
})

const readonly = computed(() => permissionsLoaded.value && !rbac.can('settings:manage'))

onMounted(async () => {
  try {
    await rbac.fetchPermissions()
  } finally {
    // Permissions resolve in every path so the tabs never stay stuck.
    permissionsLoaded.value = true
    selectFirstVisible()
  }
})

// The default tab is the first visible one: a contacts-only viewer lands on
// Contacts, an admin on Contacts too (the domain tabs come first).
function selectFirstVisible() {
  const visible = visibleTabs.value
  if (visible.length === 0) return
  if (!visible.some((t) => t.value === activeTab.value)) {
    activeTab.value = visible[0].value
  }
}

watch(visibleTabs, (visible) => {
  // A mid-session permission change (unlikely) falls back to the first
  // visible tab rather than leaving a hidden tab active.
  if (visible.length > 0 && !visible.some((t) => t.value === activeTab.value)) {
    activeTab.value = visible[0].value
  }
})

function tabIcon(tab: SettingsTab): Component {
  return tab.icon
}
</script>

<template>
  <div class="flex flex-1 flex-col gap-4 p-6 pt-2">
    <div class="flex flex-col">
      <h1 class="text-2xl font-semibold tracking-tight">Settings</h1>
      <p class="mt-0.5 text-sm text-muted-foreground">Workspace configuration, access, and activity</p>
    </div>
    <div v-if="permissionsLoaded && visibleTabs.length === 0" class="flex justify-center pt-16">
      <Card class="w-full max-w-md">
        <CardContent class="flex flex-col items-center py-10 text-center">
          <Lock class="size-10 text-muted-foreground/40 mb-3" />
          <p class="text-sm font-medium text-muted-foreground">Settings are not available for your role</p>
          <p class="text-xs text-muted-foreground/60 mt-1">
            Ask an administrator for a role with access to contacts, leads, or settings.
          </p>
        </CardContent>
      </Card>
    </div>
    <Tabs v-else :model-value="activeTab" @update:model-value="(v) => (activeTab = String(v))" class="w-full">
      <TabsList class="mb-4 w-full justify-start overflow-x-auto rounded-lg border bg-muted/50 p-1">
        <TabsTrigger
          v-for="tab in visibleTabs"
          :key="tab.value"
          :value="tab.value"
          class="gap-2 rounded-md data-[state=active]:bg-background data-[state=active]:shadow-sm"
        >
          <component :is="tabIcon(tab)" class="size-4" />
          <span class="hidden sm:inline">{{ tab.label }}</span>
        </TabsTrigger>
      </TabsList>

      <TabsContent v-for="tab in visibleTabs" :key="tab.value" :value="tab.value" class="mt-0">
        <SettingsTabContacts v-if="tab.value === 'contacts'" :readonly="readonly" />
        <SettingsTabSales v-else-if="tab.value === 'sales'" :readonly="readonly" />
        <template v-else-if="tab.value === 'team'">
          <div class="space-y-4">
            <SettingsTabUsers />
            <SettingsTabRoles />
          </div>
        </template>
        <SettingsTabGeneral v-else-if="tab.value === 'general'" />
        <SettingsTabAudit v-else-if="tab.value === 'audit'" />
      </TabsContent>
    </Tabs>
  </div>
</template>