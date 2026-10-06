export interface QuickReplyGroup<T> {
  group: string
  items: T[]
}

// groupQuickReplies buckets quick-reply chips by group (defaulting to
// "Quick reply") and sorts each bucket by sort_order, for the ordered palette
// rendered in task forms. A second tap un-picks a chip at the call site.
export function groupQuickReplies<T extends { group_name?: string | null; sort_order: number }>(
  chips: T[],
): QuickReplyGroup<T>[] {
  const groups = new Map<string, T[]>()
  for (const chip of chips) {
    const key = chip.group_name || 'Quick reply'
    const bucket = groups.get(key) ?? []
    bucket.push(chip)
    groups.set(key, bucket)
  }
  return [...groups.entries()].map(([group, items]) => ({
    group,
    items: items.slice().sort((a, b) => a.sort_order - b.sort_order),
  }))
}
