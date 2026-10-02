import { computed, type ComputedRef, type Ref, ref } from 'vue'

export type SortValue = string | number

export interface SortableTable<T> {
  sortKey: Ref<string>
  sortAsc: Ref<boolean>
  setSort: (key: string) => void
  indicator: (key: string) => string
  sorted: ComputedRef<T[]>
}

/**
 * Makes every column of a grid sortable. `source` supplies the current
 * rows (e.g. after filtering); `accessors` maps a column key to the
 * value used for comparison (receiving the row's source index so "#"
 * columns can sort by original position). Unmapped keys read the
 * same-named property off the row.
 */
export function useSortable<T>(
  source: () => T[],
  opts: {
    defaultKey: string
    defaultAsc?: boolean
    accessors?: Record<string, (row: T, index: number) => SortValue>
  },
): SortableTable<T> {
  const sortKey = ref(opts.defaultKey)
  const sortAsc = ref(opts.defaultAsc ?? true)

  function setSort(key: string) {
    if (sortKey.value === key) {
      sortAsc.value = !sortAsc.value
    } else {
      sortKey.value = key
      sortAsc.value = true
    }
  }

  function indicator(key: string): string {
    if (sortKey.value !== key) return '↕'
    return sortAsc.value ? '↑' : '↓'
  }

  const sorted = computed<T[]>(() => {
    const rows = source().map((row, index) => ({ row, index }))
    const accessor = opts.accessors?.[sortKey.value]
    const value = (row: T, index: number): SortValue => {
      if (accessor) return accessor(row, index)
      const raw = (row as Record<string, unknown>)[sortKey.value]
      if (typeof raw === 'number') return raw
      if (raw == null) return ''
      if (Array.isArray(raw)) return raw.map(String).join(', ')
      return String(raw)
    }
    const dir = sortAsc.value ? 1 : -1
    rows.sort((a, b) => {
      const av = value(a.row, a.index)
      const bv = value(b.row, b.index)
      if (typeof av === 'number' && typeof bv === 'number') return (av - bv) * dir
      return String(av).localeCompare(String(bv), undefined, { numeric: true, sensitivity: 'base' }) * dir
    })
    return rows.map((r) => r.row)
  })

  return { sortKey, sortAsc, setSort, indicator, sorted }
}
