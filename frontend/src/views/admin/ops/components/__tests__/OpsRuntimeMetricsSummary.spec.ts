import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpsRuntimeMetricsSummary from '../OpsRuntimeMetricsSummary.vue'

const mockGetRuntimeMetrics = vi.fn()

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    getRuntimeMetrics: (...args: unknown[]) => mockGetRuntimeMetrics(...args),
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (!params) return key
        return `${key} ${Object.values(params).join(' ')}`
      },
    }),
  }
})

// runtimeSnapshot 覆盖阶段聚合、缓存命中、failover 与连接池摘要。
const runtimeSnapshot = {
  generated_at: '2026-07-16T10:00:00Z',
  request_stages: [
    {
      stage: 'selection',
      result: 'success',
      protocol: 'sse',
      error_class: 'none',
      count: 3,
      total_duration_ms: 30,
      max_duration_ms: 12,
    },
    {
      stage: 'selection',
      result: 'failure',
      protocol: 'sse',
      error_class: 'internal',
      count: 1,
      total_duration_ms: 20,
      max_duration_ms: 20,
    },
  ],
  scheduling: [
    { name: 'scheduler', event: 'failover', count: 3 },
    { name: 'scheduler', event: 'snapshot_hit', count: 9 },
    { name: 'scheduler', event: 'snapshot_miss', count: 1 },
    { name: 'scheduler', event: 'slot_wait', count: 5 },
    { name: 'scheduler', event: 'slot_wait_timeout', count: 1 },
  ],
  caches: [
    { name: 'billing', event: 'hit', count: 8 },
    { name: 'billing', event: 'miss', count: 2 },
    { name: 'billing', event: 'write_drop', count: 1 },
    { name: 'billing', event: 'write_error', count: 1 },
  ],
  connection_pools: [
    { name: 'upstream_http', event: 'acquire', count: 10 },
    { name: 'upstream_http', event: 'reuse', count: 8 },
    { name: 'upstream_http', event: 'error', count: 1 },
    { name: 'upstream_http', event: 'wait_timeout', count: 2 },
  ],
}

describe('OpsRuntimeMetricsSummary', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetRuntimeMetrics.mockResolvedValue(runtimeSnapshot)
  })

  it('仅在激活后加载并聚合四类只读运行时摘要', async () => {
    const wrapper = mount(OpsRuntimeMetricsSummary, {
      props: { active: false, refreshKey: 'request-1' },
    })

    expect(mockGetRuntimeMetrics).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="ops-runtime-metrics-summary"]').exists()).toBe(false)

    await wrapper.setProps({ active: true })
    await flushPromises()

    expect(mockGetRuntimeMetrics).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="ops-runtime-metrics-summary"]').exists()).toBe(true)

    const stage = wrapper.get('[data-testid="runtime-stage-row"]').text()
    expect(stage).toContain('selection')
    expect(stage).toContain('12.5 ms')
    expect(stage).toContain('4')
    expect(stage).toContain('1')

    const cache = wrapper.get('[data-testid="runtime-cache-row"]').text()
    expect(cache).toContain('billing')
    expect(cache).toContain('80%')
    expect(cache).toContain('8')
    expect(cache).toContain('2')

    const pool = wrapper.get('[data-testid="runtime-connection-pool-row"]').text()
    expect(pool).toContain('upstream_http')
    expect(pool).toContain('10')
    expect(pool).toContain('8')

    expect(wrapper.text()).toContain('90%')
    expect(wrapper.text()).toContain('admin.ops.runtimeMetrics.failovers')

    await wrapper.setProps({ refreshKey: 'request-2' })
    await flushPromises()
    expect(mockGetRuntimeMetrics).toHaveBeenCalledTimes(2)
  })

  it('metrics API 失败时隐藏摘要且不影响组件渲染', async () => {
    mockGetRuntimeMetrics.mockRejectedValueOnce(new Error('metrics unavailable'))

    const wrapper = mount(OpsRuntimeMetricsSummary, {
      props: { active: true, refreshKey: 'request-error' },
    })
    await flushPromises()

    expect(mockGetRuntimeMetrics).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="ops-runtime-metrics-summary"]').exists()).toBe(false)
    expect(wrapper.html()).not.toContain('metrics unavailable')
  })

  it('关闭摘要时取消仍在执行的快照请求', async () => {
    let resolveRequest: ((value: typeof runtimeSnapshot) => void) | undefined
    mockGetRuntimeMetrics.mockImplementationOnce(() => new Promise((resolve) => {
      resolveRequest = resolve
    }))

    const wrapper = mount(OpsRuntimeMetricsSummary, {
      props: { active: true, refreshKey: 'request-pending' },
    })
    await flushPromises()

    const options = mockGetRuntimeMetrics.mock.calls[0]?.[0] as { signal?: AbortSignal }
    expect(options.signal?.aborted).toBe(false)
    await wrapper.setProps({ active: false })
    expect(options.signal?.aborted).toBe(true)

    resolveRequest?.(runtimeSnapshot)
    await flushPromises()
    expect(wrapper.find('[data-testid="ops-runtime-metrics-summary"]').exists()).toBe(false)
  })

  it('优先展示标准连接池状态并兼容累计统计', async () => {
    mockGetRuntimeMetrics.mockResolvedValueOnce({
      ...runtimeSnapshot,
      connection_pools: [],
      connection_pool_stats: [
        {
          name: 'upstream_http',
          capacity: 50,
          open: 0,
          in_use: 0,
          idle: 0,
          wait_count: 0,
          wait_duration_ms: 0,
          hits: 0,
          misses: 0,
          timeouts: 0,
          closed_idle: 0,
          closed_idle_time: 0,
          closed_lifetime: 0,
          stale: 0,
          cache_hit_total: 8,
          cache_miss_total: 1,
          cache_create_total: 2,
          cache_evict_total: 1,
          entries: 7,
          in_flight: 3,
          oldest_idle_age_ms: 1250,
        },
        {
          name: 'postgres',
          capacity: 40,
          open: 12,
          in_use: 5,
          idle: 7,
          wait_count: 3,
          wait_duration_ms: 25.5,
          hits: 0,
          misses: 0,
          timeouts: 1,
          closed_idle: 2,
          closed_idle_time: 3,
          closed_lifetime: 4,
          stale: 0,
        },
        {
          name: 'redis',
          capacity: 20,
          base_size: 16,
          open: 8,
          in_use: 2,
          idle: 6,
          wait_count: 0,
          wait_duration_ms: 0,
          hits: 90,
          misses: 10,
          timeouts: 0,
          closed_idle: 0,
          closed_idle_time: 0,
          closed_lifetime: 0,
          stale: 2,
        },
      ],
    })

    const wrapper = mount(OpsRuntimeMetricsSummary, {
      props: { active: true, refreshKey: 'request-pools' },
    })
    await flushPromises()

    const pools = wrapper.findAll('[data-testid="runtime-connection-pool-row"]')
    expect(pools).toHaveLength(3)
    const upstreamHTTP = pools.find(row => row.text().includes('upstream_http'))?.text() || ''
    expect(upstreamHTTP).toContain('50')
    expect(upstreamHTTP).toContain('1250 ms')
    expect(upstreamHTTP).toContain('88.9%')
    expect(upstreamHTTP).toContain('2')
    const postgres = pools.find(row => row.text().includes('postgres'))?.text() || ''
    expect(postgres).toContain('40')
    expect(postgres).toContain('12')
    expect(postgres).toContain('25.5 ms')
    expect(postgres).toContain('9')

    const redis = pools.find(row => row.text().includes('redis'))?.text() || ''
    expect(redis).toContain('90%')
    expect(redis).toContain('16')
    expect(redis).toContain('2')
  })
})
