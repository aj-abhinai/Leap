import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { apiClient } from '@/composables/useApi'

interface User {
  id: string
  name: string
  email: string
  phone?: string
  avatar_url?: string
  must_change_password: boolean
}

interface LoginResponse {
  access_token: string
  expires_at: number
  must_change_password?: boolean
}

const CSRF_COOKIE = 'crm_csrf'

function getCookie(name: string): string {
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`))
  return match ? decodeURIComponent(match[1]) : ''
}

function csrfHeaders(): Record<string, string> {
  const token = getCookie(CSRF_COOKIE)
  return token ? { 'X-CSRF-Token': token } : {}
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const accessToken = ref<string | null>(null)
  const mustChangePassword = ref(false)

  const isAuthenticated = computed(() => !!accessToken.value)

  function setAccess(tokens: LoginResponse) {
    accessToken.value = tokens.access_token
  }

  function clear() {
    accessToken.value = null
    user.value = null
    mustChangePassword.value = false
  }

  async function fetchUser() {
    if (!accessToken.value) return
    try {
      const res = await fetch('/api/auth/me', {
        headers: { 'Authorization': `Bearer ${accessToken.value}` },
      })
      if (res.ok) {
        const json = await res.json()
        user.value = json.data
        mustChangePassword.value = json.data.must_change_password === true
      }
    } catch {}
  }

  let refreshPromise: Promise<LoginResponse> | null = null

  // Bumped on every login and logout. A refresh that started in a previous
  // session must not resurrect it after logout completed: the server rotates
  // the refresh cookie on every refresh, so a late response can otherwise
  // re-arm a logged-out session and trap the UI on an authenticated page.
  let sessionEpoch = 0

  async function refresh(): Promise<LoginResponse> {
    if (!refreshPromise) {
      refreshPromise = doRefresh().finally(() => {
        refreshPromise = null
      })
    }
    return refreshPromise
  }

  async function doRefresh(): Promise<LoginResponse> {
    const epochAtStart = sessionEpoch
    const res = await fetch('/api/auth/refresh', {
      method: 'POST',
      headers: { ...csrfHeaders() },
    })
    const json = await res.json()
    if (!res.ok || json.error) {
      clear()
      throw new Error(json.error?.message ?? 'Session expired')
    }
    if (epochAtStart !== sessionEpoch) {
      // The session this refresh belonged to is gone; discard the rotated
      // tokens instead of overwriting the current state.
      throw new Error('Session expired')
    }
    setAccess(json.data)
    return json.data
  }

  async function login(email: string, password: string) {
    sessionEpoch++
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    })
    const json = await res.json()
    if (json.error) throw new Error(json.error.message)
    setAccess(json.data)
    mustChangePassword.value = json.data.must_change_password === true
    if (!mustChangePassword.value) {
      await fetchUser()
    }
    return json.data
  }

  async function logout() {
    sessionEpoch++
    try {
      await fetch('/api/auth/logout', {
        method: 'POST',
        headers: { ...csrfHeaders() },
      })
    } catch {}
    clear()
  }

  async function bootstrap() {
    // The refresh token is HttpOnly and cannot be inspected from JavaScript.
    // The CSRF cookie is readable and is required for the refresh request.
    if (!getCookie(CSRF_COOKIE)) {
      clear()
      return
    }
    try {
      await refresh()
      await fetchUser()
    } catch {
      clear()
    }
  }

  async function updateProfile(name: string, phone: string) {
    const res = await apiClient.patch('/api/auth/me', { name, phone })
    user.value = res.data
    mustChangePassword.value = res.data.must_change_password === true
    return res.data
  }

  async function changePassword(currentPassword: string, newPassword: string) {
    const res = await apiClient.patch('/api/auth/me/password', {
      current_password: currentPassword,
      new_password: newPassword,
    })
    mustChangePassword.value = false
    return res.data
  }

  return {
    user,
    accessToken,
    mustChangePassword,
    isAuthenticated,
    login,
    refresh,
    logout,
    bootstrap,
    fetchUser,
    updateProfile,
    changePassword,
  }
})
