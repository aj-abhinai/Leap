import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import LoginPage from '@/views/LoginPage.vue'
import { useAuthStore } from '@/stores/auth'
import { createRouter, createMemoryHistory } from 'vue-router'

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function makeRouter() {
  const Dummy = { template: '<div />' }
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/dashboard', name: 'Dashboard', component: Dummy },
      { path: '/change-password', name: 'ChangePassword', component: Dummy },
    ],
  })
}

async function fillAndSubmit(
  wrapper: ReturnType<typeof mount>,
  email: string,
  password: string,
) {
  await wrapper.find('input#email').setValue(email)
  await wrapper.find('input#password').setValue(password)
  await wrapper.find('form').trigger('submit')
}

describe('LoginPage', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renders the login form', () => {
    const wrapper = mount(LoginPage, {
      global: { plugins: [createPinia(), makeRouter()] },
    })

    expect(wrapper.find('form').exists()).toBe(true)
    expect(wrapper.find('input#email').attributes('type')).toBe('email')
    expect(wrapper.find('input#password').attributes('type')).toBe('password')
    expect(wrapper.find('button[type="submit"]').text()).toContain('Sign In')
  })

  it('shows an error when submitting with empty fields', async () => {
    const wrapper = mount(LoginPage, {
      global: { plugins: [createPinia(), makeRouter()] },
    })

    await fillAndSubmit(wrapper, '', '')

    expect(wrapper.text()).toContain('Email and password are required')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('sends credentials, stores the session, and navigates to the dashboard', async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: { access_token: 'jwt-access-token', expires_at: 999 },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ data: { id: 'u1', name: 'Alice', email: 'a@b.c' } }),
      )
    const pinia = createPinia()
    const router = makeRouter()
    const pushSpy = vi.spyOn(router, 'push')
    const wrapper = mount(LoginPage, {
      global: { plugins: [pinia, router] },
    })

    await fillAndSubmit(wrapper, 'alice@example.com', 'secret-pw')
    await flushPromises()

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/auth/login')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body)).toEqual({
      email: 'alice@example.com',
      password: 'secret-pw',
    })
    expect(useAuthStore(pinia).accessToken).toBe('jwt-access-token')
    expect(pushSpy).toHaveBeenCalledWith({ name: 'Dashboard' })
  })

  it('navigates to change-password when the server flags the user', async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            access_token: 'jwt-access-token',
            expires_at: 999,
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: { id: 'u1', name: 'Bob', email: 'b@b.c', must_change_password: true },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ data: [] }))
    const router = makeRouter()
    const pushSpy = vi.spyOn(router, 'push')
    const wrapper = mount(LoginPage, {
      global: { plugins: [createPinia(), router] },
    })

    await fillAndSubmit(wrapper, 'bob@example.com', 'old-password')
    await flushPromises()

    expect(pushSpy).toHaveBeenCalledWith({ name: 'ChangePassword' })
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })

  it('shows the server error message when login fails', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(
        { error: { code: 'INVALID_CREDENTIALS', message: 'Invalid email or password' } },
        401,
      ),
    )
    const router = makeRouter()
    const pushSpy = vi.spyOn(router, 'push')
    const wrapper = mount(LoginPage, {
      global: { plugins: [createPinia(), router] },
    })

    await fillAndSubmit(wrapper, 'alice@example.com', 'wrong-password')
    await flushPromises()

    expect(wrapper.text()).toContain('Invalid email or password')
    expect(pushSpy).not.toHaveBeenCalled()
  })

  it('disables the submit button while the request is in flight', async () => {
    let resolveFetch!: (r: Response) => void
    fetchMock.mockImplementationOnce(
      () =>
        new Promise<Response>((resolve) => {
          resolveFetch = resolve
        }),
    )
    const wrapper = mount(LoginPage, {
      global: { plugins: [createPinia(), makeRouter()] },
    })

    await fillAndSubmit(wrapper, 'alice@example.com', 'secret-pw')

    const button = wrapper.find('button[type="submit"]')
    expect(button.attributes('disabled')).toBeDefined()

    resolveFetch(
      jsonResponse({
        data: {
          access_token: 'jwt-access-token',
          expires_at: 999,
          must_change_password: true,
        },
      }),
    )
    await flushPromises()

    expect(button.attributes('disabled')).toBeUndefined()
  })
})
