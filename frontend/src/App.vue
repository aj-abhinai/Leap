<script setup lang="ts">
import { onMounted } from 'vue'
import { useThemeStore } from '@/stores/theme'
import { Toaster } from '@/components/ui/sonner'

const theme = useThemeStore()

onMounted(() => {
  theme.init()
})
</script>

<template>
  <router-view />
  <Toaster rich-colors close-button />
</template>
