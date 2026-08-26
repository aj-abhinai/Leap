<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { listLeadHistory, type StageHistoryEntry } from '@/api/leads'
import { Skeleton } from '@/components/ui/skeleton'
import { Route } from '@lucide/vue'
import { formatDateTime } from '@/utils/time'
import { errorMessage } from '@/utils/errors'

const props = defineProps<{ leadId: string }>()

const history = shallowRef<StageHistoryEntry[]>([])
const loading = shallowRef(false)
const loadError = shallowRef('')

onMounted(fetchHistory)

async function fetchHistory() {
  loading.value = true
  loadError.value = ''
  try {
    const res = await listLeadHistory(props.leadId)
    history.value = res.data ?? []
  } catch (e) {
    loadError.value = errorMessage(e, 'Failed to load stage history')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section>
    <h3 class="mb-2 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
      <Route class="size-3.5" /> Stage history
    </h3>
    <div v-if="loading" class="space-y-2">
      <Skeleton v-for="i in 2" :key="i" class="h-10 w-full" />
    </div>
    <div v-else-if="loadError" class="rounded-md border border-dashed p-3 text-xs text-destructive">
      {{ loadError }}
    </div>
    <div v-else-if="history.length === 0" class="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
      No stage moves recorded
    </div>
    <ol v-else class="space-y-0 border-l border-muted pl-3">
      <li v-for="entry in history" :key="entry.id" class="relative pb-2 last:pb-0">
        <span class="absolute -left-[17px] top-1 size-2 rounded-full bg-primary/60" />
        <p class="text-xs">
          <template v-if="entry.from_stage_name && entry.to_stage_name">
            <span class="text-muted-foreground line-through decoration-muted-foreground/40">{{ entry.from_stage_name }}</span>
            <span class="mx-1 text-muted-foreground">→</span>
            <span class="font-medium">{{ entry.to_stage_name }}</span>
          </template>
          <template v-else>
            <span class="font-medium">{{ entry.to_stage_name || entry.from_stage_name || 'Moved' }}</span>
          </template>
        </p>
        <p class="text-[11px] text-muted-foreground/70">{{ formatDateTime(entry.moved_at) }}</p>
      </li>
    </ol>
  </section>
</template>
