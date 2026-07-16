import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
  },
}))

import { getRuntimeMetrics } from '@/api/admin/ops'

describe('admin ops runtime metrics api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('读取只读运行时指标端点并返回快照', async () => {
    const response = {
      generated_at: '2026-07-16T10:00:00Z',
      request_stages: [],
      scheduling: [],
      caches: [],
      connection_pools: [],
    }
    get.mockResolvedValue({ data: response })

    await expect(getRuntimeMetrics()).resolves.toEqual(response)
    expect(get).toHaveBeenCalledWith('/admin/ops/runtime/metrics', { signal: undefined })
  })
})
