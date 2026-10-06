import type { Component } from 'vue'
import { CalendarClock, CheckCheck, Mail, MessageCircle, NotepadText, Phone } from '@lucide/vue'
import type { ActivityFollowUpBody, ActivityMutationBody } from '@/api/leads'
import {
  allDayRange,
  formatDate,
  formatDateTime,
  isAllDayRange,
  mergeDateTime,
  toLocalDateInput,
  toLocalTimeInput,
} from '@/utils/time'

// Task semantics: one home for how a task row is read, displayed, and turned
// into a payload. The module is pure — no stores, no transport, no clock of
// its own. Callers pass a TaskEnv (clock + org defaults) and a plain row.

// --- environment -----------------------------------------------------------

export type Instant = string
export type Day = string

export interface Clock {
  now(): Date
}

export interface OrgDefaults {
  nudgeLeadMinutes: number
}

export interface TaskEnv {
  clock: Clock
  defaults: OrgDefaults
}

// localEnv builds the production env: the real clock and the org nudge lead
// fallback that mirrors the server default.
export function localEnv(defaults: Partial<OrgDefaults> = {}): TaskEnv {
  return {
    clock: { now: () => new Date() },
    defaults: { nudgeLeadMinutes: 5, ...defaults },
  }
}

// --- model -----------------------------------------------------------------

export interface TaskSource {
  type?: string | null
  description?: string | null
  is_done?: boolean
  is_cancelled?: boolean
  is_reminded?: boolean
  scheduled_at?: string | null
  scheduled_end_at?: string | null
  remind_at?: string | null
  occurred_at?: string | null
  responded_at?: string | null
  created_at?: string | null
  quick_reply_id?: string | null
  quick_reply_name?: string | null
}

export type TaskShape =
  | { kind: 'timed'; start: Instant; end: Instant | null }
  | { kind: 'allDay'; day: Day; start: Instant; end: Instant; remind: Instant }
  | { kind: 'reminderOnly'; remind: Instant }
  | { kind: 'undated' }

export type TaskState =
  | { kind: 'cancelled' }
  | { kind: 'done'; happenedAt: Instant | null; plannedAt: Instant | null }
  | { kind: 'open'; shape: TaskShape; dueAt: Instant | null; overdue: boolean; reminded: boolean }

export interface ResolvedTask<S extends TaskSource = TaskSource> {
  raw: S
  typeName: string
  description: string
  state: TaskState
  plannedDiffers: boolean
}

// dueBoundary is the moment an open task slips: the window end for a timed or
// all-day span, the start for a point, the reminder for a reminder-only row.
// created_at is a record, never a due time.
export function dueBoundary(shape: TaskShape): Instant | null {
  switch (shape.kind) {
    case 'timed':
      return shape.end ?? shape.start
    case 'allDay':
      return shape.end
    case 'reminderOnly':
      return shape.remind
    case 'undated':
      return null
  }
}

function shapeOf(raw: TaskSource): TaskShape {
  const start = raw.scheduled_at ?? null
  const end = raw.scheduled_end_at ?? null
  if (start && isAllDayRange(start, end)) {
    const day = toLocalDateInput(start)
    const range = allDayRange(day)
    return {
      kind: 'allDay',
      day,
      start,
      end: end ?? start,
      remind: raw.remind_at ?? range?.remind ?? start,
    }
  }
  if (start) return { kind: 'timed', start, end }
  if (raw.remind_at) return { kind: 'reminderOnly', remind: raw.remind_at }
  return { kind: 'undated' }
}

function plannedDiffers(happenedAt: Instant | null, plannedAt: Instant | null): boolean {
  if (!happenedAt || !plannedAt) return false
  return new Date(happenedAt).toDateString() !== new Date(plannedAt).toDateString()
}

// resolveTask classifies a row: cancelled, done (with the happened clock), or
// open (with its shape and due moment). Done and cancelled rows carry no
// overdue state.
export function resolveTask<S extends TaskSource>(raw: S, env: TaskEnv): ResolvedTask<S> {
  const typeName = (raw.type ?? '').trim() || 'Task'
  const description = (raw.description ?? '').trim()

  if (raw.is_cancelled) {
    return { raw, typeName, description, state: { kind: 'cancelled' }, plannedDiffers: false }
  }
  if (raw.is_done) {
    const happenedAt = raw.occurred_at ?? raw.responded_at ?? null
    const plannedAt = raw.scheduled_at ?? null
    return {
      raw,
      typeName,
      description,
      state: { kind: 'done', happenedAt, plannedAt },
      plannedDiffers: plannedDiffers(happenedAt, plannedAt),
    }
  }

  const shape = shapeOf(raw)
  const dueAt = dueBoundary(shape)
  const overdue = dueAt !== null && new Date(dueAt).getTime() < env.clock.now().getTime()
  return {
    raw,
    typeName,
    description,
    state: { kind: 'open', shape, dueAt, overdue, reminded: !!raw.is_reminded },
    plannedDiffers: false,
  }
}

// --- presentation ----------------------------------------------------------

export type SegmentTone = 'default' | 'muted' | 'destructive' | 'warning' | 'success'

export interface Segment {
  id: string
  text: string
  tone: SegmentTone
}

export type TaskStatusLabel = 'Open' | 'Reminded' | 'Overdue' | 'Done' | 'Canceled'
export type StatusVariant = 'default' | 'secondary' | 'destructive' | 'success'

export interface StatusSegment extends Segment {
  label: TaskStatusLabel
  variant: StatusVariant
}

export interface TaskPresentation {
  title: Segment
  status: StatusSegment
  due: Segment | null
  schedule: Segment | null
  reminder: Segment | null
  plannedFor: Segment | null
}

export type SegmentSlot = 'due' | 'schedule' | 'reminder' | 'plannedFor'

export interface FieldDef {
  id: SegmentSlot
  when(t: ResolvedTask): boolean
  render(t: ResolvedTask, env: TaskEnv): Segment
}

function defineView(_id: string, fields: FieldDef[]): FieldDef[] {
  return fields
}

function seg(id: string, text: string, tone: SegmentTone = 'muted'): Segment {
  return { id, text, tone }
}

function statusSegment(t: ResolvedTask): StatusSegment {
  if (t.state.kind === 'cancelled') {
    return { id: 'status', text: 'Canceled', label: 'Canceled', variant: 'secondary', tone: 'muted' }
  }
  if (t.state.kind === 'done') {
    return { id: 'status', text: 'Done', label: 'Done', variant: 'success', tone: 'success' }
  }
  if (t.state.reminded) {
    return { id: 'status', text: 'Reminded', label: 'Reminded', variant: 'secondary', tone: 'muted' }
  }
  if (t.state.overdue) {
    return { id: 'status', text: 'Overdue', label: 'Overdue', variant: 'destructive', tone: 'destructive' }
  }
  return { id: 'status', text: 'Open', label: 'Open', variant: 'default', tone: 'default' }
}

function scheduleField(compact: boolean): FieldDef {
  return {
    id: 'schedule',
    when: (t) =>
      t.state.kind === 'open' && (t.state.shape.kind === 'timed' || t.state.shape.kind === 'allDay'),
    render: (t) => {
      const shape = (t.state as Extract<TaskState, { kind: 'open' }>).shape
      if (shape.kind === 'allDay') {
        const text = `${formatDate(shape.start)} (all day)`
        return seg('schedule', compact ? `Scheduled for ${text}` : text)
      }
      if (shape.kind === 'timed') {
        const text = shape.end
          ? `${formatDateTime(shape.start)} – ${formatDateTime(shape.end)}`
          : formatDateTime(shape.start)
        return seg('schedule', compact ? `Scheduled for ${text}` : text)
      }
      return seg('schedule', '')
    },
  }
}

function reminderField(compact: boolean): FieldDef {
  return {
    id: 'reminder',
    when: (t) => !!t.raw.remind_at,
    render: (t) => {
      const text = formatDateTime(t.raw.remind_at!)
      return seg('reminder', compact ? `Reminder at ${text}` : text)
    },
  }
}

const dueField: FieldDef = {
  id: 'due',
  when: () => true,
  render: (t) => {
    if (t.state.kind === 'done') {
      return seg('due', t.state.happenedAt ? formatDateTime(t.state.happenedAt) : '')
    }
    if (t.state.kind === 'open' && t.state.dueAt) {
      const text =
        t.state.shape.kind === 'allDay'
          ? `${formatDate(t.state.dueAt)} (all day)`
          : formatDateTime(t.state.dueAt)
      return seg('due', text)
    }
    return seg('due', '')
  },
}

const plannedForField: FieldDef = {
  id: 'plannedFor',
  when: (t) => t.state.kind === 'done' && t.plannedDiffers,
  render: (t) => seg('plannedFor', formatDateTime((t.state as Extract<TaskState, { kind: 'done' }>).plannedAt!)),
}

// views select which derived lines a surface shows. full and reminder carry
// bare values (templates keep their own "Scheduled:"/"Reminder:" prefixes);
// compact carries the notification phrasing; table carries the due column.
export const views = {
  full: defineView('full', [scheduleField(false), reminderField(false)]),
  reminder: defineView('reminder', [scheduleField(false), reminderField(false)]),
  compact: defineView('compact', [scheduleField(true), reminderField(true)]),
  table: defineView('table', [dueField]),
  history: defineView('history', [plannedForField]),
}

function iconFor(type: string): Component {
  const t = (type || '').toLowerCase()
  if (t.includes('call')) return Phone
  if (t.includes('whatsapp') || t.includes('wa ') || t.includes('message')) return MessageCircle
  if (t.includes('mail') || t.includes('email')) return Mail
  if (t.includes('meeting') || t.includes('visit')) return CalendarClock
  if (t.includes('follow')) return CheckCheck
  return NotepadText
}

export { iconFor }

// present derives the display segments for one surface. title and status are
// always present; the date slots depend on the view and the task's state.
export function present(
  t: ResolvedTask,
  view: keyof typeof views = 'full',
  env: TaskEnv = localEnv(),
): TaskPresentation {
  const out: TaskPresentation = {
    title: seg('title', t.typeName, 'default'),
    status: statusSegment(t),
    due: null,
    schedule: null,
    reminder: null,
    plannedFor: null,
  }
  for (const field of views[view]) {
    if (!field.when(t)) continue
    out[field.id] = field.render(t, env)
  }
  return out
}

// --- payload builders ------------------------------------------------------

export type WhenInput =
  | { kind: 'none' }
  | { kind: 'allDay'; day: Day }
  | { kind: 'timed'; day: Day; time: string; endDay?: string; endTime?: string }

export type ReminderChoice = { kind: 'orgDefault' } | { kind: 'offsetMinutes'; minutes: number }

export interface Issue {
  field: 'type' | 'when' | 'remind'
  code: 'missing' | 'invalid'
  message: string
}

export type Built<B> = { ok: true; body: B } | { ok: false; issues: Issue[] }

function issue(field: Issue['field'], code: Issue['code'], message: string): Issue {
  return { field, code, message }
}

function isIssue(v: unknown): v is Issue {
  return typeof v === 'object' && v !== null && 'field' in v && 'code' in v
}

// whenFromInputs maps the two date inputs every form uses: a day alone is the
// whole local day, a day plus time is a point, no day is no schedule.
export function whenFromInputs(i: {
  day?: string
  time?: string
  endDay?: string
  endTime?: string
}): WhenInput | null {
  if (!i.day) return null
  if (!i.time) return { kind: 'allDay', day: i.day }
  return { kind: 'timed', day: i.day, time: i.time, endDay: i.endDay, endTime: i.endTime }
}

interface TimingPayload {
  scheduled_at: string
  scheduled_end_at?: string
  remind_at?: string
}

// timing assembles the schedule triple. A date without a time travels as its
// whole local day with the fixed 09:00 reminder; a timed task sends its start
// and, unless a relative override is picked, no remind_at so the server's org
// nudge applies. A malformed date reports an issue instead of being dropped.
function timing(day: Day, time: string | undefined, reminder: ReminderChoice): TimingPayload | Issue {
  if (!time) {
    const range = allDayRange(day)
    if (!range) return issue('when', 'invalid', 'Enter a valid date')
    return { scheduled_at: range.start, scheduled_end_at: range.end, remind_at: range.remind }
  }
  const start = mergeDateTime(day, time)
  if (!start) return issue('when', 'invalid', 'Enter a valid date')
  const out: TimingPayload = { scheduled_at: start }
  if (reminder.kind === 'offsetMinutes') {
    out.remind_at = new Date(new Date(start).getTime() - reminder.minutes * 60_000).toISOString()
  }
  return out
}

// buildTaskBody builds the "Schedule task" payload. mode is reserved for a
// future move wrapper; both modes produce the same create-shaped body today.
export function buildTaskBody(
  draft: { type: string; when: WhenInput; reminder: ReminderChoice; description?: string },
  _env: TaskEnv,
  _mode: 'create' | 'move' = 'create',
): Built<ActivityMutationBody> {
  const type = draft.type.trim()
  const issues: Issue[] = []
  if (!type) issues.push(issue('type', 'missing', 'Type is required'))
  if (draft.when.kind === 'none') issues.push(issue('when', 'missing', 'Date is required'))
  if (issues.length) return { ok: false, issues }

  const w = draft.when as Exclude<WhenInput, { kind: 'none' }>
  const payload = timing(w.day, w.kind === 'timed' ? w.time : undefined, draft.reminder)
  if (isIssue(payload)) return { ok: false, issues: [payload] }

  const body: ActivityMutationBody = { type, ...payload }
  const description = draft.description?.trim()
  if (description) body.description = description
  return { ok: true, body }
}

export interface FollowUpDraft {
  type?: string
  when: WhenInput
  reminder: ReminderChoice
}

// buildAttempt builds the "What happened?" payload: the completed attempt
// plus, only when the picked behavior schedules one, the follow-up task.
export function buildAttempt(
  draft: { quickReplyId: string; behavior: string; followUp: FollowUpDraft | null },
  _env: TaskEnv,
  behaviors: BehaviorMap = defaultBehaviors,
): Built<ActivityMutationBody> {
  const spec = behaviorOf(behaviors, draft.behavior)
  const body: ActivityMutationBody = {
    quick_reply_id: draft.quickReplyId || null,
    is_done: true,
  }
  if (!spec.enablesFollowUp || !draft.followUp || draft.followUp.when.kind === 'none') {
    return { ok: true, body }
  }

  const fu = draft.followUp
  const w = fu.when as Exclude<WhenInput, { kind: 'none' }>
  const payload = timing(w.day, w.kind === 'timed' ? w.time : undefined, fu.reminder)
  if (isIssue(payload)) return { ok: false, issues: [payload] }

  const followUp: ActivityFollowUpBody = { ...payload }
  const type = fu.type?.trim()
  if (type) followUp.type = type
  body.follow_up = followUp
  return { ok: true, body }
}

// --- edit draft (the reminder-follows policy) ------------------------------

export interface EditDraft {
  type: string
  description: string
  day: string
  time: string
  endDay: string
  endTime: string
  remindDay: string
  remindTime: string
  remindTouched: boolean
  build(): Built<ActivityMutationBody>
}

// followReminder resolves the reminder for a moved task. When the user did not
// touch the reminder fields:
// - timed to timed keeps the same offset from the start;
// - all-day to all-day re-anchors 09:00 on the new day;
// - a shape change follows the destination default (timed: org nudge lead,
//   all-day: 09:00);
// - a cleared schedule keeps the stored reminder.
function followReminder(
  raw: TaskSource,
  newStart: string | null,
  newAllDay: boolean,
  env: TaskEnv,
): string | null {
  const oldRemind = raw.remind_at ? Date.parse(raw.remind_at) : NaN
  if (newStart === null) return raw.remind_at ?? null

  const oldAllDay = isAllDayRange(raw.scheduled_at, raw.scheduled_end_at)
  const oldStart = raw.scheduled_at ? Date.parse(raw.scheduled_at) : NaN

  if (newAllDay) {
    const range = allDayRange(toLocalDateInput(newStart))
    return range?.remind ?? null
  }
  if (!oldAllDay && !Number.isNaN(oldRemind) && !Number.isNaN(oldStart)) {
    const offset = oldStart - oldRemind
    return new Date(Date.parse(newStart) - offset).toISOString()
  }
  if (Number.isNaN(oldRemind)) return null
  return new Date(Date.parse(newStart) - env.defaults.nudgeLeadMinutes * 60_000).toISOString()
}

// editDraftFrom prefills the inline edit form: an all-day task keeps the date
// only and rebuilds its day window on save; a timed task keeps start, end, and
// reminder. build() applies the reminder-follows policy above.
export function editDraftFrom(raw: TaskSource, env: TaskEnv): EditDraft {
  const allDay = isAllDayRange(raw.scheduled_at, raw.scheduled_end_at)
  const draft: EditDraft = {
    type: raw.type ?? '',
    description: raw.description ?? '',
    day: raw.scheduled_at ? toLocalDateInput(raw.scheduled_at) : '',
    time: allDay || !raw.scheduled_at ? '' : toLocalTimeInput(raw.scheduled_at),
    endDay: allDay || !raw.scheduled_end_at ? '' : toLocalDateInput(raw.scheduled_end_at),
    endTime: allDay || !raw.scheduled_end_at ? '' : toLocalTimeInput(raw.scheduled_end_at),
    remindDay: raw.remind_at ? toLocalDateInput(raw.remind_at) : '',
    remindTime: raw.remind_at ? toLocalTimeInput(raw.remind_at) : '',
    remindTouched: false,
    build(): Built<ActivityMutationBody> {
      const type = draft.type.trim()
      if (!type) return { ok: false, issues: [issue('type', 'missing', 'Type is required')] }

      let scheduledAt: string | null = null
      let scheduledEnd: string | null = null
      let newAllDay = false
      if (draft.day && !draft.time) {
        const range = allDayRange(draft.day)
        if (!range) return { ok: false, issues: [issue('when', 'invalid', 'Enter a valid date')] }
        scheduledAt = range.start
        scheduledEnd = range.end
        newAllDay = true
      } else if (draft.day && draft.time) {
        scheduledAt = mergeDateTime(draft.day, draft.time)
        if (!scheduledAt) return { ok: false, issues: [issue('when', 'invalid', 'Enter a valid date')] }
        scheduledEnd = mergeDateTime(draft.endDay, draft.endTime)
      }

      const remindAt = draft.remindTouched
        ? mergeDateTime(draft.remindDay, draft.remindTime)
        : followReminder(raw, scheduledAt, newAllDay, env)

      return {
        ok: true,
        body: {
          type,
          description: draft.description.trim(),
          scheduled_at: scheduledAt,
          scheduled_end_at: scheduledEnd,
          remind_at: remindAt,
        },
      }
    },
  }
  return draft
}

// --- behaviors -------------------------------------------------------------

export interface BehaviorSpec {
  enablesFollowUp: boolean
  closesLead: boolean
  doneMessage: string
}

export type BehaviorMap = Record<string, BehaviorSpec>

export const defaultBehaviors: BehaviorMap = {
  log: { enablesFollowUp: false, closesLead: false, doneMessage: 'Attempt logged' },
  next: { enablesFollowUp: true, closesLead: false, doneMessage: 'Attempt logged, follow-up created' },
  close_lost: { enablesFollowUp: false, closesLead: true, doneMessage: 'Attempt logged' },
}

// behaviorOf resolves a quick reply's behavior; an unknown or missing one is
// record-only.
export function behaviorOf(behaviors: BehaviorMap, behavior?: string | null): BehaviorSpec {
  return (behavior && behaviors[behavior]) || behaviors.log
}

// --- presets and snooze ----------------------------------------------------

export interface NextPreset {
  label: string
  at(): Date
}

function inDays(days: number): Date {
  const d = new Date()
  d.setDate(d.getDate() + days)
  return d
}

export const nextPresets: NextPreset[] = [
  { label: 'In 2 hours', at: () => new Date(Date.now() + 2 * 60 * 60_000) },
  { label: 'Tomorrow', at: () => inDays(1) },
  { label: 'In 2 days', at: () => inDays(2) },
  { label: 'In 3 days', at: () => inDays(3) },
  { label: 'Next week', at: () => inDays(7) },
]

export interface SnoozePreset {
  label: string
  minutes: number
}

export const snoozePresets: SnoozePreset[] = [
  { label: '15 minutes', minutes: 15 },
  { label: '1 hour', minutes: 60 },
  { label: '3 hours', minutes: 180 },
  { label: 'Tomorrow', minutes: 24 * 60 },
]

export const allDaySnoozePresets: SnoozePreset[] = [
  { label: 'Tomorrow', minutes: 24 * 60 },
  { label: 'In 2 days', minutes: 2 * 24 * 60 },
  { label: 'Next week', minutes: 7 * 24 * 60 },
]

// snoozeTarget resolves the new remind_at for a snooze tap. A timed task moves
// to now + minutes; an all-day task keeps its day grid by re-anchoring the
// stored nudge time-of-day on the preset day.
export function snoozeTarget(r: TaskSource, minutes: number, env: TaskEnv): Instant {
  if (r.remind_at && isAllDayRange(r.scheduled_at, r.scheduled_end_at)) {
    const nudge = new Date(r.remind_at)
    const days = Math.max(1, Math.round(minutes / (24 * 60)))
    const target = new Date(env.clock.now())
    target.setDate(target.getDate() + days)
    target.setHours(nudge.getHours(), nudge.getMinutes(), nudge.getSeconds(), nudge.getMilliseconds())
    return target.toISOString()
  }
  return new Date(env.clock.now().getTime() + minutes * 60_000).toISOString()
}

// findSelectedPreset highlights the preset that produced the current local
// date/time inputs, or '' when none matches.
export function findSelectedPreset(date: string, time: string, list: NextPreset[]): string {
  if (!date || !time) return ''
  const preset = list.find((p) => {
    const at = p.at().toISOString()
    return toLocalDateInput(at) === date && toLocalTimeInput(at) === time
  })
  return preset?.label ?? ''
}
