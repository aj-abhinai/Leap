import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import CreateLeadsDialog from '@/components/contacts/CreateLeadsDialog.vue'
import { bulkCreateLeads } from '@/api/leads'
import { listPipelines, type Pipeline } from '@/api/pipelines'
import { listPrograms } from '@/api/programs'
import { listUserOptions } from '@/api/users'

vi.mock('@/api/leads', () => ({ bulkCreateLeads: vi.fn() }))
vi.mock('@/api/pipelines', () => ({ listPipelines: vi.fn() }))
vi.mock('@/api/programs', () => ({ listPrograms: vi.fn() }))
vi.mock('@/api/users', () => ({ listUserOptions: vi.fn() }))

const { toast } = vi.hoisted(() => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('vue-sonner', () => ({ toast }))

// The real ui/select renders through reka-ui's portal, which only mounts when
// the select is open — jsdom cannot open it. Stub the select module so the
// option slots render inline and are directly assertable.
vi.mock('@/components/ui/select', () => ({
  Select: { template: '<div><slot /></div>' },
  SelectTrigger: { template: '<button type="button"><slot /></button>' },
  SelectValue: { template: '<span><slot /></span>' },
  SelectContent: { template: '<div data-testid="select-content"><slot /></div>' },
  SelectItem: { template: '<div data-testid="select-item"><slot /></div>' },
}))

const pipelines: Pipeline[] = [
  {
    id: 'p1',
    name: 'Sales',
    stages: [
      { id: 's-open', pipeline_id: 'p1', name: 'New', order: 0, is_closing: false, outcome: 'open' },
      { id: 's-lost', pipeline_id: 'p1', name: 'Closed Lost', order: 1, is_closing: true, outcome: 'lost' },
    ],
  },
  {
    id: 'p2',
    name: 'Renewals',
    stages: [{ id: 's2', pipeline_id: 'p2', name: 'Start', order: 0, is_closing: false, outcome: 'open' }],
  },
]

let pinia: Pinia

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(CreateLeadsDialog, {
    props: { open: true, contactIds: ['c1', 'c2'], ...props },
    global: {
      plugins: [pinia],
      stubs: {
        Dialog: { template: '<div><slot /></div>' },
        DialogContent: { template: '<div><slot /></div>' },
        DialogHeader: { template: '<div><slot /></div>' },
        DialogTitle: { template: '<div><slot /></div>' },
        DialogDescription: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('CreateLeadsDialog', () => {
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.mocked(bulkCreateLeads).mockReset()
    vi.mocked(listPipelines).mockReset().mockResolvedValue({ data: pipelines })
    vi.mocked(listPrograms).mockReset().mockResolvedValue({
      data: [{ id: 'prog1', name: 'Coaching', price: 25000, archived: false }],
    })
    vi.mocked(listUserOptions).mockReset().mockResolvedValue({ data: [{ id: 'u1', name: 'Alice User' }] })
    toast.success.mockClear()
    toast.error.mockClear()
  })

  it('defaults to the first pipeline, its first open stage, no program, and unassigned', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    const vm = wrapper.vm as any
    expect(vm.formPipelineId).toBe('p1')
    expect(vm.formStageId).toBe('s-open')
    expect(vm.formProgramId).toBe('__none__')
    expect(vm.formAssignedTo).toBe('__unassigned__')

    const html = wrapper.html()
    expect(html).toContain('Create leads for 2 contacts')
    expect(html).toContain('New')
    // Closing stages are unreachable at create and must not be offered.
    expect(html).not.toContain('Closed Lost')
  })

  it('surfaces a pipeline load failure instead of a silent disabled form', async () => {
    vi.mocked(listPipelines).mockReset().mockRejectedValue(new Error('network down'))
    const wrapper = mountDialog()
    await flushPromises()

    const vm = wrapper.vm as any
    expect(vm.formPipelineId).toBe('')
    expect(vm.error).toContain('network down')
    expect(wrapper.html()).toContain('network down')
    expect(bulkCreateLeads).not.toHaveBeenCalled()
  })

  it('resets the stage when the pipeline changes', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    const vm = wrapper.vm as any
    vm.formPipelineId = 'p2'
    await wrapper.vm.$nextTick()
    expect(vm.formStageId).toBe('s2')
  })

  it('submits the shared fields and renders the per-row report', async () => {
    vi.mocked(bulkCreateLeads).mockResolvedValue({
      data: {
        created: 1,
        skipped: 1,
        failed: 1,
        errors: [
          { contact_id: 'c2', name: 'Bob', outcome: 'skipped', message: 'already has an open deal: New · Coaching · Sales' },
          { contact_id: 'c3', name: 'Carol', outcome: 'failed', message: 'contact not found or deleted' },
        ],
      },
    })
    const wrapper = mountDialog({ contactIds: ['c1', 'c2', 'c3'] })
    await flushPromises()
    await (wrapper.vm as any).submit()
    await flushPromises()

    // None/unassigned sentinels travel as empty strings.
    expect(bulkCreateLeads).toHaveBeenCalledWith({
      contact_ids: ['c1', 'c2', 'c3'],
      pipeline_id: 'p1',
      stage_id: 's-open',
      program_id: '',
      assigned_to: '',
    })

    const vm = wrapper.vm as any
    expect(vm.step).toBe('result')
    const html = wrapper.html()
    expect(html).toContain('Created')
    expect(html).toContain('Skipped')
    expect(html).toContain('Failed')
    expect(html).toContain('Skipped:')
    expect(html).toContain('Bob')
    expect(html).toContain('already has an open deal: New · Coaching · Sales')
    expect(html).toContain('Failed:')
    expect(html).toContain('Carol')
    expect(html).toContain('contact not found or deleted')
    // The per-row report is the scrollable panel, like the import result.
    expect(html).toContain('max-h-40 overflow-y-auto')
    expect(toast.success).toHaveBeenCalledWith('Created 1 leads')
  })

  it('keeps the form open and shows the refusal when the run is rejected', async () => {
    vi.mocked(bulkCreateLeads).mockRejectedValue(new Error('stage_id does not belong to the pipeline'))
    const wrapper = mountDialog()
    await flushPromises()

    await (wrapper.vm as any).submit()
    await flushPromises()

    const vm = wrapper.vm as any
    expect(vm.step).toBe('form')
    expect(vm.error).toBe('stage_id does not belong to the pipeline')
    expect(wrapper.html()).toContain('stage_id does not belong to the pipeline')
    expect(toast.success).not.toHaveBeenCalled()
  })

  it('refuses to close while a run is in flight, then closes after it settles', async () => {
    let resolveRun!: (value: unknown) => void
    vi.mocked(bulkCreateLeads).mockReturnValue(
      new Promise((resolve) => {
        resolveRun = resolve
      }) as any,
    )
    const wrapper = mountDialog()
    await flushPromises()

    const vm = wrapper.vm as any
    const run = vm.submit()
    await wrapper.vm.$nextTick()
    expect(vm.submitting).toBe(true)

    // The X, Esc, and overlay all route through close(); all must be refused
    // while the report is still pending, or the result is lost and a re-run
    // turns into a wall of skips.
    vm.close()
    expect(wrapper.emitted('close')).toBeFalsy()
    const cancel = wrapper.findAll('button').find((b) => b.text() === 'Cancel')
    expect(cancel).toBeTruthy()
    expect((cancel!.element as HTMLButtonElement).disabled).toBe(true)

    resolveRun({ data: { created: 2, skipped: 0, failed: 0 } })
    await run
    await flushPromises()
    expect((wrapper.vm as any).step).toBe('result')

    vm.close()
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('emits done after the result step is acknowledged', async () => {
    vi.mocked(bulkCreateLeads).mockResolvedValue({ data: { created: 2, skipped: 0, failed: 0 } })
    const wrapper = mountDialog()
    await flushPromises()
    await (wrapper.vm as any).submit()
    await flushPromises()

    const done = wrapper.findAll('button').find((b) => b.text() === 'Done')
    expect(done).toBeTruthy()
    await done!.trigger('click')
    expect(wrapper.emitted('done')).toBeTruthy()
  })
})
