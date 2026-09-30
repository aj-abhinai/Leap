import { describe, it, expect, beforeEach } from 'vitest'
import {
  loadRememberedCreateValues,
  rememberCreateValues,
  rememberedStageId,
} from '@/composables/useLeadFormDefaults'

const stages = [
  { id: 's-open', is_closing: false },
  { id: 's-lost', is_closing: true },
]

describe('remembered create values', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('round-trips the program and assignee for the next create', () => {
    rememberCreateValues('u1', 'p1', { programId: 'prog1', assignedTo: 'u2', stageId: 's-open' })

    expect(loadRememberedCreateValues('u1')).toEqual({ programId: 'prog1', assignedTo: 'u2' })
  })

  it('keeps stages per pipeline', () => {
    rememberCreateValues('u1', 'p1', { programId: '', assignedTo: '', stageId: 's-open' })
    rememberCreateValues('u1', 'p2', { programId: '', assignedTo: '', stageId: 's2' })

    expect(rememberedStageId('u1', 'p1', stages)).toBe('s-open')
    expect(rememberedStageId('u1', 'p2', [{ id: 's2', is_closing: false }])).toBe('s2')
  })

  it('isolates users', () => {
    rememberCreateValues('u1', 'p1', { programId: 'prog1', assignedTo: 'u2', stageId: 's-open' })

    expect(loadRememberedCreateValues('u2')).toEqual({ programId: '', assignedTo: '' })
    expect(rememberedStageId('u2', 'p1', stages)).toBe('')
  })

  it('never returns a closing or unknown stage', () => {
    rememberCreateValues('u1', 'p1', { programId: '', assignedTo: '', stageId: 's-lost' })

    expect(rememberedStageId('u1', 'p1', stages)).toBe('')
    expect(rememberedStageId('u1', 'p1', [{ id: 's-other', is_closing: false }])).toBe('')
  })

  it('reads dead storage as no memory', () => {
    localStorage.setItem('crm:leads:form:u1:stages', 'not json')

    expect(loadRememberedCreateValues('u1')).toEqual({ programId: '', assignedTo: '' })
    expect(rememberedStageId('u1', 'p1', stages)).toBe('')
  })
})
