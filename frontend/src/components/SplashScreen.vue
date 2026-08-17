<script setup lang="ts">
defineProps<{ visible: boolean }>()
</script>

<template>
  <Transition name="splash">
    <div v-if="visible" class="pointer-events-none fixed inset-0 z-[60] flex items-center justify-center bg-background">
      <div
        class="size-16 rounded-2xl bg-cover bg-center shadow-lg"
        style="background-image: url('/logo.png')"
        role="img"
        aria-label="Leap logo"
      ></div>
    </div>
  </Transition>
</template>

<style scoped>
.splash-enter-active {
  transition: opacity 150ms ease-out;
}
.splash-enter-from {
  opacity: 0;
}
.splash-leave-active {
  transition: opacity 250ms ease-in;
}
.splash-leave-to {
  opacity: 0;
}
</style>
