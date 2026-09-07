<script setup lang="ts">
import { onMounted } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import SettingsTagStatusCard from '@/components/settings/SettingsTagStatusCard.vue'
import SettingsTabPipelines from '@/components/settings/SettingsTabPipelines.vue'
import SettingsTabPrograms from '@/components/settings/SettingsTabPrograms.vue'

// Sales tab: the lead workflow's configuration — pipelines and stages,
// programs, and the three quick-reply/task/loss word lists. Without
// settings:manage everything renders read-only.
defineProps<{ readonly?: boolean }>()

const store = useSettingsStore()

onMounted(() => {
  store.fetchTags()
})
</script>

<template>
  <div class="space-y-4">
    <SettingsTabPipelines :readonly="readonly" />
    <SettingsTabPrograms :readonly="readonly" />
    <SettingsTagStatusCard kind="quick_reply" title="Quick Replies" placeholder="e.g. No Reply, Busy" :readonly="readonly" />
    <SettingsTagStatusCard kind="activity_type" title="Task types" placeholder="e.g. Call, Email" :readonly="readonly" />
    <SettingsTagStatusCard kind="loss_reason" title="Loss Reasons" placeholder="e.g. Not interested, Budget" :readonly="readonly" />
  </div>
</template>