import { apiClient, type ApiResponse, type RequestOptions } from '@/composables/useApi'

// Auth endpoints. The auth store owns session state (tokens, epochs) and
// calls these; refresh/logout carry the CSRF cookie header and skip the
// client's 401-refresh loop (a failed refresh must not refresh itself).
// Login has neither: the route enforces no CSRF check because a first
// login has no cookie to echo yet.

const CSRF_COOKIE = 'crm_csrf'

function getCookie(name: string): string {
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`))
  return match ? decodeURIComponent(match[1]) : ''
}

function csrfOptions(): RequestOptions {
  const token = getCookie(CSRF_COOKIE)
  return { skipAuth: true, headers: token ? { 'X-CSRF-Token': token } : {} }
}

// hasCsrfCookie tells the auth store whether a session can exist at all: the
// refresh cookie is HttpOnly and unreadable, so the CSRF cookie is the only
// client-side signal of an established session.
export function hasCsrfCookie(): boolean {
  return !!getCookie(CSRF_COOKIE)
}

export interface AuthUser {
  id: string
  name: string
  email: string
  phone?: string
  avatar_url?: string
}

export interface LoginResponse {
  access_token: string
  expires_at: number
  must_change_password?: boolean
}

export function login(email: string, password: string): Promise<ApiResponse<LoginResponse>> {
  return apiClient.post('/api/auth/login', { email, password }, { skipAuth: true })
}

export function refresh(): Promise<ApiResponse<LoginResponse>> {
  return apiClient.post('/api/auth/refresh', undefined, csrfOptions())
}

export function logout(): Promise<ApiResponse<null>> {
  return apiClient.post('/api/auth/logout', undefined, csrfOptions())
}

export function getMe(): Promise<ApiResponse<AuthUser>> {
  return apiClient.get('/api/auth/me')
}

export function updateProfile(body: { name: string; phone: string }): Promise<ApiResponse<AuthUser>> {
  return apiClient.patch('/api/auth/me', body)
}

export function changePassword(body: {
  current_password: string
  new_password: string
}): Promise<ApiResponse<LoginResponse>> {
  return apiClient.patch('/api/auth/me/password', body)
}

export function getMyPermissions(): Promise<ApiResponse<string[]>> {
  return apiClient.get('/api/auth/me/permissions')
}