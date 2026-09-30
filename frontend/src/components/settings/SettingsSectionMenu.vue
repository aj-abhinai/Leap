<script setup lang="ts">
import { shallowRef } from 'vue'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

// SettingsSectionMenu is the in-tab section row shared by the tabs whose
// sections are long enough to hide (Contacts, Sales, and Audit log; amended
// ADR 015): a row of small pills over one content panel. Callers pass only
// the sections the current user can access, so the first entry is what opens
// on each visit — the selection is deliberately not persisted (ADR 015).
interface Section {
  value: string
  label: string
}

const props = defineProps<{ sections: Section[] }>()

const active = shallowRef(props.sections[0]?.value ?? '')
</script>

<template>
  <Tabs v-model="active" class="w-full">
    <TabsList class="mb-4 h-auto w-full justify-start gap-1 overflow-x-auto rounded-lg bg-transparent p-0">
      <TabsTrigger
        v-for="section in sections"
        :key="section.value"
        :value="section.value"
        class="flex-none rounded-full border-transparent px-2.5 py-1 text-xs font-normal text-muted-foreground hover:text-foreground data-[state=active]:border-primary data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
      >
        {{ section.label }}
      </TabsTrigger>
    </TabsList>
    <TabsContent v-for="section in sections" :key="section.value" :value="section.value" class="mt-0">
      <slot :name="section.value" />
    </TabsContent>
  </Tabs>
</template>
