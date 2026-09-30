// Remembered create values: the program, assignee, and stage a user submitted
// last. They are personal view defaults stored client-side (localStorage),
// keyed per user so a shared browser never carries one person's last pick into
// another's session. Nothing here is authoritative — every value is validated
// against the live pickers before it is used.

// RememberedCreateValues are the values shared across pipelines: a program id
// and an assignee user id, each possibly empty (no program / unassigned).
export interface RememberedCreateValues {
  programId: string
  assignedTo: string
}

const KEY_ROOT = 'crm:leads:form'

// storageKey namespaces keys by user; a session without a loaded user falls
// back to a shared key rather than throwing.
function storageKey(userId: string | undefined, name: string): string {
  return `${KEY_ROOT}:${userId || 'anon'}:${name}`
}

// read returns the stored string; unreadable storage reads as empty.
function read(userId: string | undefined, name: string): string {
  try {
    return localStorage.getItem(storageKey(userId, name)) ?? ''
  } catch {
    return ''
  }
}

// write stores a value. Storage failures (private mode, quota) are ignored: a
// preference must never block lead entry.
function write(userId: string | undefined, name: string, value: string): void {
  try {
    localStorage.setItem(storageKey(userId, name), value)
  } catch {
    // Best-effort preference.
  }
}

// StageMemory maps a pipeline id to its remembered stage id, so a stage is
// never carried from one pipeline to another.
interface StageMemory {
  [pipelineId: string]: string
}

function readStageMemory(userId: string | undefined): StageMemory {
  const raw = read(userId, 'stages')
  if (!raw) return {}
  try {
    const parsed: unknown = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? (parsed as StageMemory) : {}
  } catch {
    return {}
  }
}

// loadRememberedCreateValues returns the last submitted program and assignee
// for the user. Missing entries and unreadable storage yield empty strings.
export function loadRememberedCreateValues(userId: string | undefined): RememberedCreateValues {
  return {
    programId: read(userId, 'program'),
    assignedTo: read(userId, 'assignee'),
  }
}

// rememberCreateValues stores the values a create submitted: the program and
// assignee are shared across pipelines, the stage is remembered per pipeline.
export function rememberCreateValues(
  userId: string | undefined,
  pipelineId: string,
  values: { programId: string; assignedTo: string; stageId: string },
): void {
  write(userId, 'program', values.programId)
  write(userId, 'assignee', values.assignedTo)
  const stages = readStageMemory(userId)
  stages[pipelineId] = values.stageId
  write(userId, 'stages', JSON.stringify(stages))
}

// rememberedStageId returns the stage remembered for a pipeline when it is
// still an open stage of that pipeline, else ''. A stale or closing stage is
// never returned: creates cannot start in a closing stage.
export function rememberedStageId(
  userId: string | undefined,
  pipelineId: string,
  stages: ReadonlyArray<{ id: string; is_closing?: boolean }>,
): string {
  const stageId = readStageMemory(userId)[pipelineId] ?? ''
  const stage = stages.find((s) => s.id === stageId)
  return stage && !stage.is_closing ? stage.id : ''
}
