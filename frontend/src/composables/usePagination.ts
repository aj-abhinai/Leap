import { ref } from 'vue'
import { createRequestGuard } from '@/composables/useRequestGuard'

export interface PaginatedResponse<T> {
  data: T
  meta?: { total?: number }
}

// totalOf unwraps the envelope's total, defaulting to 0 when absent.
export function totalOf<T>(res: PaginatedResponse<T>): number {
  return res.meta?.total || 0
}

// usePagination owns the shared list-pagination state: the items, the server
// total, and the loading flag. The caller supplies a buildRequest closure that
// performs the entity-specific fetch (URL + params) and returns the envelope;
// the composable unwraps data and total, and manages loading around the call.
export function usePagination<T>() {
  const items = ref<T[]>([])
  const total = ref(0)
  const loading = ref(false)

  // The guard discards out-of-order responses: only the latest request's
  // result may land, so rapid page/search changes never show stale data.
  const guard = createRequestGuard()

  async function fetch(
    buildRequest: (page: number, perPage: number) => Promise<PaginatedResponse<T[]>>,
    page = 1,
    perPage = 20,
  ) {
    const run = guard.start()
    loading.value = true
    try {
      const res = await buildRequest(page, perPage)
      if (!run.isCurrent()) return
      items.value = res.data
      total.value = totalOf(res)
    } finally {
      if (run.isCurrent()) loading.value = false
    }
  }

  function setTotal(n: number) {
    total.value = n
  }

  return { items, total, loading, fetch, setTotal }
}
