<script setup lang="ts">
import { onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { Toaster } from '@/components/ui/sonner'

const auth = useAuthStore()
const theme = useThemeStore()

onMounted(() => {
  theme.init()
  auth.bootstrap()
})
</script>

<template>
  <router-view />
  <Toaster rich-colors close-button />
</template>
