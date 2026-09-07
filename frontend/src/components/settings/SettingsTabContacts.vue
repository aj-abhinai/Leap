<script setup lang="ts">
import { onMounted } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import SettingsTagStatusCard from '@/components/settings/SettingsTagStatusCard.vue'

// Contacts tab: the two lists the contact workflow reads. Without
// settings:manage the cards render read-only.
defineProps<{ readonly?: boolean }>()

const store = useSettingsStore()

onMounted(() => {
  store.fetchTags()
})
</script>

<template>
  <div class="space-y-4">
    <SettingsTagStatusCard kind="tag" title="Tags" placeholder="Tag name" :readonly="readonly" />
    <SettingsTagStatusCard kind="status" title="Statuses" placeholder="Status name" :readonly="readonly" />
  </div>
</template>