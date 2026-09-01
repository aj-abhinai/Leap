<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import SettingsTagStatusCard from '@/components/settings/SettingsTagStatusCard.vue'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { toast } from 'vue-sonner'
import { getNudgeLeadMinutes, setNudgeLeadMinutes } from '@/api/settings'
import { errorMessage } from '@/utils/errors'

const store = useSettingsStore()

const nudgeMinutes = shallowRef(5)
const nudgeLoading = shallowRef(false)

// The nudge lead time (ADR 004): how many minutes before a task's start time
// its reminder fires. One org-wide value, default 5.
async function loadNudge() {
  try {
    const res = await getNudgeLeadMinutes()
    nudgeMinutes.value = res.data.minutes
  } catch {}
}

async function saveNudge() {
  const v = Number(nudgeMinutes.value)
  if (!Number.isFinite(v) || v < 0) {
    toast.error('Lead time must be a non-negative number of minutes')
    return
  }
  nudgeLoading.value = true
  try {
    await setNudgeLeadMinutes(v)
    toast.success('Nudge lead time saved')
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to save nudge lead time'))
  } finally {
    nudgeLoading.value = false
  }
}

onMounted(() => {
  store.fetchTags()
  loadNudge()
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
    <AccordionItem value="reminders">
      <AccordionTrigger>Reminders</AccordionTrigger>
      <AccordionContent>
        <div class="flex items-end gap-3">
          <div class="space-y-1.5">
            <Label for="nudge-lead">Nudge lead time (minutes before start)</Label>
            <Input id="nudge-lead" v-model.number="nudgeMinutes" type="number" min="0" class="w-32" />
          </div>
          <Button :disabled="nudgeLoading" @click="saveNudge">Save</Button>
        </div>
        <p class="mt-2 text-xs text-muted-foreground">
          Tasks scheduled without an explicit reminder get one this many minutes before the start time.
        </p>
      </AccordionContent>
    </AccordionItem>
  </Accordion>
</template>
