import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import LeadForm from '@/components/leads/LeadForm.vue'
import { apiClient } from '@/composables/useApi'
import type { Stage } from '@/api/pipelines'
import type { OpenLeadRef } from '@/api/leads'

vi.mock('@/composables/useApi', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  },
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    lossReasons: [],
    fetchTags: vi.fn(),
  }),
}))

vi.mock('@/stores/rbac', () => ({
  useRBACStore: () => ({
    can: () => true,
  }),
}))

// The real ui/select renders through reka-ui's portal, which only mounts
// when the select is open — jsdom cannot open it. Stub the select module so
// the option slots render inline and are directly assertable.
vi.mock('@/components/ui/select', () => ({
  Select: { template: '<div><slot /></div>' },
  SelectTrigger: { template: '<button type="button"><slot /></button>' },
  SelectValue: { template: '<span><slot /></span>' },
  SelectContent: { template: '<div data-testid="select-content"><slot /></div>' },
  SelectItem: { template: '<div data-testid="select-item"><slot /></div>' },
}))

vi.mock('vue-sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const getMock = vi.mocked(apiClient.get)
const postMock = vi.mocked(apiClient.post)

const openLead: OpenLeadRef = {
  id: 'l1',
  display_name: 'Deal One',
  stage_name: 'New',
  program_name: 'Coaching',
  pipeline_id: 'p1',
  pipeline_name: 'Test Pipeline',
}

function makeStages(): Stage[] {
  return [
    { id: 's-open', pipeline_id: 'p1', name: 'New', order: 0, is_closing: false, outcome: 'open' },
  ]
}

function mountForm(props: Record<string, any> = {}, onEnquiryLogged?: () => void) {
  const Host = {
    components: { LeadForm },
    data() {
      return { stages: makeStages() }
    },
    template: `
      <LeadForm
        :editing-lead="null"
        :stages="stages"
        pipeline-id="p1"
        ${props.openLeadConflict !== undefined ? ':open-lead-conflict="conflict"' : ''}
        ${onEnquiryLogged ? '@enquiry-logged="onEnquiryLogged"' : ''}
      />
    `,
    setup() {
      return { conflict: props.openLeadConflict ?? null, onEnquiryLogged }
    },
  }
  return mount(Host, {
    global: { plugins: [createPinia()] },
    attachTo: document.body,
  })
}

async function fillNewContactAndLink(wrapper: ReturnType<typeof mount>) {
  const newContactBtn = wrapper.findAll('button').find((b) => b.text() === 'New contact')
  await newContactBtn!.trigger('click')
  await wrapper.find('#nc-name').setValue('Alice Example')
  await wrapper.find('#nc-phone').setValue('98765 43210')
  const createBtn = wrapper.findAll('button').find((b) => b.text() === 'Create')
  await createBtn!.trigger('click')
  await flushPromises()
  const linkBtn = wrapper.findAll('button').find((b) => b.text().includes('Link to Alice Example'))
  expect(linkBtn).toBeTruthy()
  await linkBtn!.trigger('click')
  await flushPromises()
}

describe('LeadForm resolve-or-log banner', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getMock.mockReset()
    postMock.mockReset()
    getMock.mockResolvedValue({ data: [] })
    postMock.mockResolvedValue({ data: { id: 'a1' } })
    document.body.innerHTML = ''
  })

  it('renders the banner with the existing open lead after linking a resolve match', async () => {
    getMock.mockImplementation(async (url: string) => {
      if (url.startsWith('/api/contacts/resolve')) {
        return {
          data: [{
            id: 'c1',
            name: 'Alice Example',
            phone: '9876543210',
            open_leads: [openLead],
          }],
        }
      }
      return { data: [] }
    })
    const wrapper = mountForm()
    await flushPromises()
    await fillNewContactAndLink(wrapper)

    expect(wrapper.text()).toContain('Deal One has an open deal')
    expect(wrapper.text()).toContain('New · Coaching · Test Pipeline')
    expect(wrapper.text()).toContain('Log enquiry')
    expect(wrapper.text()).toContain('View lead')
    wrapper.unmount()
  })

  it('logs exactly one done Enquiry activity on the existing lead', async () => {
    getMock.mockImplementation(async (url: string) => {
      if (url.startsWith('/api/contacts/resolve')) {
        return {
          data: [{
            id: 'c1',
            name: 'Alice Example',
            phone: '9876543210',
            open_leads: [openLead],
          }],
        }
      }
      return { data: [] }
    })
    let logged = 0
    const wrapper = mountForm({}, () => { logged += 1 })
    await flushPromises()
    await fillNewContactAndLink(wrapper)

    const logBtn = wrapper.findAll('button').find((b) => b.text() === 'Log enquiry')
    await logBtn!.trigger('click')
    await flushPromises()

    expect(postMock).toHaveBeenCalledTimes(1)
    expect(postMock).toHaveBeenCalledWith(`/api/leads/${openLead.id}/activities`, {
      type: 'Enquiry',
      is_done: true,
    })
    expect(logged).toBe(1)
    wrapper.unmount()
  })

  it('renders the same banner from a refused create (409 backstop)', async () => {
    const wrapper = mountForm({ openLeadConflict: openLead })
    await flushPromises()

    expect(wrapper.text()).toContain('Deal One has an open deal')
    expect(wrapper.text()).toContain('New · Coaching · Test Pipeline')
    const logBtn = wrapper.findAll('button').find((b) => b.text() === 'Log enquiry')
    expect(logBtn).toBeTruthy()
    wrapper.unmount()
  })

  it('offers program selection when the contact holds several open leads', async () => {
    const secondLead: OpenLeadRef = {
      ...openLead,
      id: 'l2',
      display_name: 'Deal Two',
      program_name: 'Mentorship',
    }
    getMock.mockImplementation(async (url: string) => {
      if (url.startsWith('/api/contacts/resolve')) {
        return {
          data: [{
            id: 'c1',
            name: 'Alice Example',
            phone: '9876543210',
            open_leads: [openLead, secondLead],
          }],
        }
      }
      return { data: [] }
    })
    const wrapper = mountForm()
    await flushPromises()
    await fillNewContactAndLink(wrapper)

    expect(wrapper.text()).toContain('Open deals exist for this contact')
    const options = wrapper.findAll('[data-testid="select-item"]').map((o) => o.text()?.trim() ?? '')
    expect(options).toContain('Deal One · Coaching')
    expect(options).toContain('Deal Two · Mentorship')

    // The one-tap log targets the first (oldest) open lead by default.
    const logBtn = wrapper.findAll('button').find((b) => b.text() === 'Log enquiry')
    await logBtn!.trigger('click')
    await flushPromises()
    expect(postMock).toHaveBeenCalledWith(`/api/leads/${openLead.id}/activities`, expect.any(Object))
    wrapper.unmount()
  })
})