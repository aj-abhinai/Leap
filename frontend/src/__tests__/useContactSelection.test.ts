import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useContactSelection } from '@/composables/useContactSelection'

const { toast } = vi.hoisted(() => ({ toast: { error: vi.fn(), success: vi.fn() } }))
vi.mock('vue-sonner', () => ({ toast }))

describe('useContactSelection', () => {
  beforeEach(() => {
    toast.error.mockClear()
  })

  it('toggles ids and reports the count', () => {
    const selection = useContactSelection(() => ['a', 'b'])
    selection.toggle('a')
    expect(selection.count.value).toBe(1)
    expect(selection.isSelected('a')).toBe(true)
    selection.toggle('a')
    expect(selection.count.value).toBe(0)
    expect(selection.isSelected('a')).toBe(false)
  })

  it('reports the page check state from the rendered page ids', () => {
    const selection = useContactSelection(() => ['a', 'b'])
    expect(selection.pageCheckState()).toBe(false)
    selection.toggle('a')
    expect(selection.pageCheckState()).toBe('indeterminate')
    selection.toggle('b')
    expect(selection.pageCheckState()).toBe(true)
  })

  it('keeps ids selected across page changes', () => {
    let page = ['a', 'b']
    const selection = useContactSelection(() => page)
    selection.toggle('a')
    page = ['c', 'd']
    selection.toggle('d')
    expect(selection.count.value).toBe(2)
    expect(selection.isSelected('a')).toBe(true)
    expect(selection.isSelected('d')).toBe(true)
    expect(selection.pageCheckState()).toBe('indeterminate')
  })

  it('selects and clears the whole page', () => {
    const selection = useContactSelection(() => ['a', 'b'])
    selection.togglePage()
    expect(selection.count.value).toBe(2)
    expect(selection.pageCheckState()).toBe(true)
    selection.togglePage()
    expect(selection.count.value).toBe(0)
  })

  it('refuses a row selection beyond the cap with a toast', () => {
    const selection = useContactSelection(() => ['a', 'b', 'c'], 2)
    selection.toggle('a')
    selection.toggle('b')
    selection.toggle('c')
    expect(selection.count.value).toBe(2)
    expect(selection.isSelected('c')).toBe(false)
    expect(toast.error).toHaveBeenCalledTimes(1)
  })

  it('refuses a page toggle that would pass the cap', () => {
    const selection = useContactSelection(() => ['a', 'b'], 2)
    selection.toggle('x')
    selection.toggle('y')
    selection.togglePage()
    expect(selection.count.value).toBe(2)
    expect(selection.isSelected('a')).toBe(false)
    expect(toast.error).toHaveBeenCalledTimes(1)
  })

  it('clears every selected id', () => {
    const selection = useContactSelection(() => ['a', 'b'])
    selection.toggle('a')
    selection.toggle('b')
    selection.clear()
    expect(selection.count.value).toBe(0)
    expect(selection.pageCheckState()).toBe(false)
  })
})
