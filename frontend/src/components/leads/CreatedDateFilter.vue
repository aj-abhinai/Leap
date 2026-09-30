<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { DateRange, DateValue } from 'reka-ui'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { RangeCalendar } from '@/components/ui/range-calendar'
import { Button } from '@/components/ui/button'
import { Calendar, Check, ChevronDown } from '@lucide/vue'
import type { DatePreset } from '@/utils/time'

// CreatedDateFilter is the leads board's created-at control: one dropdown
// holding the quick presets and a dual-month range calendar. Presets apply on
// click; a range applies when its second day is picked.
const props = defineProps<{
  preset: DatePreset
  from: string
  to: string
}>()

const emit = defineEmits<{
  'select-preset': [preset: DatePreset]
  'select-range': [from: string, to: string]
}>()

const options: ReadonlyArray<{ value: DatePreset; label: string }> = [
  { value: 'all', label: 'All dates' },
  { value: 'today', label: 'Today' },
  { value: 'yesterday', label: 'Yesterday' },
  { value: 'week', label: 'This week' },
  { value: 'month', label: 'This month' },
]

const open = shallowRef(false)
// range is the calendar's selection: it keeps a completed range visible while
// the popover stays open, and is dropped when a preset replaces it.
const range = shallowRef<DateRange | null>(null)

const label = computed(() => {
  const option = options.find((o) => o.value === props.preset)
  if (option) return option.label
  const from = dayLabel(props.from)
  const to = dayLabel(props.to)
  if (!from) return 'Custom range'
  return !to || from === to ? from : `${from} – ${to}`
})

// dayLabel renders a stored YYYY-MM-DD day in the viewer's locale, parsed as a
// local day so the label never shifts across time zones.
function dayLabel(day: string): string {
  const parts = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day)
  if (!parts) return ''
  return new Date(Number(parts[1]), Number(parts[2]) - 1, Number(parts[3])).toLocaleDateString(
    undefined,
    { month: 'short', day: 'numeric' },
  )
}

// dayValue formats a picked calendar day as the stored YYYY-MM-DD.
function dayValue(day: DateValue): string {
  const month = String(day.month).padStart(2, '0')
  const date = String(day.day).padStart(2, '0')
  return `${day.year}-${month}-${date}`
}

// closePopover drops an uncommitted start day, so reopening never resumes a
// half-picked range; a completed range stays visible.
function closePopover() {
  open.value = false
  if (!range.value?.end) range.value = null
}

function handleOpenChange(next: boolean) {
  if (next) {
    open.value = true
    return
  }
  closePopover()
}

function selectPreset(preset: DatePreset) {
  range.value = null
  emit('select-preset', preset)
  open.value = false
}

// A range applies when both ends are picked; the first click only marks the
// start and keeps the popover open.
function onRangeUpdate(next: DateRange) {
  range.value = next
  if (!next.start || !next.end) return
  emit('select-range', dayValue(next.start), dayValue(next.end))
  open.value = false
}

// A preset outside 'custom' replaces whatever range was shown.
watch(
  () => props.preset,
  (preset) => {
    if (preset !== 'custom') range.value = null
  },
)
</script>

<template>
  <Popover :open="open" @update:open="handleOpenChange">
    <PopoverTrigger as-child>
      <Button
        variant="outline"
        class="h-9 w-36 justify-between gap-2 font-normal"
        aria-label="Created date"
      >
        <span class="flex min-w-0 items-center gap-1.5">
          <Calendar class="size-3.5 shrink-0 text-muted-foreground" />
          <span class="truncate">{{ label }}</span>
        </span>
        <ChevronDown class="size-3.5 shrink-0 text-muted-foreground" />
      </Button>
    </PopoverTrigger>
    <PopoverContent align="start" class="w-auto p-0">
      <div class="flex flex-col sm:flex-row">
        <div class="flex w-36 shrink-0 flex-col gap-0.5 border-b p-2 sm:border-r sm:border-b-0">
          <button
            v-for="option in options"
            :key="option.value"
            type="button"
            class="flex items-center justify-between rounded-sm px-2 py-1.5 text-sm hover:bg-accent"
            :class="{ 'font-medium': preset === option.value }"
            @click="selectPreset(option.value)"
          >
            {{ option.label }}
            <Check v-if="preset === option.value" class="size-3.5" />
          </button>
        </div>
        <RangeCalendar
          :model-value="range"
          :number-of-months="2"
          :week-starts-on="0"
          @update:model-value="onRangeUpdate"
        />
      </div>
    </PopoverContent>
  </Popover>
</template>
