import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'

interface ApiResponse<T = any> {
  data: T
  error?: { code: string; message: string }
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

async function request<T = any>(method: string, url: string, body?: any): Promise<ApiResponse<T>> {
  const auth = useAuthStore()
  const router = useRouter()

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (auth.accessToken) {
    headers['Authorization'] = `Bearer ${auth.accessToken}`
  }

  let res = await fetch(url, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (res.status === 401) {
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

export const apiClient = {
  get: <T = any>(url: string) => request<T>('GET', url),
  post: <T = any>(url: string, body?: any) => request<T>('POST', url, body),
  patch: <T = any>(url: string, body?: any) => request<T>('PATCH', url, body),
  put: <T = any>(url: string, body?: any) => request<T>('PUT', url, body),
  delete: <T = any>(url: string) => request<T>('DELETE', url),
}
