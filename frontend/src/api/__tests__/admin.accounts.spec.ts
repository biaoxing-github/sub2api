import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, deleteRequest } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  deleteRequest: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
    put,
    delete: deleteRequest,
  },
}))

import {
  getActionItems,
  getDashboardSummary,
  getStatusSummary,
  getUsageSummary,
  list
} from '@/api/admin/accounts'

describe('admin accounts api usage summary', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('loads the usage summary with account list filters', async () => {
    const response = {
      generated_at: '2026-05-22T01:00:00Z',
      total_accounts: 2,
      schedulable_accounts: 1,
      rate_limited_accounts: 0,
      missing_snapshot_accounts: 1,
      five_hour: {
        used_cost: 1,
        estimated_limit_cost: 4,
        utilization: 25,
        requests: 3,
        accounts_with_snapshot: 1,
        accounts_with_limit_estimate: 1,
      },
      seven_day: {
        used_cost: 2,
        estimated_limit_cost: 10,
        utilization: 20,
        requests: 6,
        accounts_with_snapshot: 1,
        accounts_with_limit_estimate: 1,
      },
      plans: [],
    }
    get.mockResolvedValue({ data: response })

    const result = await getUsageSummary({
      platform: 'openai',
      type: 'oauth',
      status: 'active',
      group: '12',
      search: 'free',
      plan_type: 'free',
      privacy_mode: 'training_off',
      sort_by: 'name',
      sort_order: 'asc',
    })

    expect(get).toHaveBeenCalledWith('/admin/accounts/usage-summary', {
      params: {
        platform: 'openai',
        type: 'oauth',
        status: 'active',
        group: '12',
        search: 'free',
        plan_type: 'free',
        privacy_mode: 'training_off',
        sort_by: 'name',
        sort_order: 'asc',
      },
      signal: undefined,
    })
    expect(result).toEqual(response)
  })

  it('loads status summary totals without carrying the active status filter', async () => {
    get.mockResolvedValueOnce({ data: { total: 8 } })
    get.mockResolvedValueOnce({ data: { total: 2 } })
    get.mockResolvedValueOnce({ data: { total: 3 } })
    get.mockResolvedValueOnce({ data: { total: 1 } })
    get.mockResolvedValueOnce({ data: { total: 4 } })
    get.mockResolvedValueOnce({ data: { total: 5 } })

    const result = await getStatusSummary({
      platform: 'openai',
      type: 'oauth',
      status: 'rate_limited',
      group: '12',
      search: 'free',
      plan_type: 'plus',
      privacy_mode: 'training_off',
      sort_by: 'name',
      sort_order: 'asc',
    })

    expect(get).toHaveBeenCalledTimes(6)
    expect(get).toHaveBeenNthCalledWith(1, '/admin/accounts', expect.objectContaining({
      params: expect.objectContaining({
        page: 1,
        page_size: 1,
        platform: 'openai',
        type: 'oauth',
        group: '12',
        search: 'free',
        plan_type: 'plus',
        privacy_mode: 'training_off',
        status: 'active',
        lite: '1',
      })
    }))
    expect(get.mock.calls.map((call) => call[1]?.params?.status)).toEqual([
      'active',
      'rate_limited',
      'error',
      'inactive',
      'temp_unschedulable',
      'unschedulable',
    ])
    expect(result).toEqual({
      active: 8,
      rate_limited: 2,
      error: 3,
      inactive: 1,
      temp_unschedulable: 4,
      unschedulable: 5,
    })
  })

  it('loads dashboard summary with account list filters in one request', async () => {
    const response = {
      generated_at: '2026-05-24T10:00:00Z',
      status_summary: {
        active: 10,
        rate_limited: 2,
        error: 1,
        inactive: 3,
        temp_unschedulable: 1,
        unschedulable: 4,
      },
      usage_summary: null,
      balance_summary: {
        healthy: 8,
        draining: 2,
        exhausted: 1,
        balance_unknown: 3,
        missing_snapshot: 4,
      },
      action_item_counts: {
        critical: 2,
        warning: 5,
        info: 4,
      },
    }
    get.mockResolvedValue({ data: response })

    const result = await getDashboardSummary({
      platform: 'openai',
      type: 'oauth',
      status: 'active',
      group: '12',
      search: 'free',
      plan_type: 'free',
      privacy_mode: 'training_off',
      sort_by: 'name',
      sort_order: 'asc',
    })

    expect(get).toHaveBeenCalledTimes(1)
    expect(get).toHaveBeenCalledWith('/admin/accounts/dashboard-summary', {
      params: {
        platform: 'openai',
        type: 'oauth',
        status: 'active',
        group: '12',
        search: 'free',
        plan_type: 'free',
        privacy_mode: 'training_off',
        sort_by: 'name',
        sort_order: 'asc',
      },
      signal: undefined,
    })
    expect(result).toEqual(response)
  })

  it('loads action items with account filters', async () => {
    const response = {
      generated_at: '2026-05-24T10:00:00Z',
      items: [
        {
          account_id: 12,
          account_name: 'openai-a',
          severity: 'critical',
          reason: 'header_timeout_spike',
          summary: '5 header timeouts in the last hour',
          suggested_action: 'Pause or refresh balance',
          status: 'active',
          schedulable: true,
        },
      ],
    }
    get.mockResolvedValue({ data: response })

    const result = await getActionItems({
      platform: 'openai',
      group: '12',
    })

    expect(get).toHaveBeenCalledWith('/admin/accounts/action-items', {
      params: {
        platform: 'openai',
        group: '12',
      },
      signal: undefined,
    })
    expect(result).toEqual(response)
  })

  it('passes plan type when listing accounts', async () => {
    get.mockResolvedValue({
      data: {
        items: [],
        total: 0,
        page: 1,
        page_size: 20,
        pages: 0,
      }
    })

    await list(1, 20, {
      platform: 'openai',
      type: 'oauth',
      plan_type: 'free',
    })

    expect(get).toHaveBeenCalledWith('/admin/accounts', expect.objectContaining({
      params: expect.objectContaining({
        page: 1,
        page_size: 20,
        platform: 'openai',
        type: 'oauth',
        plan_type: 'free',
      })
    }))
  })
})
