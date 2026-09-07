<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { toast } from 'vue-sonner'
import { getNudgeLeadMinutes, setNudgeLeadMinutes } from '@/api/settings'
import { errorMessage } from '@/utils/errors'
import { Clock } from '@lucide/vue'

const nudgeMinutes = shallowRef(5)
const nudgeLoading = shallowRef(false)

// The nudge lead time: how many minutes before a task's start time its
// reminder fires. One org-wide value, default 5.
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
    toast.success('Reminder lead time saved')
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to save reminder lead time'))
  } finally {
    nudgeLoading.value = false
  }
}

onMounted(() => {
  loadNudge()
})
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle class="text-base flex items-center gap-2">
        <Clock class="size-4 text-muted-foreground" /> Reminders
      </CardTitle>
    </CardHeader>
    <CardContent>
      <div class="flex items-end gap-3">
        <div class="space-y-1.5">
          <Label for="nudge-lead">Remind before tasks start (minutes)</Label>
          <Input id="nudge-lead" v-model.number="nudgeMinutes" type="number" min="0" class="w-32" />
        </div>
        <Button :disabled="nudgeLoading" @click="saveNudge">Save</Button>
      </div>
      <p class="mt-2 text-xs text-muted-foreground">
        Tasks scheduled without an explicit reminder get one this many minutes before the start time.
      </p>
    </CardContent>
  </Card>
</template>