import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

interface User {
  id: string
  name: string
  email: string
  avatar_url?: string
}

interface TokenPair {
  access_token: string
  refresh_token: string
  expires_at: number
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const accessToken = ref<string | null>(null)
  const refreshToken = ref<string | null>(null)

  const isAuthenticated = computed(() => !!accessToken.value)

  function loadTokens() {
    const at = localStorage.getItem('access_token')
    const rt = localStorage.getItem('refresh_token')
    if (at && rt) {
      accessToken.value = at
      refreshToken.value = rt
    }
  }

  function saveTokens(tokens: TokenPair) {
    accessToken.value = tokens.access_token
    refreshToken.value = tokens.refresh_token
    localStorage.setItem('access_token', tokens.access_token)
    localStorage.setItem('refresh_token', tokens.refresh_token)
  }

  async function login(email: string, password: string) {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    })
    const json = await res.json()
    if (json.error) throw new Error(json.error.message)
    saveTokens(json.data)
    await fetchUser()
    return json.data
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
      }
    } catch {}
  }

  async function refresh() {
    if (!refreshToken.value) throw new Error('No refresh token')
    const res = await fetch('/api/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken.value }),
    })
    const json = await res.json()
    if (json.error) {
      logout()
      throw new Error(json.error.message)
    }
    saveTokens(json.data)
    return json.data
  }

  async function logout() {
    if (refreshToken.value) {
      fetch('/api/auth/logout', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${accessToken.value}`,
        },
        body: JSON.stringify({ refresh_token: refreshToken.value }),
      }).catch(() => {})
    }
    user.value = null
    accessToken.value = null
    refreshToken.value = null
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
  }

  return { user, accessToken, refreshToken, isAuthenticated, loadTokens, login, refresh, logout, saveTokens, fetchUser }
})
