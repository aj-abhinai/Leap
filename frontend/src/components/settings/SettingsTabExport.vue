<script setup lang="ts">
import { shallowRef } from 'vue'
import { useRBACStore } from '@/stores/rbac'
import { downloadCsv } from '@/api/export'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Download } from '@lucide/vue'
import { errorMessage } from '@/utils/errors'

const rbac = useRBACStore()

const entity = shallowRef<'contacts' | 'leads' | 'both'>('contacts')
const exporting = shallowRef(false)

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
        <select
          v-model="entity"
          class="h-9 w-48 rounded-md border bg-background px-2 text-sm"
          :disabled="exporting"
        >
          <option value="contacts">Contacts</option>
          <option value="leads">Leads</option>
          <option value="both">Contacts &amp; Leads</option>
        </select>
        <Button :disabled="exporting || !rbac.can('data:export')" @click="runExport">
          <Download class="mr-2 size-4" />
          {{ exporting ? 'Exporting…' : 'Export CSV' }}
        </Button>
      </div>
      <p v-if="!rbac.can('data:export')" class="mt-3 text-sm text-muted-foreground">
        You need the <span class="font-medium">data:export</span> permission to export data.
      </p>
    </CardContent>
  </Card>
</template>
