/**
 * Common component types
 */

export interface Column {
  key: string
  label: string
  sortable?: boolean
  /** 首次点击该列时使用的排序方向，未配置时保持升序。 */
  defaultSortOrder?: 'asc' | 'desc'
  class?: string
  formatter?: (value: any, row: any) => string
}
