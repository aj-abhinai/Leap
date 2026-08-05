<script setup lang="ts">
import { onMounted } from 'vue'
import { useThemeStore } from '@/stores/theme'
import { useRBACStore } from '@/stores/rbac'
import { Toaster } from '@/components/ui/sonner'

const theme = useThemeStore()
const rbac = useRBACStore()

onMounted(() => {
  theme.init()
  rbac.fetchPermissions()
})
</script>

<template>
  <RouterView v-slot="{ Component }">
    <Transition name="page">
      <component :is="Component" />
    </Transition>
  </RouterView>
  <Toaster rich-colors close-button />
</template>
