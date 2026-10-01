import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import ScheduleTaskForm from '@/components/leads/ScheduleTaskForm.vue'
import { Select } from '@/components/ui/select'

vi.mock('@/api/leads', () => ({ createLeadActivity: vi.fn() }))
vi.mock('vue-sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    activityTypes: [{ id: 't1', name: 'Call' }],
    fetchTags: vi.fn(),
  }),
}))

import { createLeadActivity } from '@/api/leads'

const createMock = vi.mocked(createLeadActivity)

function mountForm() {
  return mount(ScheduleTaskForm, {
    props: { leadId: 'l1' },
    global: { plugins: [createPinia()] },
  })
}

function findButton(wrapper: ReturnType<typeof mount>, text: string) {
  const b = wrapper.findAll('button').find((x) => x.text().includes(text))
  expect(b, `button containing "${text}"`).toBeTruthy()
  return b!
}

// Schedule task creates an open task and nothing else: no is_done, no
// history entry — that is the What-happened form's job.
describe('ScheduleTaskForm', () => {
  beforeEach(() => {
    createMock.mockReset()
  })

  it('requires a type and a date before saving', async () => {
    const wrapper = mountForm()
    await flushPromises()

    await findButton(wrapper, 'Schedule').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Type is required')
    expect(createMock).not.toHaveBeenCalled()

    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'Call')
    await flushPromises()
    await findButton(wrapper, 'Schedule').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Date is required')
    expect(createMock).not.toHaveBeenCalled()
  })

  // A date without a time is the whole local day: start, end, and the 09:00
  // remind travel together.
  it('sends the whole local day when only a date is set', async () => {
    const wrapper = mountForm()
    await flushPromises()

    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'Call')
    await wrapper.find('input[type="date"]').setValue('2026-10-08')
    await findButton(wrapper, 'Schedule').trigger('click')
    await flushPromises()

    expect(createMock).toHaveBeenCalledTimes(1)
    const payload = createMock.mock.calls[0][1]
    expect(payload.type).toBe('Call')
    expect(payload.is_done).toBeUndefined()
    expect(payload.scheduled_at).toBe(new Date(2026, 9, 8).toISOString())
    expect(payload.scheduled_end_at).toBe(new Date(new Date(2026, 9, 9).getTime() - 1).toISOString())
    expect(payload.remind_at).toBe(new Date(2026, 9, 8, 9, 0, 0).toISOString())
  })

  it('sends only the start when a time is set', async () => {
    const wrapper = mountForm()
    await flushPromises()

    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'Call')
    await wrapper.find('input[type="date"]').setValue('2026-10-08')
    await wrapper.find('input[type="time"]').setValue('15:30')
    await findButton(wrapper, 'Schedule').trigger('click')
    await flushPromises()

    const payload = createMock.mock.calls[0][1]
    expect(payload.scheduled_at).toBe(new Date(2026, 9, 8, 15, 30).toISOString())
    expect(payload.scheduled_end_at).toBeUndefined()
    expect(payload.remind_at).toBeUndefined()
  })
})
