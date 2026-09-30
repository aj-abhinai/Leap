import { computed, shallowRef } from 'vue'
import { toast } from 'vue-sonner'

// useContactSelection tracks the contacts selected for a bulk action. The id
// set is sticky across pagination and view switches (the host component
// stays mounted); pageIds() reports the ids currently rendered so a header
// checkbox can act on the visible page. Selecting beyond cap is refused with
// a toast — the server enforces the same bound. State is component-scoped
// and never persisted.
export function useContactSelection(pageIds: () => string[], cap = 500) {
  const selectedIds = shallowRef<Set<string>>(new Set())
  const count = computed(() => selectedIds.value.size)

  function isSelected(id: string): boolean {
    return selectedIds.value.has(id)
  }

  function pageAllSelected(): boolean {
    const ids = pageIds()
    return ids.length > 0 && ids.every((id) => selectedIds.value.has(id))
  }

  function pageSomeSelected(): boolean {
    return pageIds().some((id) => selectedIds.value.has(id))
  }

  function pageCheckState(): boolean | 'indeterminate' {
    if (pageAllSelected()) return true
    if (pageSomeSelected()) return 'indeterminate'
    return false
  }

  function toggle(id: string) {
    const next = new Set(selectedIds.value)
    if (next.has(id)) {
      next.delete(id)
    } else {
      if (next.size >= cap) {
        toast.error(`Selection limit is ${cap} contacts`)
        return
      }
      next.add(id)
    }
    selectedIds.value = next
  }

  // togglePage selects or deselects every id on the rendered page. A toggle
  // that would pass the cap is refused whole (partial pages would leave the
  // header checkbox in a state the operator did not choose).
  function togglePage() {
    const ids = pageIds()
    if (ids.length === 0) return
    const next = new Set(selectedIds.value)
    if (pageAllSelected()) {
      for (const id of ids) next.delete(id)
    } else {
      const added = ids.filter((id) => !next.has(id))
      if (next.size + added.length > cap) {
        toast.error(`Selection limit is ${cap} contacts`)
        return
      }
      for (const id of added) next.add(id)
    }
    selectedIds.value = next
  }

  function clear() {
    selectedIds.value = new Set()
  }

  return { selectedIds, count, isSelected, pageCheckState, toggle, togglePage, clear }
}
