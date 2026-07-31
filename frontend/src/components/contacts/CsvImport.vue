<script setup lang="ts">
import { ref } from 'vue'
import { apiClient } from '@/composables/useApi'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Download, Upload } from '@lucide/vue'

defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const step = ref<'upload' | 'preview' | 'result'>('upload')
const fileInput = ref<HTMLInputElement | null>(null)
const error = ref('')
const headers = ref<string[]>([])
const rows = ref<string[][]>([])
const preview = ref<string[][]>([])
const importing = ref(false)
const result = ref<{ imported: number; failed: number; errors?: { row: number; message: string }[] } | null>(null)

function parseCSV(text: string): { headers: string[]; rows: string[][] } {
  const clean = text.replace(/^\uFEFF/, '')
  const lines = clean.trim().split('\n').map(l => l.trim()).filter(l => l)
  if (lines.length === 0) return { headers: [], rows: [] }
  const h = lines[0].split(',').map(hh => hh.trim().toLowerCase())
  const r: string[][] = []
  for (let i = 1; i < lines.length; i++) {
    const line = lines[i]
    const result: string[] = []
    let current = ''
    let inQuotes = false
    for (const ch of line) {
      if (ch === '"') { inQuotes = !inQuotes }
      else if (ch === ',' && !inQuotes) { result.push(current.trim()); current = '' }
      else { current += ch }
    }
    result.push(current.trim())
    r.push(result)
  }
  return { headers: h, rows: r }
}

function onFileSelect(e: Event) {
  error.value = ''
  const target = e.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return
  if (!file.name.endsWith('.csv')) {
    error.value = 'Please select a CSV file'
    return
  }
  const reader = new FileReader()
  reader.onload = (ev) => {
    const text = ev.target?.result as string
    const { headers: h, rows: r } = parseCSV(text)
    if (h.length === 0 || !h.includes('name')) {
      error.value = 'CSV must have a "name" column'
      return
    }
    headers.value = h
    rows.value = r
    preview.value = r.slice(0, 5)
    if (r.length > 500) {
      error.value = 'Maximum 500 contacts per import'
      return
    }
    step.value = 'preview'
  }
  reader.readAsText(file)
}

function triggerUpload() {
  fileInput.value?.click()
}

async function doImport() {
  importing.value = true
  try {
    const contacts = rows.value.map(row => {
      const contact: Record<string, any> = {}
      headers.value.forEach((h, i) => {
        contact[h] = row[i] || ''
      })
      const tags = contact.tags ? contact.tags.split(',').map((t: string) => t.trim()).filter(Boolean) : []
      return {
        name: contact.name,
        email: contact.email || undefined,
        phone: contact.phone || undefined,
        location: contact.location || undefined,
        tags,
      }
    })
    const res = await apiClient.post('/api/contacts/bulk', { contacts })
    result.value = res.data
    step.value = 'result'
    if (res.data.imported > 0) {
      toast.success(`Imported ${res.data.imported} contacts`)
    }
  } catch (e: any) {
    error.value = e.message || 'Import failed'
  } finally {
    importing.value = false
  }
}

function reset() {
  step.value = 'upload'
  error.value = ''
  headers.value = []
  rows.value = []
  preview.value = []
  result.value = null
}

function close() {
  emit('close')
  reset()
}
</script>

<template>
  <Dialog :open="open" @update:open="(val) => !val && close()">
    <DialogContent class="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>Import Contacts from CSV</DialogTitle>
        <DialogDescription>
          Download the template, fill your data, and upload.
        </DialogDescription>
      </DialogHeader>

      <div v-if="step === 'upload'" class="space-y-4">
        <Button variant="outline" as-child>
          <a href="/contacts-template.csv" download>
            <Download class="size-4 mr-2" /> Download Template
          </a>
        </Button>
        <div
          class="border-2 border-dashed rounded-lg p-8 text-center cursor-pointer hover:border-primary/50 transition-colors"
          @click="triggerUpload"
          @dragover.prevent
          @drop.prevent="(e) => { const files = e.dataTransfer?.files; if (files?.length) { const dt = new DataTransfer(); dt.items.add(files[0]); if (fileInput) { fileInput.files = dt.files; onFileSelect({ target: fileInput } as any) } } }"
        >
          <input ref="fileInput" type="file" accept=".csv" class="hidden" @change="onFileSelect" />
          <Upload class="size-8 text-muted-foreground/40 mx-auto mb-2" />
          <p class="text-sm font-medium">Click to browse or drag & drop</p>
          <p class="text-xs text-muted-foreground mt-1">CSV files only</p>
        </div>
        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      </div>

      <div v-else-if="step === 'preview'" class="space-y-4">
        <p class="text-sm text-muted-foreground">Previewing first {{ preview.length }} of {{ rows.length }} rows</p>
        <div class="rounded-lg border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead v-for="h in headers" :key="h" class="capitalize">{{ h }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="(row, i) in preview" :key="i">
                <TableCell v-for="(cell, j) in row" :key="j" class="whitespace-nowrap">{{ cell }}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
        <div class="flex justify-end gap-2">
          <Button variant="outline" @click="step = 'upload'; error = ''">Back</Button>
          <Button :disabled="importing" @click="doImport">
            {{ importing ? 'Importing...' : `Import ${rows.length} contacts` }}
          </Button>
        </div>
      </div>

      <div v-else-if="step === 'result'" class="space-y-4">
        <div class="flex items-center gap-4">
          <div class="text-center">
            <div class="text-2xl font-bold text-emerald-600">{{ result?.imported || 0 }}</div>
            <div class="text-xs text-muted-foreground">Imported</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-destructive">{{ result?.failed || 0 }}</div>
            <div class="text-xs text-muted-foreground">Failed</div>
          </div>
        </div>
        <div v-if="result?.errors?.length" class="text-sm text-destructive space-y-1 max-h-40 overflow-y-auto">
          <p v-for="err in result.errors" :key="err.row">Row {{ err.row }}: {{ err.message }}</p>
        </div>
        <div class="flex justify-end">
          <Button @click="close()">Done</Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
