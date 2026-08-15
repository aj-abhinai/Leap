<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import { apiClient } from '@/composables/useApi'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ChevronRight, FolderKanban, Trophy, XCircle } from '@lucide/vue'
import { formatCurrency } from '@/utils/format'
import { formatDate } from '@/utils/time'
import { errorMessage } from '@/utils/errors'

interface LeadInfo {
  id: string
  display_name: string
  pipeline_id: string
  stage_name?: string
  outcome?: string
  lost_reason?: string
  program_name?: string
  value?: number
  assigned_to?: string
  created_at: string
}

const props = defineProps<{
  contactId: string
}>()

const leads = shallowRef<LeadInfo[]>([])
const loading = shallowRef(false)
const loadError = shallowRef('')

// Watch the prop: the component instance is reused when the route param
// changes, so onMounted-only fetching would leave stale leads behind.
let fetchSeq = 0
watch(() => props.contactId, () => fetchLeads(), { immediate: true })

async function fetchLeads() {
  const seq = ++fetchSeq
  loading.value = true
  loadError.value = ''
  try {
    const res = await apiClient.get(`/api/leads?contact_id=${props.contactId}`)
    if (seq !== fetchSeq) return
    leads.value = res.data
  } catch (e) {
    if (seq !== fetchSeq) return
    leads.value = []
    loadError.value = errorMessage(e, 'Failed to load leads')
  } finally {
    if (seq === fetchSeq) loading.value = false
  }
}
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold">Lead Journey</h3>
      <Badge variant="secondary" class="text-xs">{{ leads.length }} leads</Badge>
    </div>

    <div v-if="loading" class="space-y-2">
      <Skeleton v-for="i in 3" :key="i" class="h-16 w-full" />
    </div>

    <div v-else-if="loadError" class="rounded-lg border border-dashed p-6 text-center">
      <p class="text-sm text-destructive">{{ loadError }}</p>
    </div>

    <div v-else-if="leads.length === 0" class="rounded-lg border border-dashed p-6 text-center">
      <FolderKanban class="size-8 text-muted-foreground/40 mx-auto mb-2" />
      <p class="text-sm text-muted-foreground">No leads for this contact</p>
    </div>

    <div v-else class="space-y-2">
      <Card v-for="lead in leads" :key="lead.id" class="hover:shadow-sm transition-shadow">
        <CardContent class="p-3">
          <div class="flex items-start justify-between">
            <div class="min-w-0">
              <div class="font-medium text-sm truncate">{{ lead.display_name }}</div>
            <div class="flex items-center gap-1.5 mt-1 text-xs text-muted-foreground">
              <span>Pipeline</span>
              <ChevronRight class="size-3" />
              <Badge variant="outline" class="text-xs px-1.5">{{ lead.stage_name || '–' }}</Badge>
              <span v-if="lead.outcome === 'won'" class="inline-flex items-center gap-1 text-success font-medium">
                <Trophy class="size-3" /> Won
              </span>
              <span v-else-if="lead.outcome === 'lost'" class="inline-flex items-center gap-1 text-destructive font-medium">
                <XCircle class="size-3" /> Lost
              </span>
            </div>
            <div v-if="lead.outcome === 'lost' && lead.lost_reason" class="mt-1 text-xs text-muted-foreground">
              Reason: {{ lead.lost_reason }}
            </div>
            <div v-if="lead.program_name" class="mt-1 text-xs text-muted-foreground">
              {{ lead.program_name }}
            </div>
            </div>
            <div v-if="lead.value" class="text-sm font-semibold text-primary">
              {{ formatCurrency(lead.value) }}
            </div>
          </div>
          <div class="mt-1 text-xs text-muted-foreground">
            Created {{ formatDate(lead.created_at) }}
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
