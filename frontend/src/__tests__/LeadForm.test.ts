import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import LeadForm from '@/components/leads/LeadForm.vue'
import { apiClient } from '@/composables/useApi'

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

let rbacCan = (permission: string) => true
vi.mock('@/stores/rbac', () => ({
  useRBACStore: () => ({
    can: (permission: string) => rbacCan(permission),
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

const getMock = vi.mocked(apiClient.get)

function makeStages() {
  return [
    { id: 's-open', pipeline_id: 'p1', name: 'New', order: 0, is_closing: false },
    { id: 's-closed', pipeline_id: 'p1', name: 'Closed Lost', order: 1, is_closing: true },
    { id: 's-won', pipeline_id: 'p1', name: 'Converted', order: 2, is_closing: true },
  ]
}

function mountForm(props: Record<string, any> = {}) {
  return mount(LeadForm, {
    props: {
      editingLead: null,
      stages: makeStages(),
      pipelineId: 'p1',
      ...props,
    },
    global: { plugins: [createPinia()] },
    attachTo: document.body,
  })
}

function optionLabels(wrapper: ReturnType<typeof mount>): string[] {
  return wrapper.findAll('[data-testid="select-item"]').map((o) => o.text()?.trim() ?? '')
}

describe('LeadForm', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    rbacCan = () => true
    getMock.mockReset()
    // onMounted fetches programs
    getMock.mockResolvedValue({ data: [] })
    document.body.innerHTML = ''
  })

  it('filters closing stages from the create-mode stage select', async () => {
    const wrapper = mountForm()
    await flushPromises()

    const labels = optionLabels(wrapper)
    expect(labels).toContain('New')
    expect(labels).not.toContain('Closed Lost')
    expect(labels).not.toContain('Converted')
    wrapper.unmount()
  })

  it('keeps closing stages visible when editing a lead', async () => {
    const wrapper = mountForm({
      editingLead: {
        id: 'l1',
        stage_id: 's-open',
        pipeline_id: 'p1',
        display_name: 'Alice',
        contact_id: 'c1',
      },
    })
    await flushPromises()

    const labels = optionLabels(wrapper)
    expect(labels).toContain('New')
    expect(labels).toContain('Closed Lost')
    expect(labels).toContain('Converted')
    wrapper.unmount()
  })

  it('calls resolve on new-contact save and shows the picker when a match exists', async () => {
    getMock.mockImplementation(async (url: string) => {
      if (url.startsWith('/api/contacts/resolve')) {
        return { data: [{ id: 'c1', name: 'Alice Example', phone: '9876543210' }] }
      }
      return { data: [] }
    })
    const wrapper = mountForm()
    await flushPromises()

    const newContactBtn = wrapper.findAll('button').find((b) => b.text() === 'New contact')
    expect(newContactBtn).toBeTruthy()
    await newContactBtn!.trigger('click')
    await wrapper.find('#nc-name').setValue('Alice Example')
    await wrapper.find('#nc-phone').setValue('98765 43210')
    await wrapper.find('#nc-email').setValue('alice@example.com')

    const createBtn = wrapper.findAll('button').find((b) => b.text() === 'Create')
    await createBtn!.trigger('click')
    await flushPromises()

    expect(getMock).toHaveBeenCalledWith(expect.stringContaining('/api/contacts/resolve'))
    expect(getMock).toHaveBeenCalledWith(expect.stringContaining('98765%2043210'))
    expect(wrapper.text()).toContain('Matching contacts')
    expect(wrapper.text()).toContain('Alice Example')

    // Linking the match exits new-contact mode and shows the linked banner.
    const linkBtn = wrapper.findAll('button').find((b) => b.text().includes('Link to Alice Example'))
    expect(linkBtn).toBeTruthy()
    await linkBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Linked to')
    wrapper.unmount()
  })

  it('emits new_contact when resolve returns no matches', async () => {
    getMock.mockImplementation(async (url: string) => {
      if (url.startsWith('/api/contacts/resolve')) {
        return { data: [] }
      }
      return { data: [] }
    })
    // Mount inside a host that binds @save — the black-box way to capture
    // the emitted payload.
    const saved: Record<string, any>[] = []
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
          @save="(b) => saved.push(b)"
        />
      `,
      setup() {
        return { saved }
      },
    }
    const wrapper = mount(Host, {
      global: { plugins: [createPinia()] },
      attachTo: document.body,
    })
    await flushPromises()

    const newContactBtn = wrapper.findAll('button').find((b) => b.text() === 'New contact')
    await newContactBtn!.trigger('click')
    await wrapper.find('#nc-name').setValue('Fresh Person')
    await wrapper.find('#nc-phone').setValue('9999999999')

    const createBtn = wrapper.findAll('button').find((b) => b.text() === 'Create')
    await createBtn!.trigger('click')
    await flushPromises()

    expect(getMock).toHaveBeenCalledWith(expect.stringContaining('/api/contacts/resolve'))
    expect(wrapper.text()).not.toContain('Matching contacts')
    expect(saved).toHaveLength(1)
    expect(saved[0].new_contact).toEqual({
      name: 'Fresh Person',
      phone: '9999999999',
      email: '',
    })
    wrapper.unmount()
  })

  it('disables the search box and shows the hint without contact:read', async () => {
    rbacCan = (permission: string) => permission !== 'contact:read'
    const wrapper = mountForm()
    await flushPromises()

    const input = wrapper.find('input[placeholder="Search by name, phone or email"]')
    expect(input.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('contact:read permission required to search contacts')
    wrapper.unmount()
  })
})
