import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useThemeStore } from '@/stores/theme'

// jsdom does not implement matchMedia; the theme store follows the OS
// preference through it, so every test gets a light/dark-controllable stub.
function stubMatchMedia(matches: boolean) {
  vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
    matches,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }))
}

describe('theme store', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('normalizes missing and junk stored values to system', () => {
    stubMatchMedia(false)
    const store = useThemeStore()
    store.init()
    expect(store.theme).toBe('system')

    localStorage.setItem('crm-theme', 'neon')
    const second = useThemeStore()
    second.init()
    expect(second.theme).toBe('system')
  })

  it('keeps explicit light and dark and applies the resolved class', () => {
    stubMatchMedia(true)
    localStorage.setItem('crm-theme', 'light')
    const store = useThemeStore()
    store.init()
    expect(store.theme).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)

    store.set('dark')
    expect(localStorage.getItem('crm-theme')).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('resolves system from the OS preference', () => {
    stubMatchMedia(true)
    const store = useThemeStore()
    store.init()
    expect(store.resolvedTheme).toBe('dark')
  })
})
