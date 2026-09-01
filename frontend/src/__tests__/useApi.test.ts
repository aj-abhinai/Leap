import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'
import { apiClient } from '@/composables/useApi'

const pushMock = vi.fn()

// The apiClient redirects to /login on session expiry via a lazy import of the
// router module; mock it so the real router (and its createRouter dependency)
// is not loaded in the unit test.
vi.mock('@/router', () => ({
  default: { push: pushMock },
}))

describe('useApi', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.cookie = 'crm_csrf=csrf-token-abc'
    localStorage.clear()
    pushMock.mockClear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    document.cookie = 'crm_csrf=; Max-Age=-1'
  })

  function jsonResponse(body: unknown, status = 200): Response {
    return new Response(JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    })
  }

  it('unwraps the envelope on success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ data: [{ id: 'c1' }], meta: { total: 1 } })))
    const res = await apiClient.get('/api/contacts')
    expect(res.data).toEqual([{ id: 'c1' }])
  })

  it('throws a stable error for non-JSON responses', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('<html>bad gateway</html>', { status: 502 }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(apiClient.get('/api/contacts')).rejects.toThrow('Unexpected response from server (502)')
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('refreshes once and retries on a 401', async () => {
    const fetchMock = vi.fn()
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: { code: 'UNAUTHORIZED', message: 'expired' } }, 401))
      .mockResolvedValueOnce(jsonResponse({ data: { access_token: 'fresh-token', expires_at: 999 } }))
      .mockResolvedValueOnce(jsonResponse({ data: ['contact:read'] }))
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    auth.accessToken = 'expired-token'

    const res = await apiClient.get('/api/auth/me/permissions')
    expect(res.data).toEqual(['contact:read'])
    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(fetchMock.mock.calls[2][1].headers.Authorization).toBe('Bearer fresh-token')
  })

  it('logs out and redirects when the refresh fails', async () => {
    const fetchMock = vi.fn()
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: { code: 'UNAUTHORIZED', message: 'expired' } }, 401))
      .mockResolvedValueOnce(jsonResponse({ error: { code: 'UNAUTHORIZED', message: 'session expired' } }, 401))
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    auth.accessToken = 'expired-token'

    await expect(apiClient.get('/api/contacts')).rejects.toThrow('Session expired')
    expect(auth.isAuthenticated).toBe(false)
    expect(pushMock).toHaveBeenCalledWith('/login')
  })
})
