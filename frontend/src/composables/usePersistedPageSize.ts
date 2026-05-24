import { getConfiguredTableDefaultPageSize, normalizeTablePageSize } from '@/utils/tablePreferences'

const STORAGE_KEY = 'table-page-size'

export function getPersistedPageSize(fallback = getConfiguredTableDefaultPageSize()): number {
  if (typeof window !== 'undefined') {
    try {
      const configuredDefault = getConfiguredTableDefaultPageSize()
      const storedSource = window.localStorage.getItem('table-page-size-source')
      const stored = window.localStorage.getItem(STORAGE_KEY)
      if (storedSource === 'user' && stored !== null && normalizeTablePageSize(stored) !== configuredDefault) {
        window.localStorage.removeItem(STORAGE_KEY)
        window.localStorage.removeItem('table-page-size-source')
        return normalizeTablePageSize(configuredDefault)
      }

      if (stored !== null) {
        const parsed = Number(stored)
        if (Number.isFinite(parsed)) {
          return normalizeTablePageSize(parsed)
        }
      }
    } catch (error) {
      console.warn('Failed to read persisted page size:', error)
    }
  }
  return normalizeTablePageSize(getConfiguredTableDefaultPageSize() || fallback)
}

export function setPersistedPageSize(size: number): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, String(size))
  } catch (error) {
    console.warn('Failed to persist page size:', error)
  }
}
