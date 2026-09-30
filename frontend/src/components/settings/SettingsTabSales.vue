<script setup lang="ts">
import { onMounted } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import SettingsTagStatusCard from '@/components/settings/SettingsTagStatusCard.vue'
import SettingsTabPipelines from '@/components/settings/SettingsTabPipelines.vue'
import SettingsTabPrograms from '@/components/settings/SettingsTabPrograms.vue'
import SettingsSectionMenu from '@/components/settings/SettingsSectionMenu.vue'

// Sales tab: the lead workflow's configuration — pipelines and stages,
// programs, and the three quick-reply/task/loss word lists. Without
// settings:manage everything renders read-only. The section menu starts at
// Pipelines on each visit (ADR 015).
defineProps<{ readonly?: boolean }>()

const store = useSettingsStore()

const sections = [
  { value: 'pipelines', label: 'Pipelines' },
  { value: 'programs', label: 'Programs' },
  { value: 'quick-replies', label: 'Quick Replies' },
  { value: 'task-types', label: 'Task types' },
  { value: 'loss-reasons', label: 'Loss Reasons' },
]

onMounted(() => {
  store.fetchTags()
})
</script>

<template>
  <SettingsSectionMenu :sections="sections">
    <template #pipelines>
      <SettingsTabPipelines :readonly="readonly" />
    </template>
    <template #programs>
      <SettingsTabPrograms :readonly="readonly" />
    </template>
    <template #quick-replies>
      <SettingsTagStatusCard kind="quick_reply" title="Quick Replies" placeholder="e.g. No Reply, Busy" :readonly="readonly" />
    </template>
    <template #task-types>
      <SettingsTagStatusCard kind="activity_type" title="Task types" placeholder="e.g. Call, Email" :readonly="readonly" />
    </template>
    <template #loss-reasons>
      <SettingsTagStatusCard kind="loss_reason" title="Loss Reasons" placeholder="e.g. Not interested, Budget" :readonly="readonly" />
    </template>
  </SettingsSectionMenu>
</template>
