import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'

const accessToken = 'jwt-access-token'
const csrfToken = 'csrf-token-abc'

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.cookie = `crm_csrf=${csrfToken}`
    localStorage.clear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    document.cookie = 'crm_csrf=; Max-Age=-1'
  })

  it('keeps the access token in memory only after login', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(jsonResponse({ data: { access_token: accessToken, expires_at: 999 } }))
    fetchMock.mockResolvedValueOnce(jsonResponse({ data: { id: 'u1', name: 'Alice', email: 'a@b.c' } }))
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    await auth.login('alice@example.com', 'password')

    expect(auth.accessToken).toBe(accessToken)
    expect(auth.isAuthenticated).toBe(true)
    expect(localStorage.getItem('access_token')).toBeNull()
    expect(localStorage.getItem('refresh_token')).toBeNull()
  })

  it('refreshes with the CSRF header and stores the new access token', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(jsonResponse({ data: { access_token: accessToken, expires_at: 999 } }))
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    await auth.refresh()

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/auth/refresh')
    expect(init.method).toBe('POST')
    expect(init.headers['X-CSRF-Token']).toBe(csrfToken)
    expect(auth.accessToken).toBe(accessToken)
  })

  it('makes a single refresh request for concurrent callers', async () => {
    let resolveFetch: (r: Response) => void
    const fetchMock = vi.fn()
    fetchMock.mockImplementationOnce(
      () => new Promise<Response>((resolve) => { resolveFetch = resolve }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    const first = auth.refresh()
    const second = auth.refresh()
    resolveFetch!(jsonResponse({ data: { access_token: accessToken, expires_at: 999 } }))

    await Promise.all([first, second])
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('clears state and sends the CSRF header on logout', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(jsonResponse({ message: 'Logged out' }))
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    auth.accessToken = accessToken
    await auth.logout()

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/auth/logout')
    expect(init.method).toBe('POST')
    expect(init.headers['X-CSRF-Token']).toBe(csrfToken)
    expect(auth.accessToken).toBeNull()
    expect(auth.isAuthenticated).toBe(false)
  })

  it('a refresh that resolves after logout cannot resurrect the session', async () => {
    let resolveRefresh: (r: Response) => void
    const fetchMock = vi.fn()
    fetchMock.mockImplementationOnce(
      () => new Promise<Response>((resolve) => { resolveRefresh = resolve }),
    )
    fetchMock.mockResolvedValueOnce(jsonResponse({ message: 'Logged out' }))
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    auth.accessToken = accessToken

    const pendingRefresh = auth.refresh()
    await auth.logout()

    resolveRefresh!(jsonResponse({ data: { access_token: 'rotated-token', expires_at: 999 } }))
    await expect(pendingRefresh).rejects.toThrow('Session expired')
    expect(auth.accessToken).toBeNull()
    expect(auth.isAuthenticated).toBe(false)
  })

  it('bootstrap clears state without fetching when no CSRF cookie exists', async () => {
    document.cookie = 'crm_csrf=; Max-Age=-1'
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    await auth.bootstrap()

    expect(fetchMock).not.toHaveBeenCalled()
    expect(auth.accessToken).toBeNull()
    expect(auth.isAuthenticated).toBe(false)
  })

  it('restores the session during bootstrap with the CSRF cookie', async () => {
    const fetchMock = vi.fn()
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ data: { access_token: accessToken, expires_at: 999 } }))
      .mockResolvedValueOnce(jsonResponse({ data: { id: 'u1', name: 'Alice', email: 'a@b.c' } }))
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    await auth.bootstrap()

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/auth/refresh')
    expect(fetchMock.mock.calls[1][0]).toBe('/api/auth/me')
    expect(auth.isAuthenticated).toBe(true)
    expect(auth.user?.id).toBe('u1')
  })

  it('failed refresh clears state', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: { code: 'UNAUTHORIZED', message: 'expired' } }, 401))
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    auth.accessToken = accessToken

    await expect(auth.refresh()).rejects.toThrow('expired')
    expect(auth.accessToken).toBeNull()
    expect(auth.isAuthenticated).toBe(false)
  })

  it('login sets mustChangePassword from the user record', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          access_token: accessToken,
          expires_at: 999,
        },
      }),
    )
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: { id: 'u1', name: 'Alice', email: 'a@b.c', must_change_password: true },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    await auth.login('user@example.com', 'password')

    expect(auth.mustChangePassword).toBe(true)
    expect(auth.user?.id).toBe('u1')
    expect(auth.isAuthenticated).toBe(true)
  })

  it('login keeps mustChangePassword from the login response when the user fetch fails', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          access_token: accessToken,
          expires_at: 999,
          must_change_password: true,
        },
      }),
    )
    fetchMock.mockRejectedValueOnce(new Error('network down'))
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    await auth.login('user@example.com', 'password')

    expect(auth.mustChangePassword).toBe(true)
    expect(auth.isAuthenticated).toBe(true)
  })

  it('login keeps mustChangePassword when the user record omits the field', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          access_token: accessToken,
          expires_at: 999,
          must_change_password: true,
        },
      }),
    )
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ data: { id: 'u1', name: 'Alice', email: 'a@b.c' } }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    await auth.login('user@example.com', 'password')

    expect(auth.mustChangePassword).toBe(true)
    expect(auth.user?.id).toBe('u1')
  })

  it('changePassword stores the fresh access token and clears the flag', async () => {
    const fetchMock = vi.fn()
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ data: { access_token: accessToken, expires_at: 999 } }))
      .mockResolvedValueOnce(jsonResponse({ data: { id: 'u1', name: 'Alice', email: 'a@b.c', must_change_password: false } }))
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    await auth.bootstrap()
    auth.mustChangePassword = true

    fetchMock.mockResolvedValueOnce(
      jsonResponse({ data: { access_token: 'fresh-access-token', expires_at: 999 } }),
    )
    await auth.changePassword('old', 'new-password')

    expect(auth.mustChangePassword).toBe(false)
    expect(auth.accessToken).toBe('fresh-access-token')
    expect(auth.isAuthenticated).toBe(true)
  })
})
