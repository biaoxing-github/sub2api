import type { UsageRequestType } from '@/types'

export type UsageRequestKind = UsageRequestType

export function statusCodeBadgeClass(code: number): string {
  if (code >= 500) return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
  if (code === 429) return 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
  if (code >= 400) return 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200'
  return 'bg-gray-100 text-gray-800 dark:bg-dark-700 dark:text-gray-200'
}

export function requestTypeBadgeClass(kind: UsageRequestKind): string {
  if (kind === 'cyber') return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
  if (kind === 'ws_v2') return 'bg-violet-100 text-violet-800 dark:bg-violet-900 dark:text-violet-200'
  if (kind === 'stream') return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
  if (kind === 'sync') return 'bg-gray-100 text-gray-800 dark:bg-dark-700 dark:text-gray-200'
  return 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200'
}

export function requestTypeLabelKey(kind: UsageRequestKind): string {
  if (kind === 'cyber') return 'usage.cyber'
  if (kind === 'ws_v2') return 'usage.ws'
  if (kind === 'stream') return 'usage.stream'
  if (kind === 'sync') return 'usage.sync'
  return 'usage.unknown'
}

export function numericRequestTypeKind(
  requestType?: number | null,
  stream?: boolean | null
): UsageRequestKind | null {
  const normalized = requestType ?? (stream == null ? 0 : stream ? 2 : 1)
  if (normalized === 3) return 'ws_v2'
  if (normalized === 2) return 'stream'
  if (normalized === 1) return 'sync'
  return null
}

export function mapErrorSortKey(key: string): string {
  return key === 'status' ? 'status_code' : key
}

export const COMMON_ERROR_STATUS_CODES = [400, 401, 403, 404, 408, 413, 429, 499, 500, 502, 503, 504, 529]
