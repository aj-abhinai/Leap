import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as authApi from '@/api/auth'
import { useRBACStore } from '@/stores/rbac'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<authApi.AuthUser | null>(null)
  const accessToken = ref<string | null>(null)
  const mustChangePassword = ref(false)

  const isAuthenticated = computed(() => !!accessToken.value)

  function setAccess(tokens: authApi.LoginResponse) {
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
      const res = await authApi.getMe()
      user.value = res.data
      if ('must_change_password' in res.data) {
        mustChangePassword.value = res.data.must_change_password === true
      }
    } catch {}
  }

  let refreshPromise: Promise<authApi.LoginResponse> | null = null

  // Bumped on every login and logout. A refresh that started in a previous
  // session must not resurrect it after logout completed: the server rotates
  // the refresh cookie on every refresh, so a late response can otherwise
  // re-arm a logged-out session and trap the UI on an authenticated page.
  let sessionEpoch = 0

  async function refresh(): Promise<authApi.LoginResponse> {
    if (!refreshPromise) {
      refreshPromise = doRefresh().finally(() => {
        refreshPromise = null
      })
    }
    return refreshPromise
  }

  async function doRefresh(): Promise<authApi.LoginResponse> {
    const epochAtStart = sessionEpoch
    let tokens: authApi.LoginResponse
    try {
      const res = await authApi.refresh()
      tokens = res.data
    } catch {
      clear()
      throw new Error('Session expired')
    }
    if (epochAtStart !== sessionEpoch) {
      // The session this refresh belonged to is gone; discard the rotated
      // tokens instead of overwriting the current state.
      throw new Error('Session expired')
    }
    setAccess(tokens)
    return tokens
  }

  async function login(email: string, password: string) {
    sessionEpoch++
    const res = await authApi.login(email, password)
    setAccess(res.data)
    // The login response is the authoritative source: if /me fails below, a
    // user flagged for a forced password change must still be redirected.
    mustChangePassword.value = res.data.must_change_password === true
    await fetchUser()
    // Refresh permissions for the newly authenticated role; the boot-time
    // fetch in App.vue ran before any user was signed in.
    await useRBACStore().fetchPermissions()
    return res.data
  }

  async function logout() {
    sessionEpoch++
    try {
      await authApi.logout()
    } catch {}
    clear()
    useRBACStore().clear()
  }

  async function bootstrap() {
    // The refresh token is HttpOnly and cannot be inspected from JavaScript.
    // The CSRF cookie is readable and is required for the refresh request.
    if (!authApi.hasCsrfCookie()) {
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
    const res = await authApi.updateProfile({ name, phone })
    user.value = res.data
    return res.data
  }

  async function changePassword(currentPassword: string, newPassword: string) {
    const res = await authApi.changePassword({ current_password: currentPassword, new_password: newPassword })
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