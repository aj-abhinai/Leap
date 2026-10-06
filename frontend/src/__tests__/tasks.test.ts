import { describe, expect, it } from 'vitest'
import {
  behaviorOf,
  buildAttempt,
  buildTaskBody,
  defaultBehaviors,
  editDraftFrom,
  findSelectedPreset,
  present,
  resolveTask,
  snoozeTarget,
  whenFromInputs,
  type NextPreset,
  type TaskSource,
} from '@/tasks'
import { fixedEnv } from '@/tasks/testing'
import { allDayRange, mergeDateTime, toLocalDateInput, toLocalTimeInput } from '@/utils/time'

const NOW = '2026-10-06T12:00:00'
const env = fixedEnv(NOW)

function timed(day: string, time: string): string {
  return mergeDateTime(day, time)!
}

function statusOf(raw: TaskSource) {
  return present(resolveTask(raw, env), 'full', env).status
}

describe('resolveTask due boundary', () => {
  it('uses the window end, then the start, then the reminder; never created_at', () => {
    const start = timed('2026-10-06', '12:30')
    const end = timed('2026-10-06', '13:30')
    const ranged = resolveTask({ scheduled_at: start, scheduled_end_at: end }, env)
    expect(ranged.state.kind).toBe('open')
    if (ranged.state.kind === 'open') {
      expect(ranged.state.dueAt).toBe(end)
      expect(ranged.state.overdue).toBe(false)
    }

    const point = resolveTask({ scheduled_at: timed('2026-10-06', '10:00') }, env)
    if (point.state.kind === 'open') expect(point.state.overdue).toBe(true)

    const reminderOnly = resolveTask({ remind_at: timed('2026-10-06', '10:00') }, env)
    if (reminderOnly.state.kind === 'open') {
      expect(reminderOnly.state.shape.kind).toBe('reminderOnly')
      expect(reminderOnly.state.overdue).toBe(true)
    }

    const undated = resolveTask({ created_at: '2026-01-01T00:00:00' }, env)
    if (undated.state.kind === 'open') {
      expect(undated.state.dueAt).toBeNull()
      expect(undated.state.overdue).toBe(false)
    }
  })

  it('treats an all-day task as overdue only after its day', () => {
    const past = allDayRange('2026-10-05')!
    const slipped = resolveTask({ scheduled_at: past.start, scheduled_end_at: past.end }, env)
    if (slipped.state.kind === 'open') expect(slipped.state.overdue).toBe(true)

    const today = allDayRange('2026-10-06')!
    const open = resolveTask({ scheduled_at: today.start, scheduled_end_at: today.end }, env)
    if (open.state.kind === 'open') expect(open.state.overdue).toBe(false)
  })

  it('gives done and cancelled rows no overdue state', () => {
    const done = resolveTask({ is_done: true, scheduled_at: timed('2026-10-01', '10:00') }, env)
    expect(done.state.kind).toBe('done')
    const cancelled = resolveTask({ is_cancelled: true, scheduled_at: timed('2026-10-01', '10:00') }, env)
    expect(cancelled.state.kind).toBe('cancelled')
  })
})

describe('present', () => {
  it('derives the status by precedence', () => {
    expect(statusOf({ scheduled_at: timed('2026-10-06', '13:00') }).label).toBe('Open')
    expect(statusOf({ scheduled_at: timed('2026-10-06', '10:00') })).toMatchObject({
      label: 'Overdue',
      variant: 'destructive',
    })
    expect(statusOf({ scheduled_at: timed('2026-10-06', '10:00'), is_reminded: true })).toMatchObject({
      label: 'Reminded',
      variant: 'secondary',
    })
    expect(statusOf({ is_done: true })).toMatchObject({ label: 'Done', variant: 'success' })
    expect(statusOf({ is_cancelled: true })).toMatchObject({ label: 'Canceled', variant: 'secondary' })
  })

  it('falls back to the Task label', () => {
    expect(present(resolveTask({}, env), 'full', env).title.text).toBe('Task')
  })

  it('renders the schedule triple and the reminder line', () => {
    const range = allDayRange('2026-10-06')!
    const full = present(
      resolveTask({ scheduled_at: range.start, scheduled_end_at: range.end, remind_at: range.remind }, env),
      'full',
      env,
    )
    expect(full.schedule?.text).toContain('(all day)')
    expect(full.reminder).not.toBeNull()

    const compact = present(
      resolveTask({ scheduled_at: range.start, scheduled_end_at: range.end, remind_at: range.remind }, env),
      'compact',
      env,
    )
    expect(compact.schedule?.text).toContain('Scheduled for')
    expect(compact.reminder?.text).toContain('Reminder at')
  })

  it('leads done rows with the happened time and shows plannedFor only when the day differs', () => {
    const raw = {
      is_done: true,
      occurred_at: timed('2026-10-06', '10:00'),
      scheduled_at: timed('2026-10-05', '09:00'),
    }
    const table = present(resolveTask(raw, env), 'table', env)
    expect(table.due?.text).toBeTruthy()
    expect(table.due?.text).not.toContain('all day')

    const history = present(resolveTask(raw, env), 'history', env)
    expect(history.plannedFor).not.toBeNull()

    const sameDay = present(
      resolveTask({ is_done: true, occurred_at: timed('2026-10-06', '10:00'), scheduled_at: timed('2026-10-06', '09:00') }, env),
      'history',
      env,
    )
    expect(sameDay.plannedFor).toBeNull()
  })

  it('marks an all-day due in the table view', () => {
    const range = allDayRange('2026-10-06')!
    const table = present(resolveTask({ scheduled_at: range.start, scheduled_end_at: range.end }, env), 'table', env)
    expect(table.due?.text).toContain('(all day)')
  })
})

describe('buildTaskBody', () => {
  it('refuses a missing type or date', () => {
    const noType = buildTaskBody(
      { type: '  ', when: { kind: 'allDay', day: '2026-10-15' }, reminder: { kind: 'orgDefault' } },
      env,
    )
    expect(noType.ok).toBe(false)
    const noDate = buildTaskBody({ type: 'Call', when: { kind: 'none' }, reminder: { kind: 'orgDefault' } }, env)
    expect(noDate.ok).toBe(false)
  })

  it('builds the all-day triple from a date alone', () => {
    const range = allDayRange('2026-10-15')!
    const built = buildTaskBody(
      { type: 'Call', when: whenFromInputs({ day: '2026-10-15' })!, reminder: { kind: 'orgDefault' } },
      env,
    )
    expect(built.ok).toBe(true)
    if (built.ok) {
      expect(built.body).toEqual({
        type: 'Call',
        scheduled_at: range.start,
        scheduled_end_at: range.end,
        remind_at: range.remind,
      })
    }
  })

  it('omits the org-default reminder on a timed task and computes an override', () => {
    const start = timed('2026-10-15', '15:00')
    const def = buildTaskBody(
      { type: 'Call', when: whenFromInputs({ day: '2026-10-15', time: '15:00' })!, reminder: { kind: 'orgDefault' } },
      env,
    )
    if (def.ok) expect(def.body.remind_at).toBeUndefined()

    const override = buildTaskBody(
      {
        type: 'Call',
        when: whenFromInputs({ day: '2026-10-15', time: '15:00' })!,
        reminder: { kind: 'offsetMinutes', minutes: 60 },
      },
      env,
    )
    if (override.ok) expect(override.body.remind_at).toBe(new Date(Date.parse(start) - 3_600_000).toISOString())
  })

  it('reports a malformed date instead of dropping it', () => {
    const built = buildTaskBody(
      { type: 'Call', when: whenFromInputs({ day: '2026-02-31' })!, reminder: { kind: 'orgDefault' } },
      env,
    )
    expect(built.ok).toBe(false)
    if (!built.ok) expect(built.issues[0].message).toBe('Enter a valid date')
  })
})

describe('buildAttempt', () => {
  const followUp = { when: whenFromInputs({ day: '2026-10-15' })!, reminder: { kind: 'orgDefault' as const } }

  it('creates a follow-up only for the next behavior', () => {
    const next = buildAttempt({ quickReplyId: 'q1', behavior: 'next', followUp }, env)
    if (next.ok) expect(next.body.follow_up).toBeTruthy()

    const log = buildAttempt({ quickReplyId: 'q1', behavior: 'log', followUp }, env)
    if (log.ok) expect(log.body.follow_up).toBeUndefined()

    const lost = buildAttempt({ quickReplyId: 'q1', behavior: 'close_lost', followUp }, env)
    if (lost.ok) expect(lost.body.follow_up).toBeUndefined()
  })

  it('passes the reply through verbatim', () => {
    const cleared = buildAttempt({ quickReplyId: '', behavior: 'log', followUp: null }, env)
    if (cleared.ok) expect(cleared.body).toEqual({ quick_reply_id: '', is_done: true })

    const kept = buildAttempt({ quickReplyId: null, behavior: 'log', followUp: null }, env)
    if (kept.ok) expect(kept.body).toEqual({ quick_reply_id: null, is_done: true })
  })

  it('falls back to record-only for an unknown behavior', () => {
    expect(behaviorOf(defaultBehaviors, 'escalate').enablesFollowUp).toBe(false)
    expect(behaviorOf(defaultBehaviors, 'close_lost').closesLead).toBe(true)
  })
})

describe('editDraftFrom (the reminder-follows policy)', () => {
  it('keeps the relative offset when a timed task moves', () => {
    const draft = editDraftFrom(
      { type: 'Call', scheduled_at: timed('2026-10-06', '15:00'), remind_at: timed('2026-10-06', '14:55') },
      env,
    )
    draft.day = '2026-10-07'
    draft.time = '17:00'

    const built = draft.build()
    expect(built.ok).toBe(true)
    if (built.ok) {
      const expected = new Date(Date.parse(timed('2026-10-07', '17:00')) - 5 * 60_000).toISOString()
      expect(built.body.remind_at).toBe(expected)
    }
  })

  it('leaves a reminder-less task without one', () => {
    const draft = editDraftFrom({ type: 'Call', scheduled_at: timed('2026-10-06', '15:00') }, env)
    draft.time = '17:00'

    const built = draft.build()
    if (built.ok) expect(built.body.remind_at).toBeNull()
  })

  it('lets an explicitly touched reminder win', () => {
    const draft = editDraftFrom(
      { type: 'Call', scheduled_at: timed('2026-10-06', '15:00'), remind_at: timed('2026-10-06', '14:55') },
      env,
    )
    draft.day = '2026-10-07'
    draft.time = '17:00'
    draft.remindTouched = true
    draft.remindDay = '2026-10-07'
    draft.remindTime = '16:00'

    const built = draft.build()
    if (built.ok) expect(built.body.remind_at).toBe(timed('2026-10-07', '16:00'))
  })

  it('re-anchors 09:00 when an all-day task moves day', () => {
    const range = allDayRange('2026-10-05')!
    const draft = editDraftFrom(
      { type: 'Call', scheduled_at: range.start, scheduled_end_at: range.end, remind_at: range.remind },
      env,
    )
    draft.day = '2026-10-08'

    const built = draft.build()
    const next = allDayRange('2026-10-08')!
    if (built.ok) {
      expect(built.body).toMatchObject({
        scheduled_at: next.start,
        scheduled_end_at: next.end,
        remind_at: next.remind,
      })
    }
  })

  it('uses the org nudge when an all-day task becomes timed', () => {
    const range = allDayRange('2026-10-05')!
    const draft = editDraftFrom(
      { type: 'Call', scheduled_at: range.start, scheduled_end_at: range.end, remind_at: range.remind },
      env,
    )
    draft.day = '2026-10-08'
    draft.time = '10:00'

    const built = draft.build()
    if (built.ok) {
      const start = timed('2026-10-08', '10:00')
      expect(built.body.remind_at).toBe(new Date(Date.parse(start) - 5 * 60_000).toISOString())
    }
  })

  it('keeps the stored reminder when the date is cleared', () => {
    const raw = { type: 'Call', scheduled_at: timed('2026-10-06', '15:00'), remind_at: timed('2026-10-06', '14:55') }
    const draft = editDraftFrom(raw, env)
    draft.day = ''
    draft.time = ''

    const built = draft.build()
    if (built.ok) {
      expect(built.body.scheduled_at).toBeNull()
      expect(built.body.remind_at).toBe(raw.remind_at)
    }
  })

  it('reports a malformed date', () => {
    const draft = editDraftFrom({ type: 'Call' }, env)
    draft.day = '2026-02-31'
    expect(draft.build().ok).toBe(false)
  })
})

describe('snoozeTarget', () => {
  it('moves a timed task to now + minutes', () => {
    const at = snoozeTarget({ scheduled_at: timed('2026-10-06', '15:00') }, 60, env)
    expect(at).toBe(new Date(Date.parse(NOW) + 3_600_000).toISOString())
  })

  it('keeps the all-day grid on the preset day', () => {
    const range = allDayRange('2026-10-05')!
    const at = snoozeTarget(
      { scheduled_at: range.start, scheduled_end_at: range.end, remind_at: range.remind },
      24 * 60,
      env,
    )
    const target = new Date(at)
    expect(target.getDate()).toBe(new Date(NOW).getDate() + 1)
    expect(target.getHours()).toBe(9)
    expect(target.getMinutes()).toBe(0)
  })
})

describe('findSelectedPreset', () => {
  it('highlights the preset that produced the inputs', () => {
    const list: NextPreset[] = [{ label: 'X', at: () => new Date(2026, 9, 8, 15, 30) }]
    const at = list[0].at().toISOString()
    expect(findSelectedPreset(toLocalDateInput(at), toLocalTimeInput(at), list)).toBe('X')
    expect(findSelectedPreset('', '', list)).toBe('')
  })
})
