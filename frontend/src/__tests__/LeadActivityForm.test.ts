import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import LeadActivityForm from '@/components/leads/LeadActivityForm.vue'
import { Select } from '@/components/ui/select'

vi.mock('@/api/leads', () => ({ createLeadActivity: vi.fn() }))
vi.mock('vue-sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    activityTypes: [
      { id: 't1', name: 'Call' },
      { id: 't2', name: 'Send Details' },
    ],
    quickReplies: [
      { id: 'q1', name: 'Share Details', group_name: 'Connected', sort_order: 0, behavior: 'log' },
      { id: 'q2', name: 'No Reply', group_name: 'Not Connected', sort_order: 1, behavior: 'next' },
      { id: 'q3', name: 'Closed Lost', group_name: 'End', sort_order: 2, behavior: 'close_lost' },
    ],
    fetchTags: vi.fn(),
  }),
}))

import { createLeadActivity } from '@/api/leads'

const createMock = vi.mocked(createLeadActivity)

function mountForm() {
  return mount(LeadActivityForm, {
    props: { leadId: 'l1' },
    global: { plugins: [createPinia()] },
  })
}

function findButton(wrapper: ReturnType<typeof mount>, text: string) {
  const b = wrapper.findAll('button').find((x) => x.text().includes(text))
  expect(b, `button containing "${text}"`).toBeTruthy()
  return b!
}

async function pickTypeAndChip(wrapper: ReturnType<typeof mount>, chipText: string) {
  wrapper.findComponent(Select).vm.$emit('update:modelValue', 'Call')
  await flushPromises()
  await findButton(wrapper, chipText).trigger('click')
}

describe('LeadActivityForm — What happened?', () => {
  beforeEach(() => {
    localStorage.clear()
    createMock.mockReset()
  })

  // A save with no outcome chip is a direct log: an attempt stamped now,
  // no follow-up, no schedule.
  it('logs the attempt when no outcome is picked', async () => {
    const wrapper = mountForm()
    await flushPromises()

    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'Call')
    await flushPromises()
    await findButton(wrapper, 'Save').trigger('click')
    await flushPromises()

    expect(createMock).toHaveBeenCalledTimes(1)
    const payload = createMock.mock.calls[0][1]
    expect(payload.type).toBe('Call')
    expect(payload.is_done).toBe(true)
    expect(payload.quick_reply_id).toBeNull()
    expect(payload.follow_up).toBeUndefined()
  })

  // THE original bug: a follow-up date entered without a time was dropped.
  // It now travels as the whole local day: start, end, and the 09:00 remind.
  it('sends the whole local day when a follow-up date has no time', async () => {
    const wrapper = mountForm()
    await flushPromises()

    await pickTypeAndChip(wrapper, 'No Reply')
    await wrapper.find('input[type="date"]').setValue('2026-10-08')
    await findButton(wrapper, 'Save').trigger('click')
    await flushPromises()

    expect(createMock).toHaveBeenCalledTimes(1)
    const payload = createMock.mock.calls[0][1]
    expect(payload.is_done).toBe(true)
    expect(payload.quick_reply_id).toBe('q2')
    expect(payload.follow_up).toEqual({
      scheduled_at: new Date(2026, 9, 8).toISOString(),
      scheduled_end_at: new Date(new Date(2026, 9, 9).getTime() - 1).toISOString(),
      remind_at: new Date(2026, 9, 8, 9, 0, 0).toISOString(),
    })
  })

  // A timed follow-up sends only the start; the reminder stays with the
  // system default unless a relative override was picked.
  it('sends only the start for a timed follow-up', async () => {
    const wrapper = mountForm()
    await flushPromises()

    await pickTypeAndChip(wrapper, 'No Reply')
    await wrapper.find('input[type="date"]').setValue('2026-10-08')
    await wrapper.find('input[type="time"]').setValue('15:30')
    await findButton(wrapper, 'Save').trigger('click')
    await flushPromises()

    const payload = createMock.mock.calls[0][1]
    expect(payload.follow_up).toEqual({
      scheduled_at: new Date(2026, 9, 8, 15, 30).toISOString(),
    })
  })

  // A `next` outcome without a date logs the attempt alone — and the preview
  // says so before the save, so nothing is silent.
  it('creates no follow-up without a date and previews that', async () => {
    const wrapper = mountForm()
    await flushPromises()

    await pickTypeAndChip(wrapper, 'No Reply')
    expect(wrapper.text()).toContain('No follow-up will be created')

    await findButton(wrapper, 'Save').trigger('click')
    await flushPromises()

    const payload = createMock.mock.calls[0][1]
    expect(payload.follow_up).toBeUndefined()
    expect(payload.is_done).toBe(true)
  })

  // The preview states what the save will do.
  it('previews the all-day follow-up', async () => {
    const wrapper = mountForm()
    await flushPromises()

    await pickTypeAndChip(wrapper, 'No Reply')
    await wrapper.find('input[type="date"]').setValue('2026-10-08')
    await flushPromises()

    expect(wrapper.text()).toContain('all day')
    expect(wrapper.text()).toContain('9:00 AM')
  })

  // A second tap un-picks the chip: the save carries no quick reply.
  it('un-picks the outcome chip on a second tap', async () => {
    const wrapper = mountForm()
    await flushPromises()

    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'Call')
    await flushPromises()
    const chip = findButton(wrapper, 'No Reply')
    await chip.trigger('click')
    await chip.trigger('click')
    await findButton(wrapper, 'Save').trigger('click')
    await flushPromises()

    const payload = createMock.mock.calls[0][1]
    expect(payload.quick_reply_id).toBeNull()
  })

  // A close_lost outcome completes the attempt; the backend moves the lead.
  it('completes the attempt on a close_lost outcome', async () => {
    const wrapper = mountForm()
    await flushPromises()

    await pickTypeAndChip(wrapper, 'Closed Lost')
    await findButton(wrapper, 'Save').trigger('click')
    await flushPromises()

    const payload = createMock.mock.calls[0][1]
    expect(payload.is_done).toBe(true)
    expect(payload.quick_reply_id).toBe('q3')
    expect(payload.follow_up).toBeUndefined()
  })

  it('requires a type before saving', async () => {
    const wrapper = mountForm()
    await flushPromises()

    await findButton(wrapper, 'Save').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Type is required')
    expect(createMock).not.toHaveBeenCalled()
  })
})
