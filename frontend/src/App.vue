<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useThemeStore } from '@/stores/theme'
import { useRBACStore } from '@/stores/rbac'
import { Toaster } from '@/components/ui/sonner'
import SplashScreen from '@/components/SplashScreen.vue'
import { useSplash } from '@/composables/useSplash'

const router = useRouter()
const theme = useThemeStore()
const rbac = useRBACStore()
const { splashVisible, showSplash, hideSplash } = useSplash()

onMounted(() => {
  theme.init()
  rbac.fetchPermissions()
  prefetchRouteChunks()
  showSplash()
  hideSplash()
})

// Warm the lazy route chunks in the background so the first navigation has no
// load gap mid-transition. Fire-and-forget; failures are harmless.
function prefetchRouteChunks() {
  const idle = window.requestIdleCallback ?? ((cb: () => void) => setTimeout(cb, 1000))
  idle(() => {
    for (const route of router.getRoutes()) {
      const loader = route.components?.default
      if (typeof loader === 'function') (loader as () => Promise<unknown>)().catch(() => {})
    }
  })
}
</script>

<template>
  <RouterView />
  <Toaster rich-colors close-button />
  <SplashScreen :visible="splashVisible" />
</template>
