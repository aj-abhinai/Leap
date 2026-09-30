<script setup lang="ts">
import { onMounted } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import SettingsTagStatusCard from '@/components/settings/SettingsTagStatusCard.vue'
import SettingsSectionMenu from '@/components/settings/SettingsSectionMenu.vue'

// Contacts tab: the two lists the contact workflow reads. Without
// settings:manage the cards render read-only. The section menu starts at
// Tags on each visit (ADR 015).
defineProps<{ readonly?: boolean }>()

const store = useSettingsStore()

const sections = [
  { value: 'tags', label: 'Tags' },
  { value: 'statuses', label: 'Statuses' },
]

onMounted(() => {
  store.fetchTags()
})
</script>

<template>
  <SettingsSectionMenu :sections="sections">
    <template #tags>
      <SettingsTagStatusCard kind="tag" title="Tags" placeholder="Tag name" :readonly="readonly" />
    </template>
    <template #statuses>
      <SettingsTagStatusCard kind="status" title="Statuses" placeholder="Status name" :readonly="readonly" />
    </template>
  </SettingsSectionMenu>
</template>
