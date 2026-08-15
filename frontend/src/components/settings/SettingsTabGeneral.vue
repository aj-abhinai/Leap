<script setup lang="ts">
import { onMounted } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import SettingsTagStatusCard from '@/components/settings/SettingsTagStatusCard.vue'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'

const store = useSettingsStore()

onMounted(() => {
  store.fetchTags()
})
</script>

<template>
  <Accordion type="multiple" :default-value="['tags']" class="w-full">
    <AccordionItem value="tags">
      <AccordionTrigger>Tags</AccordionTrigger>
      <AccordionContent>
        <SettingsTagStatusCard kind="tag" title="Tags" placeholder="Tag name" />
      </AccordionContent>
    </AccordionItem>
    <AccordionItem value="statuses">
      <AccordionTrigger>Statuses</AccordionTrigger>
      <AccordionContent>
        <SettingsTagStatusCard kind="status" title="Statuses" placeholder="Status name" />
      </AccordionContent>
    </AccordionItem>
    <AccordionItem value="quick-replies">
      <AccordionTrigger>Quick Replies</AccordionTrigger>
      <AccordionContent>
        <SettingsTagStatusCard kind="quick_reply" title="Quick Replies" placeholder="e.g. No Reply, Busy" />
      </AccordionContent>
    </AccordionItem>
    <AccordionItem value="activity-types">
      <AccordionTrigger>Activity Types</AccordionTrigger>
      <AccordionContent>
        <SettingsTagStatusCard kind="activity_type" title="Activity Types" placeholder="e.g. Call 3, Email" />
      </AccordionContent>
    </AccordionItem>
    <AccordionItem value="loss-reasons">
      <AccordionTrigger>Loss Reasons</AccordionTrigger>
      <AccordionContent>
        <SettingsTagStatusCard kind="loss_reason" title="Loss Reasons" placeholder="e.g. Not interested, Budget" />
      </AccordionContent>
    </AccordionItem>
  </Accordion>
</template>
