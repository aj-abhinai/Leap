<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { toast } from 'vue-sonner'
import { getNudgeLeadMinutes, setNudgeLeadMinutes, getDefaultCountryCode, setDefaultCountryCode } from '@/api/settings'
import { errorMessage } from '@/utils/errors'
import { Clock, Phone } from '@lucide/vue'

const nudgeMinutes = shallowRef(5)
const nudgeLoading = shallowRef(false)

const countryCode = shallowRef('+91')
const countryCodeLoading = shallowRef(false)

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

// The org's default country code: bare phone numbers get it when staff do not
// type an international form. One org-wide value, default +91.
async function loadCountryCode() {
  try {
    const res = await getDefaultCountryCode()
    countryCode.value = res.data.country_code
  } catch {}
}

async function saveCountryCode() {
  const v = countryCode.value.trim()
  if (!/^\+\d{1,3}$/.test(v)) {
    toast.error('Country code must look like +91 or +971')
    return
  }
  countryCodeLoading.value = true
  try {
    await setDefaultCountryCode(v)
    countryCode.value = v
    toast.success('Default country code saved')
  } catch (e) {
    toast.error(errorMessage(e, 'Failed to save default country code'))
  } finally {
    countryCodeLoading.value = false
  }
}

onMounted(() => {
  loadNudge()
  loadCountryCode()
})
</script>

<template>
  <div class="space-y-4">
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
    <Card>
      <CardHeader>
        <CardTitle class="text-base flex items-center gap-2">
          <Phone class="size-4 text-muted-foreground" /> Default country code
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex items-end gap-3">
          <div class="space-y-1.5">
            <Label for="country-code">Country code</Label>
            <Input id="country-code" v-model="countryCode" placeholder="+91" class="w-32" />
          </div>
          <Button :disabled="countryCodeLoading" @click="saveCountryCode">Save</Button>
        </div>
        <p class="mt-2 text-xs text-muted-foreground">
          Phone numbers typed without an international prefix get this country code. Use the form +91 or +971.
        </p>
      </CardContent>
    </Card>
  </div>
</template>