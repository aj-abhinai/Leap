<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { bulkCreateLeads, type BulkLeadCreateResult } from '@/api/leads'
import { listPrograms, type Program } from '@/api/programs'
import { usePipelineStore } from '@/stores/pipeline'
import { useUsersStore } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import {
  loadRememberedCreateValues,
  rememberCreateValues,
  rememberedStageId,
} from '@/composables/useLeadFormDefaults'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { errorMessage } from '@/utils/errors'
import { formatCurrency } from '@/utils/format'

const props = defineProps<{
  open: boolean
  contactIds: string[]
}>()

const emit = defineEmits<{
  close: []
  done: []
}>()

const UNASSIGNED = '__unassigned__'
const NO_PROGRAM = '__none__'

const pipelineStore = usePipelineStore()
const users = useUsersStore()
const auth = useAuthStore()

const step = shallowRef<'form' | 'result'>('form')
const formPipelineId = shallowRef('')
const formStageId = shallowRef('')
const formProgramId = shallowRef(NO_PROGRAM)
const formAssignedTo = shallowRef(UNASSIGNED)
const programs = shallowRef<Program[]>([])
const submitting = shallowRef(false)
const error = shallowRef('')
const result = shallowRef<BulkLeadCreateResult | null>(null)

const stages = computed(() => pipelineStore.pipelines.find((p) => p.id === formPipelineId.value)?.stages ?? [])
// Closing stages are unreachable at create: only open stages are offered.
const createStages = computed(() => stages.value.filter((s) => !s.is_closing))

const selectedProgram = computed(() => programs.value.find((p) => p.id === formProgramId.value))

// defaultStageId is the first open stage of a pipeline, mirroring the create
// form's starting stage.
function defaultStageId(pipelineId: string): string {
  return pipelineStore.pipelines.find((p) => p.id === pipelineId)?.stages?.find((s) => !s.is_closing)?.id ?? ''
}

function stagesOf(pipelineId: string) {
  return pipelineStore.pipelines.find((p) => p.id === pipelineId)?.stages ?? []
}

// seedStage seeds the pipeline's remembered stage when it is still an open
// stage of that pipeline, else the pipeline's first open stage.
function seedStage(pipelineId: string) {
  formStageId.value =
    rememberedStageId(auth.user?.id, pipelineId, stagesOf(pipelineId)) || defaultStageId(pipelineId)
}

// An assignee that is no longer listed (a deleted or deactivated user) must
// not reach the create, or the run is refused (ErrInvalidAssignee). Only a
// successful fetch proves the option list; a failed one must not unassign.
function ensureValidAssignee() {
  if (users.error) return
  if (formAssignedTo.value !== UNASSIGNED && !users.options.some((u) => u.id === formAssignedTo.value)) {
    formAssignedTo.value = UNASSIGNED
  }
}
watch(() => users.options, ensureValidAssignee)

// loadOptions fills the pickers and seeds the shared fields from the
// remembered create values: the program and assignee, and the selected
// pipeline's remembered stage, else its first open stage. Seeding happens
// before the catalog refetch, so a late response can never overwrite a field
// the operator already changed. The pipeline keeps its own first-pipeline
// default — the memory never chooses a pipeline.
async function loadOptions() {
  const assignees = users.fetchOptions()
  if (pipelineStore.pipelines.length === 0) {
    try {
      await pipelineStore.fetchPipelines()
    } catch (e) {
      // Pipelines are required for this dialog: without them the form is
      // unusable, so the failure is surfaced instead of leaving a silently
      // disabled create button. Reopening the dialog retries.
      error.value = errorMessage(e, 'Failed to load pipelines')
    }
  }
  if (!formPipelineId.value && pipelineStore.pipelines.length > 0) {
    formPipelineId.value = pipelineStore.pipelines[0].id
  }
  if (formPipelineId.value) seedStage(formPipelineId.value)

  const remembered = loadRememberedCreateValues(auth.user?.id)
  formProgramId.value = remembered.programId || NO_PROGRAM
  formAssignedTo.value = remembered.assignedTo || UNASSIGNED

  try {
    // Refetched on every open: a remembered program is only usable while the
    // live catalog still lists it (programs are archived, not deleted).
    const res = await listPrograms()
    programs.value = res.data ?? []
    // Drop the remembered program when the live catalog no longer lists it and
    // the operator has not replaced it since.
    if (formProgramId.value === remembered.programId && remembered.programId
      && !programs.value.some((p) => p.id === remembered.programId)) {
      formProgramId.value = NO_PROGRAM
    }
  } catch {
    // The program picker degrades to "No program"; a failed catalog fetch
    // must not block a create that does not need a program.
  }
  await assignees
  // Options another view already cached never re-emit, so the watcher above
  // cannot validate them; do it once the list is known to have settled.
  if (users.options.length > 0 && !users.loading) ensureValidAssignee()
}

// A pipeline switch reseeds the stage from that pipeline's memory.
watch(formPipelineId, (id, previous) => {
  if (id !== previous) seedStage(id)
})

watch(
  () => props.open,
  (open) => {
    if (!open) return
    step.value = 'form'
    result.value = null
    error.value = ''
    // Fire-and-forget: seeding happens inside loadOptions, and a pending
    // catalog must never gate the dialog's Cancel.
    loadOptions().catch(() => {})
  },
  { immediate: true },
)

async function submit() {
  if (!formPipelineId.value || !formStageId.value || props.contactIds.length === 0) return
  submitting.value = true
  error.value = ''
  try {
    // Remember what this run submits, so the next lead entry — here or on the
    // single-lead form — starts from it.
    rememberCreateValues(auth.user?.id, formPipelineId.value, {
      programId: formProgramId.value === NO_PROGRAM ? '' : formProgramId.value,
      assignedTo: formAssignedTo.value === UNASSIGNED ? '' : formAssignedTo.value,
      stageId: formStageId.value,
    })
    const res = await bulkCreateLeads({
      contact_ids: props.contactIds,
      pipeline_id: formPipelineId.value,
      stage_id: formStageId.value,
      program_id: formProgramId.value === NO_PROGRAM ? '' : formProgramId.value,
      assigned_to: formAssignedTo.value === UNASSIGNED ? '' : formAssignedTo.value,
    })
    result.value = res.data
    step.value = 'result'
    if (res.data.created > 0) {
      toast.success(`Created ${res.data.created} leads`)
    }
  } catch (e) {
    // A shared-field refusal (4xx) writes nothing; keep the form open so the
    // operator can correct the choice.
    error.value = errorMessage(e, 'Failed to create leads')
  } finally {
    submitting.value = false
  }
}

// close is refused while a run is in flight: the request is already sent,
// and its report is the outcome the operator must read before doing
// anything else.
function close() {
  if (submitting.value) return
  emit('close')
}

function finish() {
  emit('done')
}
</script>

<template>
  <Dialog :open="open" @update:open="(val: boolean) => { if (!val) close() }">
    <DialogContent class="sm:max-w-lg max-h-[90vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>Create leads for {{ contactIds.length }} contacts</DialogTitle>
        <DialogDescription>
          One lead per contact, all in the same pipeline, stage, program, and assignee.
        </DialogDescription>
      </DialogHeader>

      <div v-if="step === 'form'" class="space-y-4">
        <div class="space-y-2">
          <Label for="bulk-pipeline">Pipeline</Label>
          <Select v-model="formPipelineId">
            <SelectTrigger id="bulk-pipeline">
              <SelectValue placeholder="Select pipeline" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="p in pipelineStore.pipelines" :key="p.id" :value="p.id">
                {{ p.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-2">
          <Label for="bulk-stage">Stage</Label>
          <Select v-model="formStageId">
            <SelectTrigger id="bulk-stage">
              <SelectValue placeholder="Select stage" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="s in createStages" :key="s.id" :value="s.id">
                {{ s.name }}
              </SelectItem>
            </SelectContent>
          </Select>
          <p v-if="createStages.length < stages.length" class="text-xs text-muted-foreground">
            Closing stages are only reachable by moving an existing lead
          </p>
        </div>
        <div class="space-y-2">
          <Label for="bulk-program">Program</Label>
          <Select v-model="formProgramId">
            <SelectTrigger id="bulk-program">
              <SelectValue placeholder="Select program" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem :value="NO_PROGRAM">No program</SelectItem>
              <SelectItem v-for="p in programs" :key="p.id" :value="p.id">
                {{ p.name }} · {{ formatCurrency(p.price) }}
              </SelectItem>
            </SelectContent>
          </Select>
          <p class="text-xs text-muted-foreground">
            Value snapshot: {{ formatCurrency(selectedProgram?.price) || '–' }}
          </p>
        </div>
        <div class="space-y-2">
          <Label>Assignee</Label>
          <Select v-model="formAssignedTo">
            <SelectTrigger aria-label="Assignee">
              <SelectValue placeholder="Unassigned" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem :value="UNASSIGNED">Unassigned</SelectItem>
              <SelectItem v-for="u in users.options" :key="u.id" :value="u.id">
                {{ u.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
        <div class="flex justify-end gap-2">
          <Button variant="outline" :disabled="submitting" @click="close">Cancel</Button>
          <Button :disabled="submitting || !formStageId || contactIds.length === 0" @click="submit">
            {{ submitting ? 'Creating...' : `Create ${contactIds.length} leads` }}
          </Button>
        </div>
      </div>

      <div v-else class="space-y-4">
        <div class="flex items-center gap-4">
          <div class="text-center">
            <div class="text-2xl font-bold text-chart-2">{{ result?.created || 0 }}</div>
            <div class="text-xs text-muted-foreground">Created</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-muted-foreground">{{ result?.skipped || 0 }}</div>
            <div class="text-xs text-muted-foreground">Skipped</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-destructive">{{ result?.failed || 0 }}</div>
            <div class="text-xs text-muted-foreground">Failed</div>
          </div>
        </div>
        <div v-if="result?.errors?.length" class="text-sm space-y-1 max-h-40 overflow-y-auto">
          <p v-for="e in result.errors" :key="e.contact_id">
            <span :class="e.outcome === 'skipped' ? 'text-muted-foreground' : 'text-destructive'">
              {{ e.outcome === 'skipped' ? 'Skipped' : 'Failed' }}:
            </span>
            {{ e.name }} — {{ e.message }}
          </p>
        </div>
        <p v-else class="text-sm text-muted-foreground">Every contact got a lead.</p>
        <div class="flex justify-end">
          <Button @click="finish">Done</Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
