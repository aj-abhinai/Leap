import { apiDownload } from '@/composables/useApi'

// CSV export for the data:export permission.

export function downloadCsv(entity: 'contacts' | 'leads'): Promise<Blob> {
  return apiDownload(`/api/export/csv?entity=${entity}`)
}