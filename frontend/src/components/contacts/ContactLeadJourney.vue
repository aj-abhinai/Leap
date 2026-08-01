<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { apiClient } from '@/composables/useApi'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ChevronRight, FolderKanban } from '@lucide/vue'
import { formatCurrency } from '@/utils/format'
import { formatDate } from '@/utils/time'

interface LeadInfo {
  id: string
  name: string
  pipeline_id: string
  stage_name?: string
  value?: number
  assigned_to?: string
  created_at: string
}

const props = defineProps<{
  contactId: string
}>()

const leads = shallowRef<LeadInfo[]>([])
const loading = shallowRef(false)

onMounted(() => fetchLeads())

async function fetchLeads() {
  loading.value = true
  try {
    const res = await apiClient.get(`/api/leads?contact_id=${props.contactId}`)
    leads.value = res.data
  } catch {
    leads.value = []
  } finally {
    loading.value = false
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

    <div v-else-if="leads.length === 0" class="rounded-lg border border-dashed p-6 text-center">
      <FolderKanban class="size-8 text-muted-foreground/40 mx-auto mb-2" />
      <p class="text-sm text-muted-foreground">No leads for this contact</p>
    </div>

    <div v-else class="space-y-2">
      <Card v-for="lead in leads" :key="lead.id" class="hover:shadow-sm transition-shadow">
        <CardContent class="p-3">
          <div class="flex items-start justify-between">
            <div class="min-w-0">
              <div class="font-medium text-sm truncate">{{ lead.name }}</div>
              <div class="flex items-center gap-1.5 mt-1 text-xs text-muted-foreground">
                <span>Pipeline</span>
                <ChevronRight class="size-3" />
                <Badge variant="outline" class="text-xs px-1.5">{{ lead.stage_name || '—' }}</Badge>
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
