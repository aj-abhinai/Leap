import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import LeadActivityForm from '@/components/leads/LeadActivityForm.vue'
import { Select } from '@/components/ui/select'

vi.mock('@/api/leads', () => ({ createLeadActivity: vi.fn() }))
vi.mock('@/api/settings', () => ({
  getNudgeLeadMinutes: vi.fn(() => Promise.resolve({ data: { minutes: 5 } })),
}))
vi.mock('vue-sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    activityTypes: [{ id: 't1', name: 'Call' }],
    quickReplies: [
      { id: 'q1', name: 'Share Details', group_name: 'Connected', sort_order: 0, behavior: 'log' },
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

describe('LeadActivityForm', () => {
  beforeEach(() => {
    localStorage.clear()
    createMock.mockReset()
  })

  // A picked quick reply already means the attempt happened: the schedule
  // fields have nothing left to describe and hide, like next and close_lost.
  it('hides the schedule fields after a quick reply is picked', async () => {
    const wrapper = mountForm()
    await flushPromises()

    const more = wrapper.findAll('button').find((b) => b.text().includes('More options'))
    expect(more).toBeTruthy()
    await more!.trigger('click')
    expect(wrapper.text()).toContain('Scheduled date')

    const chip = wrapper.findAll('button').find((b) => b.text().includes('Share Details'))
    expect(chip).toBeTruthy()
    await chip!.trigger('click')

    expect(wrapper.text()).not.toContain('Scheduled date')
  })

  // The hidden fields must not submit stale values: a reply-only save carries
  // the reply and no schedule or reminder.
  it('submits the quick reply without a schedule after the fields hide', async () => {
    const wrapper = mountForm()
    await flushPromises()

    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'Call')
    await flushPromises()

    const chip = wrapper.findAll('button').find((b) => b.text().includes('Share Details'))
    expect(chip).toBeTruthy()
    await chip!.trigger('click')

    const save = wrapper.findAll('button').find((b) => b.text() === 'Save')
    expect(save).toBeTruthy()
    await save!.trigger('click')
    await flushPromises()

    expect(createMock).toHaveBeenCalledTimes(1)
    const payload = createMock.mock.calls[0][1]
    expect(payload.quick_reply_id).toBe('q1')
    expect(payload.scheduled_at).toBeUndefined()
    expect(payload.remind_at).toBeUndefined()
  })
})
