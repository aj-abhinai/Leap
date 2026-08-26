import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'

interface ApiErrorPayload {
  code: string
  message: string
}

export interface ApiResponse<T = any> {
  data: T
  error?: ApiErrorPayload | null
  meta?: { page: number; per_page: number; total: number }
}

export class ApiError extends Error {
  code?: string
  constructor(message: string, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

export interface RequestOptions {
  headers?: Record<string, string>
  // skipAuth disables the 401-refresh-retry loop for requests that carry
  // their own auth semantics (login, refresh, logout): a failed refresh must
  // not trigger another refresh.
  skipAuth?: boolean
}

async function request<T = any>(
  method: string,
  url: string,
  body?: any,
  options: RequestOptions = {},
): Promise<ApiResponse<T>> {
  const auth = useAuthStore()
  const router = useRouter()

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...options.headers,
  }
  if (auth.accessToken) {
    headers['Authorization'] = `Bearer ${auth.accessToken}`
  }

  let res = await fetch(url, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (res.status === 401 && !options.skipAuth) {
    try {
      await auth.refresh()
      headers['Authorization'] = `Bearer ${auth.accessToken}`
      res = await fetch(url, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
      })
    } catch {
      await auth.logout()
      router.push('/login')
      throw new Error('Session expired')
    }
  }

  const text = await res.text()
  let json: ApiResponse<T>
  try {
    json = JSON.parse(text)
  } catch {
    throw new Error(`Unexpected response from server (${res.status})`)
  }
  if (json.error) {
    throw new ApiError(json.error.message, json.error.code)
  }
  return json
}

// download fetches a URL with auth (refreshing on 401) and returns the
// response as a Blob, for file downloads (e.g. CSV export).
export async function apiDownload(url: string): Promise<Blob> {
  const auth = useAuthStore()
  const router = useRouter()

  const headers: Record<string, string> = {}
  if (auth.accessToken) {
    headers['Authorization'] = `Bearer ${auth.accessToken}`
  }

  let res = await fetch(url, { headers })

  if (res.status === 401) {
    try {
      await auth.refresh()
      headers['Authorization'] = `Bearer ${auth.accessToken}`
      res = await fetch(url, { headers })
    } catch {
      await auth.logout()
      router.push('/login')
      throw new Error('Session expired')
    }
  }

  if (!res.ok) {
    throw new Error(`Export failed (${res.status})`)
  }
  return res.blob()
}

export const apiClient = {
  get: <T = any>(url: string, options?: RequestOptions) => request<T>('GET', url, undefined, options),
  post: <T = any>(url: string, body?: any, options?: RequestOptions) => request<T>('POST', url, body, options),
  patch: <T = any>(url: string, body?: any, options?: RequestOptions) => request<T>('PATCH', url, body, options),
  put: <T = any>(url: string, body?: any, options?: RequestOptions) => request<T>('PUT', url, body, options),
  delete: <T = any>(url: string, options?: RequestOptions) => request<T>('DELETE', url, undefined, options),
}
