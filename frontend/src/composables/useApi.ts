import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'

interface ApiResponse {
  data: any
  error?: { code: string; message: string }
  meta?: { page: number; per_page: number; total: number }
}

async function request(method: string, url: string, body?: any): Promise<ApiResponse> {
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

  if (res.status === 401 && auth.refreshToken) {
    try {
      await auth.refresh()
      headers['Authorization'] = `Bearer ${auth.accessToken}`
      res = await fetch(url, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
      })
    } catch {
      auth.logout()
      router.push('/login')
      throw new Error('Session expired')
    }
  }

  const json = await res.json()
  if (json.error) {
    throw new Error(json.error.message)
  }
  return json
}

export const apiClient = {
  get: (url: string) => request('GET', url),
  post: (url: string, body?: any) => request('POST', url, body),
  patch: (url: string, body?: any) => request('PATCH', url, body),
  delete: (url: string) => request('DELETE', url),
}
