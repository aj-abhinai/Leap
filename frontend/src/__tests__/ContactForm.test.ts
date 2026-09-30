import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ContactForm from '@/components/contacts/ContactForm.vue'
import type { Contact, DuplicateMatch } from '@/api/contacts'

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    tags: [],
    statuses: [],
    fetchTags: vi.fn(),
  }),
}))

// The real select and alert dialog render through reka-ui's portal, which
// jsdom cannot open. The stubs render the same content inline so the dialog
// copy and the props that drive it are directly assertable.
vi.mock('@/components/ui/select', () => ({
  Select: { template: '<div><slot /></div>' },
  SelectTrigger: { template: '<button type="button"><slot /></button>' },
  SelectValue: { template: '<span><slot /></span>' },
  SelectContent: { template: '<div><slot /></div>' },
  SelectItem: { template: '<div><slot /></div>' },
}))

vi.mock('@/components/ui/alert-dialog', () => ({
  AlertDialog: { props: ['open'], template: '<div v-if="open"><slot /></div>' },
  AlertDialogContent: { template: '<div><slot /></div>' },
  AlertDialogHeader: { template: '<div><slot /></div>' },
  AlertDialogTitle: { template: '<h2><slot /></h2>' },
  AlertDialogDescription: { template: '<p><slot /></p>' },
  AlertDialogFooter: { template: '<div><slot /></div>' },
  AlertDialogCancel: { template: '<button><slot /></button>' },
  AlertDialogAction: { template: '<button><slot /></button>' },
}))

function makeContact(): Contact {
  return {
    id: 'c1',
    name: 'Alice Example',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  }
}

const duplicateMatches: DuplicateMatch[] = [
  { id: 'c2', name: 'Existing Person', phone: '+919876543210', email: 'existing@example.com' },
]

function mountForm(props: Record<string, any> = {}) {
  return mount(ContactForm, {
    props: {
      editingContact: null,
      duplicateMatches,
      ...props,
    },
  })
}

describe('ContactForm duplicate dialog copy', () => {
  it('says Save anyway when the form is editing a contact', async () => {
    const wrapper = mountForm({ editingContact: makeContact() })
    await flushPromises()

    expect(wrapper.text()).toContain('Duplicate contact')
    expect(wrapper.text()).toContain('Save anyway?')
    expect(wrapper.text()).not.toContain('Create anyway')

    const action = wrapper.findAll('button').find((b) => b.text() === 'Save anyway')
    expect(action).toBeTruthy()
    await action!.trigger('click')
    expect(wrapper.emitted('confirm-duplicate')).toEqual([[true]])

    wrapper.unmount()
  })

  it('says Create anyway when the form is creating a contact', async () => {
    const wrapper = mountForm()
    await flushPromises()

    expect(wrapper.text()).toContain('Create anyway?')
    expect(wrapper.text()).not.toContain('Save anyway')

    wrapper.unmount()
  })
})
