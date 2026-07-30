import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const KEY = 'crm-theme'

function getStored(): string | null {
  return localStorage.getItem(KEY)
}

function persist(value: string) {
  localStorage.setItem(KEY, value)
}

function apply(cls: string) {
  if (cls === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

function systemPrefersDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

export const useThemeStore = defineStore('theme', () => {
  const theme = ref<string>('system')

  const resolvedTheme = computed(() => {
    if (theme.value === 'system') {
      return systemPrefersDark() ? 'dark' : 'light'
    }
    return theme.value
  })

  function init() {
    const stored = getStored()
    theme.value = stored || 'system'
    apply(resolvedTheme.value)
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', onSystemChange)
  }

  function onSystemChange() {
    if (theme.value === 'system') {
      apply(resolvedTheme.value)
    }
  }

  function toggle() {
    const next =
      theme.value === 'system'
        ? systemPrefersDark()
          ? 'light'
          : 'dark'
        : theme.value === 'dark'
          ? 'light'
          : 'dark'
    set(next)
  }

  function set(value: string) {
    theme.value = value
    persist(value)
    apply(resolvedTheme.value)
  }

  return { theme, resolvedTheme, init, toggle, set }
})
