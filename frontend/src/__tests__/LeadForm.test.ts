import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import LeadForm from '@/components/leads/LeadForm.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import type { Stage } from '@/api/pipelines'
import { apiClient } from '@/composables/useApi'
import { useAuthStore } from '@/stores/auth'
import { loadRememberedCreateValues, rememberCreateValues, rememberedStageId } from '@/composables/useLeadFormDefaults'

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

function makeStages(): Stage[] {
  return [
    { id: 's-open', pipeline_id: 'p1', name: 'New', order: 0, is_closing: false, outcome: 'open' },
    { id: 's-contacted', pipeline_id: 'p1', name: 'Contacted', order: 1, is_closing: false, outcome: 'open' },
    { id: 's-closed', pipeline_id: 'p1', name: 'Closed Lost', order: 2, is_closing: true, outcome: 'lost' },
    { id: 's-won', pipeline_id: 'p1', name: 'Converted', order: 3, is_closing: true, outcome: 'won' },
  ]
}

function mountForm(props: Record<string, any> = {}, pinia?: Pinia) {
  return mount(LeadForm, {
    props: {
      editingLead: null,
      stages: makeStages(),
      pipelineId: 'p1',
      ...props,
    },
    global: { plugins: [pinia ?? createPinia()], stubs: { ConfirmDialog: true } },
    attachTo: document.body,
  })
}

function optionLabels(wrapper: ReturnType<typeof mount>): string[] {
  return wrapper.findAll('[data-testid="select-item"]').map((o) => o.text()?.trim() ?? '')
}

// submitNewContact fills the inline new-contact branch and clicks Create — the
// shortest path to an emitted save.
async function submitNewContact(wrapper: ReturnType<typeof mount>) {
  const newContactBtn = wrapper.findAll('button').find((b) => b.text() === 'New contact')
  await newContactBtn!.trigger('click')
  await wrapper.find('#nc-name').setValue('Fresh Person')
  await wrapper.find('#nc-phone').setValue('9999999999')
  const createBtn = wrapper.findAll('button').find((b) => b.text() === 'Create')
  await createBtn!.trigger('click')
  await flushPromises()
}

// mockPickers answers the program catalog and the assignee options the form
// loads on mount.
function mockPickers(programs: Record<string, unknown>[], users: Record<string, unknown>[]) {
  getMock.mockImplementation(async (url: string) => {
    if (url.startsWith('/api/programs')) return { data: programs }
    if (url.startsWith('/api/users/options')) return { data: users }
    return { data: [] }
  })
}

describe('LeadForm', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    rbacCan = () => true
    getMock.mockReset()
    // onMounted fetches programs
    getMock.mockResolvedValue({ data: [] })
    localStorage.clear()
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

  it('blocks a digitless new-contact phone before the resolve call', async () => {
    const wrapper = mountForm()
    await flushPromises()

    const newContactBtn = wrapper.findAll('button').find((b) => b.text() === 'New contact')
    await newContactBtn!.trigger('click')
    await wrapper.find('#nc-name').setValue('Fresh Person')
    await wrapper.find('#nc-phone').setValue('abc')
    await wrapper.find('#nc-email').setValue('fresh@example.com')

    const createBtn = wrapper.findAll('button').find((b) => b.text() === 'Create')
    await createBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Phone must contain at least one digit')
    expect(getMock).not.toHaveBeenCalledWith(expect.stringContaining('/api/contacts/resolve'))
    expect(wrapper.emitted('save')).toBeUndefined()
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

  it('asks before a closing edit and saves on confirm', async () => {
    const wrapper = mountForm({
      editingLead: {
        id: 'l1',
        stage_id: 's-closed',
        stage_outcome: 'open',
        pipeline_id: 'p1',
        display_name: 'Alice',
        contact_id: 'c1',
      },
    })
    await flushPromises()

    const updateBtn = wrapper.findAll('button').find((b) => b.text() === 'Update')
    expect(updateBtn).toBeTruthy()
    await updateBtn!.trigger('click')
    await flushPromises()

    const dialog = wrapper.findComponent(ConfirmDialog)
    expect(dialog.props('open')).toBe(true)
    expect(dialog.props('title')).toContain('Closed Lost')
    expect(wrapper.emitted('save')).toBeUndefined()

    dialog.vm.$emit('confirm')
    await nextTick()

    expect(wrapper.emitted('save')).toHaveLength(1)
    expect(wrapper.emitted('save')![0][0]).toMatchObject({ stage_id: 's-closed', contact_id: 'c1' })
    expect(dialog.props('open')).toBe(false)
    wrapper.unmount()
  })

  it('seeds a create from the remembered program, assignee, and stage', async () => {
    rememberCreateValues(undefined, 'p1', {
      programId: 'prog1',
      assignedTo: 'u1',
      stageId: 's-contacted',
    })
    mockPickers([{ id: 'prog1', name: 'Coaching', price: 25000 }], [{ id: 'u1', name: 'Alice User' }])

    const wrapper = mountForm()
    await flushPromises()

    const vm = wrapper.vm as any
    expect(vm.formProgramId).toBe('prog1')
    expect(vm.formAssignedTo).toBe('u1')
    expect(vm.formStageId).toBe('s-contacted')

    await submitNewContact(wrapper)
    expect(wrapper.emitted('save')![0][0]).toMatchObject({
      program_id: 'prog1',
      assigned_to: 'u1',
      stage_id: 's-contacted',
    })
    wrapper.unmount()
  })

  it('scopes the memory to the signed-in user', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useAuthStore().user = { id: 'u1', name: 'Rep One' } as any
    rememberCreateValues('u1', 'p1', { programId: 'prog1', assignedTo: '', stageId: 's-contacted' })
    rememberCreateValues('u2', 'p1', { programId: 'prog2', assignedTo: '', stageId: 's-open' })
    mockPickers([{ id: 'prog1', name: 'Coaching', price: 25000 }], [])

    const wrapper = mountForm({}, pinia)
    await flushPromises()

    const vm = wrapper.vm as any
    expect(vm.formProgramId).toBe('prog1')
    expect(vm.formStageId).toBe('s-contacted')
    wrapper.unmount()
  })

  it('drops a remembered program that is no longer in the catalog', async () => {
    rememberCreateValues(undefined, 'p1', { programId: 'gone', assignedTo: '', stageId: '' })
    mockPickers([{ id: 'prog1', name: 'Coaching', price: 25000 }], [])

    const wrapper = mountForm()
    await flushPromises()

    expect((wrapper.vm as any).formProgramId).toBe('__none__')
    await submitNewContact(wrapper)
    expect(wrapper.emitted('save')![0][0]).toMatchObject({ program_id: '' })
    wrapper.unmount()
  })

  it('validates a remembered program before the create submits', async () => {
    rememberCreateValues(undefined, 'p1', { programId: 'gone', assignedTo: '', stageId: '' })
    let resolvePrograms!: (value: { data: unknown[] }) => void
    const programsPending = new Promise<{ data: unknown[] }>((resolve) => {
      resolvePrograms = resolve
    })
    getMock.mockImplementation(async (url: string) => {
      if (url.startsWith('/api/programs')) return programsPending
      return { data: [] }
    })

    const wrapper = mountForm()
    await flushPromises()

    // Fill a new contact and click Create while the catalog is still open: the
    // save must wait for the list that validates the remembered program.
    const newContactBtn = wrapper.findAll('button').find((b) => b.text() === 'New contact')
    await newContactBtn!.trigger('click')
    await wrapper.find('#nc-name').setValue('Fresh Person')
    await wrapper.find('#nc-phone').setValue('9999999999')
    const createBtn = wrapper.findAll('button').find((b) => b.text() === 'Create')
    await createBtn!.trigger('click')
    await flushPromises()
    expect(wrapper.emitted('save')).toBeUndefined()

    resolvePrograms({ data: [{ id: 'prog1', name: 'Coaching', price: 25000 }] })
    await flushPromises()

    expect(wrapper.emitted('save')![0][0]).toMatchObject({ program_id: '' })
    wrapper.unmount()
  })

  it('submits once when Create is clicked twice', async () => {
    const wrapper = mountForm()
    await flushPromises()

    const newContactBtn = wrapper.findAll('button').find((b) => b.text() === 'New contact')
    await newContactBtn!.trigger('click')
    await wrapper.find('#nc-name').setValue('Fresh Person')
    await wrapper.find('#nc-phone').setValue('9999999999')

    const createBtn = wrapper.findAll('button').find((b) => b.text() === 'Create')
    await createBtn!.trigger('click')
    // The save still waits on the phone resolve here; the second click must
    // not start a second run.
    await createBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.emitted('save')).toHaveLength(1)
    wrapper.unmount()
  })

  it('ignores a remembered stage that is closing or from another pipeline', async () => {
    rememberCreateValues(undefined, 'p1', { programId: '', assignedTo: '', stageId: 's-closed' })
    const closing = mountForm()
    await flushPromises()
    expect((closing.vm as any).formStageId).toBe('s-open')
    closing.unmount()

    rememberCreateValues(undefined, 'p1', { programId: '', assignedTo: '', stageId: 'elsewhere' })
    const foreign = mountForm()
    await flushPromises()
    expect((foreign.vm as any).formStageId).toBe('s-open')
    foreign.unmount()
  })

  it('keeps the clicked column stage ahead of the remembered one', async () => {
    rememberCreateValues(undefined, 'p1', { programId: '', assignedTo: '', stageId: 's-open' })

    const wrapper = mountForm({ initialStageId: 's-contacted' })
    await flushPromises()

    expect((wrapper.vm as any).formStageId).toBe('s-contacted')
    wrapper.unmount()
  })

  it('never seeds an edit from the remembered values', async () => {
    rememberCreateValues(undefined, 'p1', {
      programId: 'prog1',
      assignedTo: 'u1',
      stageId: 's-contacted',
    })
    mockPickers(
      [{ id: 'prog1', name: 'Coaching', price: 25000 }],
      [{ id: 'u1', name: 'Alice User' }, { id: 'u9', name: 'Lead Owner' }],
    )

    const wrapper = mountForm({
      editingLead: {
        id: 'l1',
        stage_id: 's-open',
        pipeline_id: 'p1',
        display_name: 'Alice',
        contact_id: 'c1',
        program_id: 'prog-lead',
        assigned_to: 'u9',
      },
    })
    await flushPromises()

    const vm = wrapper.vm as any
    expect(vm.formProgramId).toBe('prog-lead')
    expect(vm.formAssignedTo).toBe('u9')
    expect(vm.formStageId).toBe('s-open')
    wrapper.unmount()
  })

  it('resets a remembered assignee that is no longer listed', async () => {
    rememberCreateValues(undefined, 'p1', { programId: '', assignedTo: 'u-gone', stageId: '' })
    mockPickers([], [{ id: 'u1', name: 'Alice User' }])

    const wrapper = mountForm()
    await flushPromises()

    expect((wrapper.vm as any).formAssignedTo).toBe('__unassigned__')
    wrapper.unmount()
  })

  it('remembers what a create submits for the next entry', async () => {
    const wrapper = mountForm()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.formProgramId = 'prog1'
    vm.formAssignedTo = 'u1'
    vm.formStageId = 's-contacted'
    await nextTick()
    await submitNewContact(wrapper)

    expect(loadRememberedCreateValues(undefined)).toEqual({ programId: 'prog1', assignedTo: 'u1' })
    expect(rememberedStageId(undefined, 'p1', makeStages())).toBe('s-contacted')
    expect(wrapper.emitted('save')![0][0]).toMatchObject({
      program_id: 'prog1',
      stage_id: 's-contacted',
    })
    wrapper.unmount()
  })
})
