// Shared activity display helpers: status derivation and labels used by the
// global activities list, the reminders page, and task rows.

export interface ActivityStatusLike {
  is_done: boolean
  is_cancelled: boolean
  is_reminded: boolean
  remind_at?: string
  scheduled_at?: string
  created_at?: string
}

export function typeLabel(type: string): string {
  return type || 'Task'
}

// isOverdue is reminder-based: an open, un-reminded task whose remind_at has
// passed. Tasks without a remind_at are never overdue here.
export function isOverdue(item: Pick<ActivityStatusLike, 'is_done' | 'is_cancelled' | 'remind_at'>): boolean {
  return (
    !item.is_done && !item.is_cancelled &&
    !!item.remind_at && new Date(item.remind_at).getTime() < Date.now()
  )
}

// due returns the effective due time: remind_at, else scheduled_at, else created_at.
export function due(item: Pick<ActivityStatusLike, 'remind_at' | 'scheduled_at' | 'created_at'>): string {
  if (item.remind_at) return item.remind_at
  if (item.scheduled_at) return item.scheduled_at
  return item.created_at || ''
}

export function dueLabel(item: Pick<ActivityStatusLike, 'remind_at' | 'scheduled_at' | 'created_at'>): string {
  const t = due(item)
  return t ? new Date(t).toLocaleString() : ''
}

// statusLabel returns the display status by precedence: cancelled, done,
// reminded, overdue, else open.
export function statusLabel(item: ActivityStatusLike): string {
  if (item.is_cancelled) return 'Canceled'
  if (item.is_done) return 'Done'
  if (item.is_reminded) return 'Reminded'
  if (isOverdue(item)) return 'Overdue'
  return 'Open'
}

export type StatusVariant = 'default' | 'destructive' | 'secondary' | 'success'

export function statusVariant(label: string): StatusVariant {
  if (label === 'Overdue') return 'destructive'
  if (label === 'Canceled') return 'secondary'
  if (label === 'Done') return 'success'
  if (label === 'Reminded') return 'secondary'
  return 'default'
}