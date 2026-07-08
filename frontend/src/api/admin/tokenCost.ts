/**
 * Admin Token Cost API endpoints
 * Handles the database-backed API Token balance calculator state.
 */

import { apiClient } from '../client'
import type {
  FetchOptions,
  TokenCostHealth,
  TokenCostSaveStateResponse,
  TokenCostState,
  TokenCostStateResponse
} from '@/types'

function noCacheParams(): Record<string, number> {
  return { _: Date.now() }
}

/**
 * Load the latest calculator state from the database.
 * @param options - Request cancellation options
 * @returns Latest persisted calculator state
 */
export async function getState(options?: FetchOptions): Promise<TokenCostState> {
  const { data } = await apiClient.get<TokenCostStateResponse>('/admin/token-cost/state', {
    params: noCacheParams(),
    signal: options?.signal,
    headers: {
      'Cache-Control': 'no-cache',
      Pragma: 'no-cache'
    }
  })
  if (!data?.ok || !data.state) {
    throw new Error('Token cost state response is invalid')
  }
  return data.state
}

/**
 * Persist the current calculator state and return the normalized state.
 * @param state - Full editable calculator state
 * @returns Normalized state saved by the backend
 */
export async function saveState(state: TokenCostState): Promise<TokenCostState> {
  const { data } = await apiClient.post<TokenCostSaveStateResponse>('/admin/token-cost/state', { state })
  if (!data?.ok || !data.state || typeof data.state === 'string') {
    throw new Error('Token cost save response is invalid')
  }
  return data.state
}

/**
 * Check token-cost storage health.
 * @param options - Request cancellation options
 * @returns Storage health payload
 */
export async function health(options?: FetchOptions): Promise<TokenCostHealth> {
  const { data } = await apiClient.get<TokenCostHealth>('/admin/token-cost/health', {
    params: noCacheParams(),
    signal: options?.signal,
    headers: {
      'Cache-Control': 'no-cache',
      Pragma: 'no-cache'
    }
  })
  return data
}

export const tokenCostAPI = {
  getState,
  saveState,
  health
}

export default tokenCostAPI
