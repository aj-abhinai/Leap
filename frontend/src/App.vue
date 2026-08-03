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
  <router-view />
  <Toaster rich-colors close-button />
</template>
