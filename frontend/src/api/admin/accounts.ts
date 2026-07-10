/**
 * Admin Accounts API endpoints
 * Handles AI platform account management for administrators
 */

import { apiClient } from '../client'
import type {
  Account,
  AccountStatusSummary,
  CreateAccountRequest,
  UpdateAccountRequest,
  PaginatedResponse,
  AccountUsageInfo,
  AccountPoolUsageSummary,
  AccountDashboardSummary,
  AccountActionItemsResponse,
  OpenAIAccountSchedulingPoolFilters,
  OpenAIAccountSchedulingPoolResponse,
  WindowStats,
  ClaudeModel,
  AccountUsageStatsResponse,
  UpstreamBalanceRefreshResult,
  TempUnschedulableStatus,
  AdminDataPayload,
  AdminDataImportResult,
  CodexSessionImportRequest,
  CodexSessionImportResult,
  CheckMixedChannelRequest,
  CheckMixedChannelResponse,
  AccountProbeRun,
  CreateAccountProbeRunRequest,
  CreateAccountModelProbeRunRequest,
  CreateBazaarLinkModelProbeRunRequest,
  BatchAccountModelProbeRunsRequest,
  BatchAccountModelProbeRunsResponse,
  AccountProbeRunListFilters,
  AccountProbeRankingItem,
  AccountProbeRunsResponse,
  BatchAccountProbeRunsRequest,
  BatchAccountProbeRunsResponse,
  DeleteAccountProbeRunsResponse,
  FetchOptions
} from '@/types'

const UPSTREAM_BALANCE_REFRESH_TIMEOUT_MS = 120000
const UPSTREAM_BALANCES_REFRESH_TIMEOUT_MS = 300000

export interface BatchTestNonAPIKeyAccountsRequest {
  model_id?: string
  platform?: string
  status?: string
  search?: string
  group?: string
  plan_type?: string
  account_ids?: number[]
  concurrency?: number
  limit?: number
}

export interface BatchTestNonAPIKeyAccountItem {
  id?: number
  run_id?: number
  account_id: number
  account_name: string
  platform: string
  type: string
  status: 'pending' | 'running' | 'success' | 'failed' | string
  category: 'ok' | 'unauthorized' | 'rate_limited' | 'timeout' | 'reauth_required' | 'error' | string
  message?: string
  error_message?: string
  latency_ms?: number
  first_token_ms?: number | null
  created_at?: string
  started_at?: string
  finished_at?: string
}

export interface BatchTestNonAPIKeyRun {
  id: number
  status: 'running' | 'success' | 'partial' | 'failed' | string
  model_id: string
  platform?: string
  status_filter?: string
  search?: string
  concurrency: number
  limit: number
  total: number
  success_count: number
  failed_count: number
  unauthorized_count: number
  rate_limited_count?: number
  error_message?: string
  created_at: string
  started_at?: string
  finished_at?: string
}

export interface BatchTestNonAPIKeyRunDetail extends BatchTestNonAPIKeyRun {
  items: BatchTestNonAPIKeyAccountItem[]
}

export interface BatchTestNonAPIKeyRunsResponse {
  items: BatchTestNonAPIKeyRun[]
  total: number
  page: number
  page_size: number
}

export interface AccountUsageSummaryFilters {
  platform?: string
  type?: string
  status?: string
  group?: string
  search?: string
  plan_type?: string
  privacy_mode?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

export type AccountStatusSummaryFilters = AccountUsageSummaryFilters
export type AccountDashboardSummaryFilters = AccountUsageSummaryFilters
export type AccountActionItemsFilters = Pick<
  AccountUsageSummaryFilters,
  'platform' | 'type' | 'status' | 'group' | 'search' | 'plan_type' | 'privacy_mode'
>

export type AccountSchedulingPoolFilters = OpenAIAccountSchedulingPoolFilters

const accountStatusSummaryStatuses = [
  'active',
  'rate_limited',
  'error',
  'inactive',
  'temp_unschedulable',
  'unschedulable',
] as const

/**
 * List all accounts with pagination
 * @param page - Page number (default: 1)
 * @param pageSize - Items per page (default: 20)
 * @param filters - Optional filters
 * @returns Paginated list of accounts
 */
export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    platform?: string
    type?: string
    status?: string
    group?: string
    search?: string
    plan_type?: string
    privacy_mode?: string
    lite?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<Account>> {
  const { data } = await apiClient.get<PaginatedResponse<Account>>('/admin/accounts', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    signal: options?.signal
  })
  return data
}

export interface AccountListWithEtagResult {
  notModified: boolean
  etag: string | null
  data: PaginatedResponse<Account> | null
}

export async function listWithEtag(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    platform?: string
    type?: string
    status?: string
    group?: string
    search?: string
    plan_type?: string
    privacy_mode?: string
    lite?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
    etag?: string | null
  }
): Promise<AccountListWithEtagResult> {
  const headers: Record<string, string> = {}
  if (options?.etag) {
    headers['If-None-Match'] = options.etag
  }

  const response = await apiClient.get<PaginatedResponse<Account>>('/admin/accounts', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    headers,
    signal: options?.signal,
    validateStatus: (status) => (status >= 200 && status < 300) || status === 304
  })

  const etagHeader = typeof response.headers?.etag === 'string' ? response.headers.etag : null
  if (response.status === 304) {
    return {
      notModified: true,
      etag: etagHeader,
      data: null
    }
  }

  return {
    notModified: false,
    etag: etagHeader,
    data: response.data
  }
}

/**
 * Get account by ID
 * @param id - Account ID
 * @returns Account details
 */
export async function getById(id: number): Promise<Account> {
  const { data } = await apiClient.get<Account>(`/admin/accounts/${id}`)
  return data
}

/**
 * Create new account
 * @param accountData - Account data
 * @returns Created account
 */
export async function create(accountData: CreateAccountRequest): Promise<Account> {
  const { data } = await apiClient.post<Account>('/admin/accounts', accountData)
  return data
}

/**
 * Update account
 * @param id - Account ID
 * @param updates - Fields to update
 * @returns Updated account
 */
export async function update(id: number, updates: UpdateAccountRequest): Promise<Account> {
  const { data } = await apiClient.put<Account>(`/admin/accounts/${id}`, updates)
  return data
}

/**
 * 按非敏感指纹删除账号保存的 API Key。
 */
export async function deleteAccountAPIKey(id: number, fingerprint: string): Promise<Account> {
  const { data } = await apiClient.delete<Account>(`/admin/accounts/${id}/api-keys/${encodeURIComponent(fingerprint)}`)
  return data
}

/**
 * 按非敏感指纹恢复账号保存的单个已停用 API Key。
 */
export async function restoreAccountAPIKeyState(id: number, fingerprint: string): Promise<Account> {
  const { data } = await apiClient.post<Account>(
    `/admin/accounts/${id}/api-keys/${encodeURIComponent(fingerprint)}/restore-state`
  )
  return data
}

/**
 * Check mixed-channel risk for account-group binding.
 */
export async function checkMixedChannelRisk(
  payload: CheckMixedChannelRequest
): Promise<CheckMixedChannelResponse> {
  const { data } = await apiClient.post<CheckMixedChannelResponse>('/admin/accounts/check-mixed-channel', payload)
  return data
}

/**
 * Delete account
 * @param id - Account ID
 * @returns Success confirmation
 */
export async function deleteAccount(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/accounts/${id}`)
  return data
}

/**
 * Toggle account status
 * @param id - Account ID
 * @param status - New status
 * @returns Updated account
 */
export async function toggleStatus(id: number, status: 'active' | 'inactive'): Promise<Account> {
  return update(id, { status })
}

/**
 * Test account connectivity
 * @param id - Account ID
 * @returns Test result
 */
export async function testAccount(id: number): Promise<{
  success: boolean
  message: string
  latency_ms?: number
}> {
  const { data } = await apiClient.post<{
    success: boolean
    message: string
    latency_ms?: number
  }>(`/admin/accounts/${id}/test`)
  return data
}

export async function batchTestNonAPIKeyAccounts(
  request: BatchTestNonAPIKeyAccountsRequest,
  options?: FetchOptions
): Promise<BatchTestNonAPIKeyRun> {
  const { data } = await apiClient.post<BatchTestNonAPIKeyRun>(
    '/admin/accounts/batch-test-non-apikey',
    request,
    {
      timeout: 30000,
      signal: options?.signal,
    }
  )
  return data
}

export async function listBatchTestNonAPIKeyRuns(
  page: number = 1,
  pageSize: number = 20,
  filters?: { status?: string; keyword?: string },
  options?: FetchOptions
): Promise<BatchTestNonAPIKeyRunsResponse> {
  const { data } = await apiClient.get<BatchTestNonAPIKeyRunsResponse>('/admin/accounts/batch-test-runs', {
    params: {
      page,
      page_size: pageSize,
      status: filters?.status || undefined,
      keyword: filters?.keyword || undefined,
    },
    signal: options?.signal,
  })
  return data
}

export async function getBatchTestNonAPIKeyRun(
  runId: number,
  filters?: { category?: string },
  options?: FetchOptions
): Promise<BatchTestNonAPIKeyRunDetail> {
  const { data } = await apiClient.get<BatchTestNonAPIKeyRunDetail>(`/admin/accounts/batch-test-runs/${runId}`, {
    params: {
      category: filters?.category || undefined,
    },
    signal: options?.signal,
  })
  return data
}

export async function createProbeRun(
  id: number,
  payload: CreateAccountProbeRunRequest
): Promise<AccountProbeRun> {
  const { data } = await apiClient.post<AccountProbeRun>(`/admin/accounts/${id}/probe-runs`, payload)
  return data
}

export async function listProbeRuns(id: number, limit = 20): Promise<AccountProbeRun[]> {
  const { data } = await apiClient.get<AccountProbeRun[]>(`/admin/accounts/${id}/probe-runs`, {
    params: { limit }
  })
  return data
}

export async function getProbeRun(id: number, runId: number): Promise<AccountProbeRun> {
  const { data } = await apiClient.get<AccountProbeRun>(`/admin/accounts/${id}/probe-runs/${runId}`)
  return data
}

export async function listAccountProbeRuns(
  page: number = 1,
  pageSize: number = 20,
  filters?: AccountProbeRunListFilters,
  options?: FetchOptions
): Promise<AccountProbeRunsResponse> {
  const { data } = await apiClient.get<AccountProbeRunsResponse>('/admin/account-probe-runs', {
    params: {
      page,
      page_size: pageSize,
      ...filters,
    },
    signal: options?.signal,
  })
  return data
}

export async function getAccountProbeRun(
  runId: number,
  options?: FetchOptions
): Promise<AccountProbeRun> {
  const { data } = await apiClient.get<AccountProbeRun>(`/admin/account-probe-runs/${runId}`, {
    signal: options?.signal,
  })
  return data
}

export async function listAccountProbeRanking(
  limit = 20,
  options?: FetchOptions
): Promise<AccountProbeRankingItem[]> {
  const { data } = await apiClient.get<AccountProbeRankingItem[]>('/admin/account-probe-runs/ranking', {
    params: { limit },
    signal: options?.signal,
  })
  return data ?? []
}

export async function batchAccountProbeRuns(
  payload: BatchAccountProbeRunsRequest,
  options?: FetchOptions
): Promise<BatchAccountProbeRunsResponse> {
  const { data } = await apiClient.post<BatchAccountProbeRunsResponse>(
    '/admin/account-probe-runs/batch',
    payload,
    {
      signal: options?.signal,
    }
  )
  return data
}

export async function createAccountModelProbeRun(
  payload: CreateAccountModelProbeRunRequest,
  options?: FetchOptions
): Promise<AccountProbeRun> {
  const { data } = await apiClient.post<AccountProbeRun>(
    '/admin/account-model-probe-runs',
    payload,
    {
      signal: options?.signal,
    }
  )
  return data
}

export async function createBazaarLinkModelProbeRun(
  payload: CreateBazaarLinkModelProbeRunRequest,
  options?: FetchOptions
): Promise<AccountProbeRun> {
  const { data } = await apiClient.post<AccountProbeRun>(
    '/admin/account-model-probe-runs/bazaarlink',
    payload,
    {
      signal: options?.signal,
    }
  )
  return data
}

export async function batchAccountModelProbeRuns(
  payload: BatchAccountModelProbeRunsRequest,
  options?: FetchOptions
): Promise<BatchAccountModelProbeRunsResponse> {
  const { data } = await apiClient.post<BatchAccountModelProbeRunsResponse>(
    '/admin/account-model-probe-runs/batch',
    payload,
    {
      signal: options?.signal,
    }
  )
  return data
}

export async function deleteAccountProbeRuns(
  runIds: number[],
  options?: FetchOptions
): Promise<DeleteAccountProbeRunsResponse> {
  const { data } = await apiClient.delete<DeleteAccountProbeRunsResponse>('/admin/account-probe-runs', {
    data: { run_ids: runIds },
    signal: options?.signal,
  })
  return data
}

/**
 * Refresh account credentials
 * @param id - Account ID
 * @returns Updated account
 */
export async function refreshCredentials(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/refresh`)
  return data
}

/**
 * Get account usage statistics
 * @param id - Account ID
 * @param days - Number of days (default: 30)
 * @returns Account usage statistics with history, summary, and models
 */
export async function getStats(id: number, days: number = 30): Promise<AccountUsageStatsResponse> {
  const { data } = await apiClient.get<AccountUsageStatsResponse>(`/admin/accounts/${id}/stats`, {
    params: { days }
  })
  return data
}

/**
 * Clear account error
 * @param id - Account ID
 * @returns Updated account
 */
export async function clearError(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/clear-error`)
  return data
}

/**
 * Get account usage information (5h/7d window)
 * @param id - Account ID
 * @returns Account usage info
 */
export async function getUsage(id: number, source?: 'passive' | 'active', force?: boolean): Promise<AccountUsageInfo> {
  const params: Record<string, string> = {}
  if (source) params.source = source
  if (force) params.force = 'true'
  const { data } = await apiClient.get<AccountUsageInfo>(`/admin/accounts/${id}/usage`, {
    params: Object.keys(params).length > 0 ? params : undefined
  })
  return data
}

/**
 * Get aggregated OpenAI account usage across the current account filters.
 */
export async function getUsageSummary(
  filters?: AccountUsageSummaryFilters,
  options?: {
    signal?: AbortSignal
  }
): Promise<AccountPoolUsageSummary> {
  const { data } = await apiClient.get<AccountPoolUsageSummary>('/admin/accounts/usage-summary', {
    params: filters,
    signal: options?.signal
  })
  return data
}

/**
 * Get account status totals across the current non-status filters.
 */
export async function getStatusSummary(
  filters?: AccountStatusSummaryFilters,
  options?: {
    signal?: AbortSignal
  }
): Promise<AccountStatusSummary> {
  const { status: _status, sort_by: _sortBy, sort_order: _sortOrder, ...baseFilters } = filters ?? {}
  const [totalResult, entries] = await Promise.all([
    list(1, 1, {
      ...baseFilters,
      lite: '1',
    }, {
      signal: options?.signal
    }),
    Promise.all(accountStatusSummaryStatuses.map(async (status) => {
      const result = await list(1, 1, {
        ...baseFilters,
        status,
        lite: '1',
      }, {
        signal: options?.signal
      })
      return [status, result.total || 0] as const
    }))
  ])
  return {
    total: totalResult.total || 0,
    ...Object.fromEntries(entries),
  } as unknown as AccountStatusSummary
}

/**
 * Get account dashboard summary across the current account filters.
 * This is the preferred first-screen summary endpoint; legacy summary methods
 * stay exported as fallback paths.
 */
export async function getDashboardSummary(
  filters?: AccountDashboardSummaryFilters,
  options?: {
    signal?: AbortSignal
  }
): Promise<AccountDashboardSummary> {
  const { data } = await apiClient.get<AccountDashboardSummary>('/admin/accounts/dashboard-summary', {
    params: filters,
    signal: options?.signal
  })
  return data
}

/**
 * Get prioritized account action items for the current account filters.
 */
export async function getActionItems(
  filters?: AccountActionItemsFilters,
  options?: {
    signal?: AbortSignal
  }
): Promise<AccountActionItemsResponse> {
  const { data } = await apiClient.get<AccountActionItemsResponse>('/admin/accounts/action-items', {
    params: filters,
    signal: options?.signal
  })
  return data
}

/**
 * Get the current OpenAI account scheduling pool snapshot.
 */
export async function listSchedulingPool(
  filters?: AccountSchedulingPoolFilters,
  options?: {
    signal?: AbortSignal
  }
): Promise<OpenAIAccountSchedulingPoolResponse> {
  const { data } = await apiClient.get<OpenAIAccountSchedulingPoolResponse>('/admin/accounts/scheduling-pool', {
    params: filters,
    signal: options?.signal
  })
  return data
}

export async function refreshUpstreamBalances(): Promise<UpstreamBalanceRefreshResult> {
  const { data } = await apiClient.post<UpstreamBalanceRefreshResult>(
    '/admin/accounts/refresh-upstream-balances',
    undefined,
    { timeout: UPSTREAM_BALANCES_REFRESH_TIMEOUT_MS }
  )
  return data
}

export async function refreshUpstreamBalance(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(
    `/admin/accounts/${id}/refresh-upstream-balance`,
    undefined,
    { timeout: UPSTREAM_BALANCE_REFRESH_TIMEOUT_MS }
  )
  return data
}

/**
 * Clear account rate limit status
 * @param id - Account ID
 * @returns Updated account
 */
export async function clearRateLimit(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(
    `/admin/accounts/${id}/clear-rate-limit`
  )
  return data
}

/**
 * Recover account runtime state in one call
 * @param id - Account ID
 * @returns Updated account
 */
export async function recoverState(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/recover-state`)
  return data
}

/**
 * Reset account quota usage
 * @param id - Account ID
 * @returns Updated account
 */
export async function resetAccountQuota(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(
    `/admin/accounts/${id}/reset-quota`
  )
  return data
}

/**
 * Get temporary unschedulable status
 * @param id - Account ID
 * @returns Status with detail state if active
 */
export async function getTempUnschedulableStatus(id: number): Promise<TempUnschedulableStatus> {
  const { data } = await apiClient.get<TempUnschedulableStatus>(
    `/admin/accounts/${id}/temp-unschedulable`
  )
  return data
}

/**
 * Reset temporary unschedulable status
 * @param id - Account ID
 * @returns Success confirmation
 */
export async function resetTempUnschedulable(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(
    `/admin/accounts/${id}/temp-unschedulable`
  )
  return data
}

/**
 * Generate OAuth authorization URL
 * @param endpoint - API endpoint path
 * @param config - Proxy configuration
 * @returns Auth URL and session ID
 */
export async function generateAuthUrl(
  endpoint: string,
  config: { proxy_id?: number }
): Promise<{ auth_url: string; session_id: string }> {
  const { data } = await apiClient.post<{ auth_url: string; session_id: string }>(endpoint, config)
  return data
}

/**
 * Exchange authorization code for tokens
 * @param endpoint - API endpoint path
 * @param exchangeData - Session ID, code, and optional proxy config
 * @returns Token information
 */
export async function exchangeCode(
  endpoint: string,
  exchangeData: { session_id: string; code: string; state?: string; proxy_id?: number }
): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>(endpoint, exchangeData)
  return data
}

/**
 * Batch create accounts
 * @param accounts - Array of account data
 * @returns Results of batch creation
 */
export async function batchCreate(accounts: CreateAccountRequest[]): Promise<{
  success: number
  failed: number
  results: Array<{ success: boolean; account?: Account; error?: string }>
}> {
  const { data } = await apiClient.post<{
    success: number
    failed: number
    results: Array<{ success: boolean; account?: Account; error?: string }>
  }>('/admin/accounts/batch', { accounts })
  return data
}

/**
 * Batch update credentials fields for multiple accounts
 * @param request - Batch update request containing account IDs, field name, and value
 * @returns Results of batch update
 */
export async function batchUpdateCredentials(request: {
  account_ids: number[]
  field: string
  value: any
}): Promise<{
  success: number
  failed: number
  results: Array<{ account_id: number; success: boolean; error?: string }>
}> {
  const { data } = await apiClient.post<{
    success: number
    failed: number
    results: Array<{ account_id: number; success: boolean; error?: string }>
  }>('/admin/accounts/batch-update-credentials', request)
  return data
}

/**
 * Bulk update multiple accounts
 * @param accountIds - Array of account IDs
 * @param updates - Fields to update
 * @returns Success confirmation
 */
export async function bulkUpdate(
  accountIdsOrPayload: number[] | Record<string, unknown>,
  updates?: Record<string, unknown>
): Promise<{
  success: number
  failed: number
  success_ids?: number[]
  failed_ids?: number[]
  results: Array<{ account_id: number; success: boolean; error?: string }>
  }> {
  const payload = Array.isArray(accountIdsOrPayload)
    ? {
        account_ids: accountIdsOrPayload,
        ...(updates ?? {})
      }
    : accountIdsOrPayload
  const { data } = await apiClient.post<{
    success: number
    failed: number
    success_ids?: number[]
    failed_ids?: number[]
    results: Array<{ account_id: number; success: boolean; error?: string }>
  }>('/admin/accounts/bulk-update', payload)
  return data
}

/**
 * Get account today statistics
 * @param id - Account ID
 * @returns Today's stats (requests, tokens, cost)
 */
export async function getTodayStats(id: number): Promise<WindowStats> {
  const { data } = await apiClient.get<WindowStats>(`/admin/accounts/${id}/today-stats`)
  return data
}

export interface BatchTodayStatsResponse {
  stats: Record<string, WindowStats>
}

/**
 * 批量获取多个账号的今日统计
 * @param accountIds - 账号 ID 列表
 * @returns 以账号 ID（字符串）为键的统计映射
 */
export async function getBatchTodayStats(accountIds: number[]): Promise<BatchTodayStatsResponse> {
  const { data } = await apiClient.post<BatchTodayStatsResponse>('/admin/accounts/today-stats/batch', {
    account_ids: accountIds
  })
  return data
}

/**
 * Set account schedulable status
 * @param id - Account ID
 * @param schedulable - Whether the account should participate in scheduling
 * @returns Updated account
 */
export async function setSchedulable(id: number, schedulable: boolean): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/schedulable`, {
    schedulable
  })
  return data
}

/**
 * Get available models for an account
 * @param id - Account ID
 * @returns List of available models for this account
 */
export async function getAvailableModels(id: number): Promise<ClaudeModel[]> {
  const { data } = await apiClient.get<ClaudeModel[]>(`/admin/accounts/${id}/models`)
  return data
}

export interface SyncUpstreamModelsResult {
  models: string[]
}

/**
 * Sync live supported models from the account's upstream model-list endpoint
 * @param id - Account ID
 * @returns List of model IDs returned by the upstream
 */
export async function syncUpstreamModels(id: number): Promise<SyncUpstreamModelsResult> {
  const { data } = await apiClient.post<SyncUpstreamModelsResult>(`/admin/accounts/${id}/models/sync-upstream`)
  return data
}

export interface CRSPreviewAccount {
  crs_account_id: string
  kind: string
  name: string
  platform: string
  type: string
}

export interface PreviewFromCRSResult {
  new_accounts: CRSPreviewAccount[]
  existing_accounts: CRSPreviewAccount[]
}

export async function previewFromCrs(params: {
  base_url: string
  username: string
  password: string
}): Promise<PreviewFromCRSResult> {
  const { data } = await apiClient.post<PreviewFromCRSResult>('/admin/accounts/sync/crs/preview', params)
  return data
}

export async function syncFromCrs(params: {
  base_url: string
  username: string
  password: string
  sync_proxies?: boolean
  selected_account_ids?: string[]
}): Promise<{
  created: number
  updated: number
  skipped: number
  failed: number
  items: Array<{
    crs_account_id: string
    kind: string
    name: string
    action: string
    error?: string
  }>
}> {
  const { data } = await apiClient.post<{
    created: number
    updated: number
    skipped: number
    failed: number
    items: Array<{
      crs_account_id: string
      kind: string
      name: string
      action: string
      error?: string
    }>
  }>('/admin/accounts/sync/crs', params, {
    timeout: 180000
  })
  return data
}

export async function exportData(options?: {
  ids?: number[]
  filters?: {
    platform?: string
    type?: string
    status?: string
    group?: string
    privacy_mode?: string
    plan_type?: string
    search?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  }
  includeProxies?: boolean
}): Promise<AdminDataPayload> {
  const params: Record<string, string> = {}
  if (options?.ids && options.ids.length > 0) {
    params.ids = options.ids.join(',')
  } else if (options?.filters) {
    const { platform, type, status, group, privacy_mode, plan_type, search, sort_by, sort_order } = options.filters
    if (platform) params.platform = platform
    if (type) params.type = type
    if (status) params.status = status
    if (group) params.group = group
    if (privacy_mode) params.privacy_mode = privacy_mode
    if (plan_type) params.plan_type = plan_type
    if (search) params.search = search
    if (sort_by) params.sort_by = sort_by
    if (sort_order) params.sort_order = sort_order
  }
  if (options?.includeProxies === false) {
    params.include_proxies = 'false'
  }
  const { data } = await apiClient.get<AdminDataPayload>('/admin/accounts/data', { params })
  return data
}

export async function importData(payload: {
  data: AdminDataPayload
  skip_default_group_bind?: boolean
}): Promise<AdminDataImportResult> {
  const { data } = await apiClient.post<AdminDataImportResult>('/admin/accounts/data', {
    data: payload.data,
    skip_default_group_bind: payload.skip_default_group_bind
  })
  return data
}

export async function importCodexSession(payload: CodexSessionImportRequest): Promise<CodexSessionImportResult> {
  const { data } = await apiClient.post<CodexSessionImportResult>('/admin/accounts/import/codex-session', payload)
  return data
}

/**
 * Get Antigravity default model mapping from backend
 * @returns Default model mapping (from -> to)
 */
export async function getAntigravityDefaultModelMapping(): Promise<Record<string, string>> {
  const { data } = await apiClient.get<Record<string, string>>(
    '/admin/accounts/antigravity/default-model-mapping'
  )
  return data
}

/**
 * Refresh OpenAI token using refresh token
 * @param refreshToken - The refresh token
 * @param proxyId - Optional proxy ID
 * @returns Token information including access_token, email, etc.
 */
export async function refreshOpenAIToken(
  refreshToken: string,
  proxyId?: number | null,
  endpoint: string = '/admin/openai/refresh-token',
  clientId?: string
): Promise<Record<string, unknown>> {
  const payload: { refresh_token: string; proxy_id?: number; client_id?: string } = {
    refresh_token: refreshToken
  }
  if (proxyId) {
    payload.proxy_id = proxyId
  }
  if (clientId) {
    payload.client_id = clientId
  }
  const { data } = await apiClient.post<Record<string, unknown>>(endpoint, payload)
  return data
}

/**
 * Batch operation result type
 */
export interface BatchOperationResult {
  total: number
  success: number
  failed: number
  errors?: Array<{ account_id: number; error: string }>
  warnings?: Array<{ account_id: number; warning: string }>
  accounts?: Account[]
}

/**
 * Batch clear account errors
 * @param accountIds - Array of account IDs
 * @returns Batch operation result
 */
export async function batchClearError(accountIds: number[]): Promise<BatchOperationResult> {
  const { data } = await apiClient.post<BatchOperationResult>('/admin/accounts/batch-clear-error', {
    account_ids: accountIds
  })
  return data
}

/**
 * Batch refresh account credentials
 * @param accountIds - Array of account IDs
 * @returns Batch operation result
 */
export async function batchRefresh(accountIds: number[]): Promise<BatchOperationResult> {
  const { data } = await apiClient.post<BatchOperationResult>('/admin/accounts/batch-refresh', {
    account_ids: accountIds,
  }, {
    timeout: 120000  // 120s timeout for large batch refreshes
  })
  return data
}

/**
 * Set privacy for an Antigravity OAuth account
 * @param id - Account ID
 * @returns Updated account
 */
export async function setPrivacy(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/set-privacy`)
  return data
}

/**
 * Manually probe an account and recover its state on success
 * @param id - Account ID
 * @param payload - Optional probe parameters (model, prompt, mode)
 * @returns Probe result and updated account if successful
 */
export async function manualProbeAccount(
  id: number,
  payload?: {
    model?: string
    prompt?: string
    mode?: string
  }
): Promise<{
  success: boolean
  result?: {
    success: boolean
    message?: string
    latency_ms?: number
    first_token_ms?: number
    http_status?: number
    reason?: string
    error?: string
  }
  account?: Account
}> {
  const { data } = await apiClient.post(`/admin/accounts/${id}/manual-probe`, payload ?? {})
  return data
}

export const accountsAPI = {
  list,
  listWithEtag,
  getById,
  create,
  update,
  deleteAccountAPIKey,
  restoreAccountAPIKeyState,
  checkMixedChannelRisk,
  delete: deleteAccount,
  toggleStatus,
  testAccount,
  batchTestNonAPIKeyAccounts,
  listBatchTestNonAPIKeyRuns,
  getBatchTestNonAPIKeyRun,
  createProbeRun,
  listProbeRuns,
  getProbeRun,
  listAccountProbeRuns,
  getAccountProbeRun,
  listAccountProbeRanking,
  batchAccountProbeRuns,
  createAccountModelProbeRun,
  createBazaarLinkModelProbeRun,
  batchAccountModelProbeRuns,
  refreshCredentials,
  getStats,
  clearError,
  getUsage,
  getUsageSummary,
  getStatusSummary,
  getDashboardSummary,
  getActionItems,
  listSchedulingPool,
  refreshUpstreamBalances,
  refreshUpstreamBalance,
  getTodayStats,
  getBatchTodayStats,
  clearRateLimit,
  recoverState,
  resetAccountQuota,
  getTempUnschedulableStatus,
  resetTempUnschedulable,
  setSchedulable,
  getAvailableModels,
  syncUpstreamModels,
  generateAuthUrl,
  exchangeCode,
  refreshOpenAIToken,
  batchCreate,
  batchUpdateCredentials,
  bulkUpdate,
  previewFromCrs,
  syncFromCrs,
  exportData,
  importData,
  importCodexSession,
  getAntigravityDefaultModelMapping,
  batchClearError,
  batchRefresh,
  setPrivacy
}

export default accountsAPI
