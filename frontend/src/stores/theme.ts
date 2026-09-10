import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const KEY = 'crm-theme'

// One theme vocabulary across the store and the first-paint bootstrap: an
// explicit light/dark choice, or 'system' which follows the OS preference.
// Anything else (missing, junk, legacy values) normalizes to 'system'.
export type Theme = 'light' | 'dark' | 'system'

function normalizeTheme(value: string | null): Theme {
  return value === 'light' || value === 'dark' ? value : 'system'
}

function getStored(): Theme {
  return normalizeTheme(localStorage.getItem(KEY))
}

function persist(value: Theme) {
  localStorage.setItem(KEY, value)
}

function apply(resolved: 'light' | 'dark') {
  if (resolved === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

function systemPrefersDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

export const useThemeStore = defineStore('theme', () => {
  const theme = ref<Theme>('system')

  const resolvedTheme = computed<'light' | 'dark'>(() => {
    if (theme.value === 'system') {
      return systemPrefersDark() ? 'dark' : 'light'
    }
    return theme.value
  })

  function init() {
    theme.value = getStored()
    apply(resolvedTheme.value)
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', onSystemChange)
  }

  function onSystemChange() {
    if (theme.value === 'system') {
      apply(resolvedTheme.value)
    }
  }

  function toggle() {
    const next: Theme =
      theme.value === 'system'
        ? systemPrefersDark()
          ? 'light'
          : 'dark'
        : theme.value === 'dark'
          ? 'light'
          : 'dark'
    set(next)
  }

  function set(value: Theme) {
    theme.value = value
    persist(value)
    apply(resolvedTheme.value)
  }

  return { theme, resolvedTheme, init, toggle, set }
})
