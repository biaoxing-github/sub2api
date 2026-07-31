import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { DashboardStats } from '@/types'
import DashboardView from '../DashboardView.vue'

const { getSnapshotV2, getUserUsageTrend, getUserSpendingRanking } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  getUserUsageTrend: vi.fn(),
  getUserSpendingRanking: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2,
      getUserUsageTrend,
      getUserSpendingRanking
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const createDashboardStats = (): DashboardStats => ({
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_cache_read_ratio: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  total_account_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_cache_read_ratio: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  today_account_cost: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0
})

const createDashboardStatsWithTokenBreakdown = (): DashboardStats => ({
  ...createDashboardStats(),
  total_input_tokens: 900,
  total_output_tokens: 400,
  total_cache_read_tokens: 100,
  total_cache_read_ratio: 0.1,
  total_tokens: 1400,
  today_input_tokens: 120,
  today_output_tokens: 60,
  today_cache_read_tokens: 30,
  today_cache_read_ratio: 0.2,
  today_tokens: 210
})

const StatCardStub = {
  props: ['icon', 'label', 'value', 'sub', 'tone'],
  template: `
    <section data-test="dashboard-stat-card" :data-tone="tone">
      <span>{{ icon }}</span>
      <span>{{ label }}</span>
      <span>{{ value }}</span>
      <span v-if="sub">{{ sub }}</span>
      <slot />
      <slot name="sub" />
    </section>
  `
}

describe('admin DashboardView', () => {
  beforeEach(() => {
    getSnapshotV2.mockReset()
    getUserUsageTrend.mockReset()
    getUserSpendingRanking.mockReset()

    getSnapshotV2.mockResolvedValue({
      stats: createDashboardStats(),
      trend: [],
      models: []
    })
    getUserUsageTrend.mockResolvedValue({
      trend: [],
      start_date: '',
      end_date: '',
      granularity: 'hour'
    })
    getUserSpendingRanking.mockResolvedValue({
      ranking: [],
      total_actual_cost: 0,
      total_requests: 0,
      total_tokens: 0,
      start_date: '',
      end_date: ''
    })
  })

  it('uses last 24 hours as default dashboard range', async () => {
    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          StatCard: StatCardStub,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          AccountSpendingRanking: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: formatLocalDate(yesterday),
      end_date: formatLocalDate(now),
      granularity: 'hour'
    }))
  })

  it('renders all eight top dashboard metrics through the shared StatCard component', async () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          StatCard: StatCardStub,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          AccountSpendingRanking: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const cards = wrapper.findAll('[data-test="dashboard-stat-card"]')
    expect(cards).toHaveLength(8)
    expect(cards.map(card => card.text())).toEqual(expect.arrayContaining([
      expect.stringContaining('admin.dashboard.apiKeys'),
      expect.stringContaining('admin.dashboard.accounts'),
      expect.stringContaining('admin.dashboard.todayRequests'),
      expect.stringContaining('admin.dashboard.users'),
      expect.stringContaining('admin.dashboard.todayTokens'),
      expect.stringContaining('admin.dashboard.totalTokens'),
      expect.stringContaining('admin.dashboard.performance'),
      expect.stringContaining('admin.dashboard.avgResponse')
    ]))
  })

  it('renders token input output cache read and cache read ratio', async () => {
    getSnapshotV2.mockResolvedValueOnce({
      stats: createDashboardStatsWithTokenBreakdown(),
      trend: [],
      models: []
    })

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          StatCard: StatCardStub,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          AccountSpendingRanking: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('admin.dashboard.input: 120')
    expect(text).toContain('admin.dashboard.output: 60')
    expect(text).toContain('admin.dashboard.cacheRead: 30')
    expect(text).toContain('20.0%')
    expect(text).toContain('admin.dashboard.input: 900')
    expect(text).toContain('admin.dashboard.output: 400')
    expect(text).toContain('admin.dashboard.cacheRead: 100')
    expect(text).toContain('10.0%')
  })
})
