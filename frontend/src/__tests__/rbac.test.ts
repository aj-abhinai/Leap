import { describe, it, expect, vi, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useRBACStore } from '@/stores/rbac'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/api/auth', () => ({
  getMyPermissions: vi.fn(),
  login: vi.fn(),
  refresh: vi.fn(),
  logout: vi.fn(),
  getMe: vi.fn(),
  hasCsrfCookie: vi.fn(),
  updateProfile: vi.fn(),
  changePassword: vi.fn(),
}))

import * as authApi from '@/api/auth'

describe('rbac store', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('does not fetch permissions while logged out', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const rbac = useRBACStore()

    await rbac.fetchPermissions()

    expect(authApi.getMyPermissions).not.toHaveBeenCalled()
    expect(rbac.can('contact:read')).toBe(false)
  })

  it('coalesces concurrent fetches into one request', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore()
    auth.accessToken = 'test-token'
    ;(authApi.getMyPermissions as ReturnType<typeof vi.fn>).mockResolvedValue({ data: ['contact:read'] })

    const rbac = useRBACStore()
    await Promise.all([rbac.fetchPermissions(), rbac.fetchPermissions()])

    expect(authApi.getMyPermissions).toHaveBeenCalledTimes(1)
    expect(rbac.can('contact:read')).toBe(true)
  })

  it('clears loaded state so the next session fetches again', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore()
    auth.accessToken = 'test-token'
    ;(authApi.getMyPermissions as ReturnType<typeof vi.fn>).mockResolvedValue({ data: ['*'] })

    const rbac = useRBACStore()
    await rbac.fetchPermissions()
    rbac.clear()
    await rbac.fetchPermissions()

    expect(authApi.getMyPermissions).toHaveBeenCalledTimes(2)
  })

  it('discards a permission response that lands after the session cleared', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore()
    auth.accessToken = 'test-token'
    let resolveRequest: (value: { data: string[] }) => void = () => {}
    ;(authApi.getMyPermissions as ReturnType<typeof vi.fn>).mockImplementation(
      () => new Promise((resolve) => { resolveRequest = resolve }),
    )

    const rbac = useRBACStore()
    const pending = rbac.fetchPermissions()
    // Logout clears everything while the request is still in flight.
    rbac.clear()
    resolveRequest({ data: ['*'] })
    await pending

    expect(rbac.permissions).toEqual([])
    expect(rbac.can('*')).toBe(false)
  })
})
