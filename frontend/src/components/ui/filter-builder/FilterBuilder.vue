<script setup lang="ts">
import { ref, computed } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Plus, Trash2 } from '@lucide/vue'

export interface FilterField {
  field: string
  label: string
  type: 'select' | 'text' | 'date'
  operators: string[]
  options?: { value: string; label: string }[]
}

export interface ActiveFilter {
  field: string
  operator: string
  value: string | string[]
}

const props = defineProps<{
  fields: FilterField[]
}>()

const emit = defineEmits<{
  apply: [filters: ActiveFilter[]]
  clear: []
}>()

const open = ref(false)
const filters = ref<ActiveFilter[]>([])

function getField(fieldName: string): FilterField | undefined {
  return props.fields.find((f) => f.field === fieldName)
}

function getOperators(fieldName: string): string[] {
  return getField(fieldName)?.operators ?? []
}

function getOptions(fieldName: string) {
  return getField(fieldName)?.options ?? []
}

function getFieldType(fieldName: string): string {
  return getField(fieldName)?.type ?? 'text'
}

function addFilter() {
  const firstField = props.fields[0]
  if (!firstField) return
  const operators = getOperators(firstField.field)
  filters.value.push({
    field: firstField.field,
    operator: operators[0] ?? '=',
    value: '',
  })
}

function removeFilter(index: number) {
  filters.value.splice(index, 1)
}

function onFieldChanged(index: number, fieldName: string) {
  const operators = getOperators(fieldName)
  filters.value[index].field = fieldName
  filters.value[index].operator = operators[0] ?? '='
  filters.value[index].value = ''
}

function apply() {
  const active = filters.value.filter((f) =>
    Array.isArray(f.value) ? f.value.length > 0 : f.value !== ''
  )
  emit('apply', active)
  open.value = false
}

function clear() {
  filters.value = []
  emit('clear')
  open.value = false
}

const slotProps = computed(() => ({
  activeCount: filters.value.filter((f) =>
    Array.isArray(f.value) ? f.value.length > 0 : f.value !== ''
  ).length,
}))
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <slot :active-count="slotProps.activeCount" />
    </PopoverTrigger>
    <PopoverContent class="w-[32rem] p-4">
      <div class="flex flex-col gap-3">
        <div v-for="(f, i) in filters" :key="i" class="flex items-center gap-2">
          <!-- Field -->
          <Select :model-value="f.field" @update:model-value="v => onFieldChanged(i, String(v ?? ''))">
            <SelectTrigger class="h-8 w-36">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="fd in fields" :key="fd.field" :value="fd.field">
                {{ fd.label }}
              </SelectItem>
            </SelectContent>
          </Select>

          <!-- Operator -->
          <Select v-model="f.operator">
            <SelectTrigger class="h-8 w-24">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="op in getOperators(f.field)" :key="op" :value="op">
                {{ op }}
              </SelectItem>
            </SelectContent>
          </Select>

          <!-- Value -->
          <Select
            v-if="getFieldType(f.field) === 'select'"
            v-model="(f.value as string)"
            class="flex-1"
          >
            <SelectTrigger class="h-8">
              <SelectValue placeholder="Select..." />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="opt in getOptions(f.field)" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>
          <Input
            v-else
            v-model="(f.value as string)"
            :type="getFieldType(f.field) === 'date' ? 'date' : 'text'"
            class="h-8 flex-1"
            :placeholder="getFieldType(f.field) === 'date' ? '' : 'Value...'"
          />

          <Button variant="ghost" size="icon" class="h-8 w-8 shrink-0" aria-label="Remove filter" @click="removeFilter(i)">
            <Trash2 class="h-4 w-4 text-muted-foreground" />
          </Button>
        </div>

        <Button variant="outline" size="sm" class="self-start" @click="addFilter">
          <Plus class="mr-1 h-3 w-3" />
          Add filter
        </Button>

        <div class="flex justify-end gap-2 pt-2">
          <Button variant="outline" size="sm" @click="clear">Clear</Button>
          <Button size="sm" @click="apply">Apply</Button>
        </div>
      </div>
    </PopoverContent>
  </Popover>
</template>
