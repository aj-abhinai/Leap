import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import TaskRow from '@/components/leads/TaskRow.vue'
import type { LeadActivity } from '@/api/leads'

vi.mock('@/api/leads', () => ({ updateLeadActivity: vi.fn() }))
vi.mock('vue-sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

import { updateLeadActivity } from '@/api/leads'

const updateMock = vi.mocked(updateLeadActivity)

function allDayActivity(): LeadActivity {
  return {
    id: 'a1',
    lead_id: 'l1',
    type: 'Call',
    description: '',
    scheduled_at: new Date(2026, 8, 30).toISOString(),
    scheduled_end_at: new Date(new Date(2026, 9, 1).getTime() - 1).toISOString(),
    remind_at: new Date(2026, 8, 30, 9, 0).toISOString(),
    is_done: false,
    is_cancelled: false,
    is_reminded: false,
    created_at: new Date(2026, 8, 29).toISOString(),
  }
}

describe('TaskRow', () => {
  beforeEach(() => {
    updateMock.mockReset()
  })

  // An all-day task shows its date once; the edit form prefills the date only,
  // and moving the day moves the 09:00 nudge with it.
  it('edits an all-day window and keeps the nudge on the task day', async () => {
    const wrapper = mount(TaskRow, {
      props: { leadId: 'l1', activity: allDayActivity(), quickReplies: [], activityTypes: [] },
    })
    expect(wrapper.text()).toContain('(all day)')

    const vm = wrapper.vm as unknown as { startEdit: () => void }
    vm.startEdit()
    await nextTick()

    const dates = wrapper.findAll('input[type="date"]')
    expect((dates[0].element as HTMLInputElement).value).toBe('2026-09-30')
    expect((dates[1].element as HTMLInputElement).value).toBe('')
    expect((dates[2].element as HTMLInputElement).value).toBe('2026-09-30')

    await dates[0].setValue('2026-10-03')
    await nextTick()

    const moved = wrapper.findAll('input[type="date"]')
    expect((moved[2].element as HTMLInputElement).value).toBe('2026-10-03')

    const save = wrapper.findAll('button').find((b) => b.text() === 'Save')
    expect(save).toBeTruthy()
    await save!.trigger('click')
    await flushPromises()

    expect(updateMock).toHaveBeenCalledTimes(1)
    const payload = updateMock.mock.calls[0][2] as Record<string, unknown>
    expect(payload.scheduled_at).toBe(new Date(2026, 9, 3).toISOString())
    expect(payload.scheduled_end_at).toBe(new Date(new Date(2026, 9, 4).getTime() - 1).toISOString())
    expect(payload.remind_at).toBe(new Date(2026, 9, 3, 9, 0, 0).toISOString())
  })

  it('offers day snooze presets for an all-day task', () => {
    const wrapper = mount(TaskRow, {
      props: { leadId: 'l1', activity: allDayActivity(), quickReplies: [], activityTypes: [] },
    })
    const options = (wrapper.vm as unknown as { snoozeOptions: { label: string }[] }).snoozeOptions
    expect(options.map((p) => p.label)).toEqual(['Tomorrow', 'In 2 days', 'Next week'])
  })

  // THE original bug from the reschedule path: a follow-up date entered
  // without a time was silently dropped. It now travels as the whole local
  // day: start, end, and the 09:00 remind.
  it('reschedules a date-only follow-up as the whole day', async () => {
    const wrapper = mount(TaskRow, {
      props: {
        leadId: 'l1',
        activity: allDayActivity(),
        quickReplies: [{ id: 'q2', name: 'No Reply', group_name: 'NC', sort_order: 0, behavior: 'next' }],
        activityTypes: [],
      },
    })
    const vm = wrapper.vm as unknown as { startReschedule: () => void }
    vm.startReschedule()
    await nextTick()

    const chip = wrapper.findAll('button').find((b) => b.text().includes('No Reply'))
    expect(chip).toBeTruthy()
    await chip!.trigger('click')

    await wrapper.find('input[type="date"]').setValue('2026-10-08')
    await wrapper.find('input[type="time"]').setValue('')

    const save = wrapper.findAll('button').find((b) => b.text() === 'Save')
    expect(save).toBeTruthy()
    await save!.trigger('click')
    await flushPromises()

    expect(updateMock).toHaveBeenCalledTimes(1)
    const payload = updateMock.mock.calls[0][2] as Record<string, unknown>
    expect(payload.is_done).toBe(true)
    expect(payload.follow_up).toEqual({
      scheduled_at: new Date(2026, 9, 8).toISOString(),
      scheduled_end_at: new Date(new Date(2026, 9, 9).getTime() - 1).toISOString(),
      remind_at: new Date(2026, 9, 8, 9, 0, 0).toISOString(),
    })
  })

  // A timed follow-up sends only the start; the reminder stays with the
  // system default.
  it('reschedules a timed follow-up as a point task', async () => {
    const wrapper = mount(TaskRow, {
      props: {
        leadId: 'l1',
        activity: allDayActivity(),
        quickReplies: [{ id: 'q2', name: 'No Reply', group_name: 'NC', sort_order: 0, behavior: 'next' }],
        activityTypes: [],
      },
    })
    const vm = wrapper.vm as unknown as { startReschedule: () => void }
    vm.startReschedule()
    await nextTick()

    const chip = wrapper.findAll('button').find((b) => b.text().includes('No Reply'))
    await chip!.trigger('click')

    await wrapper.find('input[type="date"]').setValue('2026-10-08')
    await wrapper.find('input[type="time"]').setValue('15:30')

    const save = wrapper.findAll('button').find((b) => b.text() === 'Save')
    await save!.trigger('click')
    await flushPromises()

    const payload = updateMock.mock.calls[0][2] as Record<string, unknown>
    expect(payload.follow_up).toEqual({
      scheduled_at: new Date(2026, 9, 8, 15, 30).toISOString(),
    })
  })
})
