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

  it('bootstrap clears state without fetching when no refresh cookie exists', async () => {
    document.cookie = 'crm_csrf=; Max-Age=-1'
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)

    const auth = useAuthStore()
    await auth.bootstrap()

    expect(fetchMock).not.toHaveBeenCalled()
    expect(auth.accessToken).toBeNull()
    expect(auth.isAuthenticated).toBe(false)
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
})
