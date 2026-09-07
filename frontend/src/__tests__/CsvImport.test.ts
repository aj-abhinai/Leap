import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import CsvImport from '@/components/contacts/CsvImport.vue'
import { bulkImportContacts } from '@/api/contacts'

vi.mock('@/api/contacts', () => ({
  bulkImportContacts: vi.fn(),
}))

const { toast } = vi.hoisted(() => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('vue-sonner', () => ({ toast }))

const importMock = vi.mocked(bulkImportContacts)

function mountDialog() {
  return mount(CsvImport, {
    props: { open: true },
    global: {
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

// Drive the component into the result step through the same state transitions
// doImport relies on (headers/rows/step), then assert on both the API payload
// and the rendered panel.
async function importWithRows(wrapper: ReturnType<typeof mountDialog>, rows: string[][]) {
  const vm = wrapper.vm as any
  vm.headers = ['name', 'phone']
  vm.rows = rows
  vm.step = 'preview'
  await wrapper.vm.$nextTick()
  await vm.doImport()
  await flushPromises()
}

describe('CsvImport rejects panel', () => {
  beforeEach(() => {
    importMock.mockReset()
    toast.success.mockClear()
    toast.error.mockClear()
  })

  // The result step surfaces every rejected row with its reason in a
  // scrollable panel, and relies on the panel instead of a failure toast.
  it('renders rejected rows with their reasons', async () => {
    importMock.mockResolvedValue({
      data: {
        imported: 1,
        failed: 2,
        errors: [
          { row: 2, message: 'name is required' },
          { row: 3, message: 'phone matches an existing contact' },
        ],
      },
    })
    const wrapper = mountDialog()
    await importWithRows(wrapper, [['Alice', '9876543210'], ['Bob', ''], ['Carol', '9876543210']])

    // The API received the parsed rows — the panel content flows from the
    // import result, and the flow is genuinely exercised.
    expect(importMock).toHaveBeenCalledWith([
      { name: 'Alice', phone: '9876543210', tags: [] },
      { name: 'Bob', phone: undefined, tags: [] },
      { name: 'Carol', phone: '9876543210', tags: [] },
    ])

    const vm = wrapper.vm as any
    expect(vm.step).toBe('result')
    const html = wrapper.html()
    expect(html).toContain('Row 2: name is required')
    expect(html).toContain('Row 3: phone matches an existing contact')
    // The panel is the scrollable one the plan asks for.
    expect(html).toContain('max-h-40 overflow-y-auto')
    // The failed count is distinct from the imported count.
    expect(html).toContain('>2</div>')
    // No failure-summary toast: only the success toast may fire.
    expect(toast.error).not.toHaveBeenCalled()
    expect(toast.success).toHaveBeenCalledWith('Imported 1 contacts')
  })

  it('renders no rejected rows when nothing failed', async () => {
    importMock.mockResolvedValue({ data: { imported: 3, failed: 0, errors: [] } })
    const wrapper = mountDialog()
    await importWithRows(wrapper, [['Alice'], ['Bob'], ['Carol']])

    const html = wrapper.html()
    expect(html).toContain('>3</div>')
    expect(html).not.toContain('Row ')
    expect(toast.error).not.toHaveBeenCalled()
  })
})