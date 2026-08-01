<script setup lang="ts">
import { onMounted } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import SettingsTagStatusCard from '@/components/settings/SettingsTagStatusCard.vue'

const store = useSettingsStore()

onMounted(() => {
  store.fetchTags()
})
</script>

<template>
  <div class="grid gap-6 md:grid-cols-2">
    <SettingsTagStatusCard
      kind="tag"
      title="Tags"
      placeholder="Tag name"
    />
    <SettingsTagStatusCard
      kind="status"
      title="Statuses"
      placeholder="Status name"
    />
  </div>
</template>
