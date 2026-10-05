import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useContactSave } from '@/composables/useContactSave'
import { ApiError } from '@/composables/useApi'
import * as api from '@/api/contacts'
import type { Contact, ContactSaveBody, DuplicateMatch } from '@/api/contacts'

const { toast } = vi.hoisted(() => ({ toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() } }))
vi.mock('vue-sonner', () => ({ toast }))
vi.mock('@/api/contacts', () => ({
  createContact: vi.fn(),
  updateContact: vi.fn(),
}))

function contact(over: Partial<Contact> = {}): Contact {
  return {
    id: 'c1',
    name: 'Alice',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...over,
  }
}

function body(over: Partial<ContactSaveBody> = {}): ContactSaveBody {
  return {
    name: 'Alice',
    nickname: '',
    location: '',
    age: null,
    date_of_birth: '',
    tag_ids: [],
    status_id: '',
    phones: [{ value: '9876543210', is_primary: true }],
    emails: [],
    phone: '9876543210',
    email: '',
    ...over,
  }
}

// duplicateError builds the thrown value exactly as the transport does: an
// ApiError whose payload is the response envelope, with the matches under
// data (the same nesting the lead-side conflict parser reads).
function duplicateError(matches: DuplicateMatch[]) {
  return new ApiError('A contact with this phone or email already exists', 'DUPLICATE_CONTACT', {
    data: { duplicates: matches },
    error: { code: 'DUPLICATE_CONTACT', message: 'A contact with this phone or email already exists' },
  })
}

describe('useContactSave', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  it('creates a contact and returns the saved row', async () => {
    const saved = contact()
    vi.mocked(api.createContact).mockResolvedValue({ data: saved })
    const { saving, duplicateMatches, save } = useContactSave()

    const result = await save(body())

    expect(result).toBe(saved)
    expect(api.createContact).toHaveBeenCalledWith(body())
    expect(api.updateContact).not.toHaveBeenCalled()
    expect(saving.value).toBe(false)
    expect(duplicateMatches.value).toBeNull()
  })

  it('updates a contact when an id is given', async () => {
    const saved = contact({ name: 'Alice Two' })
    vi.mocked(api.updateContact).mockResolvedValue({ data: saved })
    const { save } = useContactSave()

    const result = await save(body({ name: 'Alice Two' }), 'c1')

    expect(result).toBe(saved)
    expect(api.updateContact).toHaveBeenCalledWith('c1', body({ name: 'Alice Two' }))
    expect(api.createContact).not.toHaveBeenCalled()
  })

  it('surfaces the matched contacts when a create is refused with 409', async () => {
    const matches: DuplicateMatch[] = [{ id: 'x1', name: 'Existing', phone: '+919876543210' }]
    vi.mocked(api.createContact).mockRejectedValue(duplicateError(matches))
    const { saving, duplicateMatches, save } = useContactSave()

    const result = await save(body())

    expect(result).toBeNull()
    expect(duplicateMatches.value).toEqual(matches)
    expect(saving.value).toBe(false)
    expect(toast.error).not.toHaveBeenCalled()
  })

  it('replays a refused create with confirm_duplicates on confirm', async () => {
    const saved = contact()
    vi.mocked(api.createContact)
      .mockRejectedValueOnce(duplicateError([{ id: 'x1', name: 'Existing' }]))
      .mockResolvedValueOnce({ data: saved })
    const { duplicateMatches, save, resolveDuplicates } = useContactSave()

    await save(body())
    const result = await resolveDuplicates(true)

    expect(result).toBe(saved)
    expect(api.createContact).toHaveBeenLastCalledWith({ ...body(), confirm_duplicates: true })
    expect(duplicateMatches.value).toBeNull()
  })

  it('replays a refused update against the frozen id on confirm', async () => {
    const saved = contact()
    vi.mocked(api.updateContact)
      .mockRejectedValueOnce(duplicateError([{ id: 'x1', name: 'Existing' }]))
      .mockResolvedValueOnce({ data: saved })
    const { save, resolveDuplicates } = useContactSave()

    await save(body(), 'c1')
    const result = await resolveDuplicates(true)

    expect(result).toBe(saved)
    expect(api.updateContact).toHaveBeenLastCalledWith('c1', { ...body(), confirm_duplicates: true })
    expect(api.createContact).not.toHaveBeenCalled()
  })

  it('cancelling closes the dialog without another request', async () => {
    vi.mocked(api.createContact).mockRejectedValueOnce(
      duplicateError([{ id: 'x1', name: 'Existing' }]),
    )
    const { duplicateMatches, save, resolveDuplicates } = useContactSave()

    await save(body())
    const result = await resolveDuplicates(false)

    expect(result).toBeNull()
    expect(duplicateMatches.value).toBeNull()
    expect(api.createContact).toHaveBeenCalledTimes(1)
  })

  it('reset drops a paused duplicate flow', async () => {
    vi.mocked(api.createContact).mockRejectedValueOnce(
      duplicateError([{ id: 'x1', name: 'Existing' }]),
    )
    const { duplicateMatches, save, resolveDuplicates, reset } = useContactSave()

    await save(body())
    reset()

    expect(duplicateMatches.value).toBeNull()
    const result = await resolveDuplicates(true)
    expect(result).toBeNull()
    expect(api.createContact).toHaveBeenCalledTimes(1)
  })

  it('toasts non-duplicate failures and returns null', async () => {
    vi.mocked(api.createContact).mockRejectedValue(new Error('network down'))
    const { duplicateMatches, save } = useContactSave()

    const result = await save(body())

    expect(result).toBeNull()
    expect(duplicateMatches.value).toBeNull()
    expect(toast.error).toHaveBeenCalledTimes(1)
  })
})
