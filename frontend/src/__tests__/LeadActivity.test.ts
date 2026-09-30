import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import LeadActivity from '@/components/leads/LeadActivity.vue'
import type { LeadActivity as Activity } from '@/api/leads'

vi.mock('@/api/leads', () => ({
  listLeadActivities: vi.fn(),
  updateLeadActivity: vi.fn(),
  deleteLeadActivity: vi.fn(),
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    activityTypes: [],
    quickReplies: [],
    fetchTags: vi.fn(),
  }),
}))

const { toast } = vi.hoisted(() => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('vue-sonner', () => ({ toast }))

import { listLeadActivities } from '@/api/leads'

const listMock = vi.mocked(listLeadActivities)

const HOUR = 60 * 60 * 1000
const past = new Date(Date.now() - 2 * HOUR).toISOString()
const future = new Date(Date.now() + 2 * HOUR).toISOString()

function makeActivity(overrides: Partial<Activity>): Activity {
  return {
    id: 'a1',
    lead_id: 'l1',
    type: 'Call',
    description: '',
    is_done: false,
    is_cancelled: false,
    is_reminded: false,
    created_at: past,
    ...overrides,
  }
}

// A cancelled-never-done task with a past schedule is the regression case: it
// used to fall into the actionable buckets; it belongs to History.
const fixtures: Activity[] = [
  makeActivity({ id: 'a-cancelled', description: 'cancelled-item', scheduled_at: past, is_cancelled: true }),
  makeActivity({ id: 'a-done', description: 'done-item', occurred_at: past, is_done: true }),
  makeActivity({ id: 'a-upcoming', description: 'upcoming-item', scheduled_at: future }),
]

function mountActivity() {
  return mount(LeadActivity, {
    props: { leadId: 'l1' },
    global: { plugins: [createPinia()] },
    attachTo: document.body,
  })
}

function sectionText(wrapper: ReturnType<typeof mountActivity>, title: string): string {
  const section = wrapper.findAll('section').find((s) => s.find('h3').exists() && s.find('h3').text() === title)
  return section?.text() ?? ''
}

describe('LeadActivity section classification', () => {
  beforeEach(() => {
    listMock.mockReset().mockResolvedValue({ data: fixtures })
    document.body.innerHTML = ''
  })

  it('renders a cancelled-never-done task struck through in History only', async () => {
    const wrapper = mountActivity()
    await flushPromises()

    const cancelledDescription = wrapper.findAll('p').find((p) => p.text() === 'cancelled-item')
    expect(cancelledDescription).toBeTruthy()
    const cancelledCard = cancelledDescription!.element.closest('.opacity-60')
    expect(cancelledCard).not.toBeNull()
    expect(cancelledCard!.querySelector('.line-through')).not.toBeNull()
    expect(cancelledCard!.textContent).toContain('Cancelled')

    const historyText = sectionText(wrapper, 'History')
    expect(historyText).toContain('cancelled-item')
    expect(sectionText(wrapper, 'Upcoming')).not.toContain('cancelled-item')
    expect(sectionText(wrapper, 'Overdue')).not.toContain('cancelled-item')

    // The other fixtures still land in their sections.
    expect(historyText).toContain('done-item')
    expect(sectionText(wrapper, 'Upcoming')).toContain('upcoming-item')

    wrapper.unmount()
  })

  it('shows no Overdue section when the only past task was cancelled', async () => {
    const wrapper = mountActivity()
    await flushPromises()

    expect(wrapper.text()).not.toContain('Overdue')

    wrapper.unmount()
  })
})
