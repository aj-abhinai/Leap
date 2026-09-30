<script setup lang="ts">
import { shallowRef } from 'vue'
import { downloadCsv } from '@/api/export'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Download } from '@lucide/vue'
import { errorMessage } from '@/utils/errors'

// The card renders only inside the audit tab's Export section, which the
// section menu already filters on data:export (ADR 015); the export route
// enforces the permission server-side as well.

const entity = shallowRef<'contacts' | 'leads' | 'both'>('contacts')
const exporting = shallowRef(false)

// The scope picker's emitted value is normalised to the stored union so the
// export path keeps its three explicit cases.
function setEntity(v: unknown) {
  entity.value = v === 'leads' || v === 'both' ? v : 'contacts'
}

function fileName(entity: 'contacts' | 'leads'): string {
  const date = new Date().toISOString().slice(0, 10).replace(/-/g, '')
  return `${entity}-${date}.csv`
}

async function download(entity: 'contacts' | 'leads') {
  const blob = await downloadCsv(entity)
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = fileName(entity)
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

async function runExport() {
  if (exporting.value) return
  exporting.value = true
  try {
    if (entity.value === 'both') {
      await download('contacts')
      await download('leads')
    } else {
      await download(entity.value)
    }
    toast.success('Export started — check your downloads')
  } catch (e) {
    toast.error(errorMessage(e, 'Export failed'))
  } finally {
    exporting.value = false
  }
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle class="text-base">Export</CardTitle>
    </CardHeader>
    <CardContent>
      <p class="text-sm text-muted-foreground">
        CSV export for backup and spreadsheet work — contacts and leads only, no attached data.
      </p>
      <div class="mt-4 flex flex-wrap items-center gap-3">
        <Select
          :model-value="entity"
          :disabled="exporting"
          @update:model-value="setEntity"
        >
          <SelectTrigger class="h-9 w-48">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="contacts">Contacts</SelectItem>
            <SelectItem value="leads">Leads</SelectItem>
            <SelectItem value="both">Contacts &amp; Leads</SelectItem>
          </SelectContent>
        </Select>
        <Button :disabled="exporting" @click="runExport">
          <Download class="mr-2 size-4" />
          {{ exporting ? 'Exporting…' : 'Export CSV' }}
        </Button>
      </div>
    </CardContent>
  </Card>
</template>
