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

import { getStatusSummary, getUsageSummary, list } from '@/api/admin/accounts'

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

    expect(get).toHaveBeenCalledTimes(5)
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
    ])
    expect(result).toEqual({
      active: 8,
      rate_limited: 2,
      error: 3,
      inactive: 1,
      temp_unschedulable: 4,
    })
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
