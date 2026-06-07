import { describe, expect, it, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpsConcurrencyCard from '../OpsConcurrencyCard.vue'

const mockGetConcurrencyStats = vi.fn()
const mockGetAccountAvailabilityStats = vi.fn()

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    getConcurrencyStats: (...args: any[]) => mockGetConcurrencyStats(...args),
    getAccountAvailabilityStats: (...args: any[]) => mockGetAccountAvailabilityStats(...args),
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, any>) => {
        if (params) {
          return `${key}${JSON.stringify(params)}`
        }
        return key
      },
    }),
  }
})

const emptyConcurrencyResponse = {
  enabled: true,
  platform: {},
  group: {},
  account: {},
}

function makeConcurrencyAccount(overrides: Record<string, any> = {}) {
  return {
    account_id: 11,
    account_name: 'openai-11',
    platform: 'openai',
    group_id: 7,
    group_name: 'group-7',
    current_in_use: 1,
    max_capacity: 4,
    load_percentage: 25,
    waiting_in_queue: 0,
    ...overrides,
  }
}

function makeAvailabilityAccount(overrides: Record<string, any> = {}) {
  return {
    account_id: 11,
    account_name: 'openai-11',
    platform: 'openai',
    group_id: 7,
    group_name: 'group-7',
    status: 'active',
    is_available: true,
    is_rate_limited: false,
    rate_limit_remaining_sec: 0,
    is_overloaded: false,
    overload_remaining_sec: 0,
    has_error: false,
    error_message: '',
    path_health_state: 'healthy',
    path_health_cooldown_until: '',
    path_health_last_failure_reason: '',
    path_health_consecutive_failures: 0,
    path_health_window_failures: 0,
    path_health_eof_count: 0,
    path_health_header_timeout_count: 0,
    path_health_ttft_ewma_ms: 0,
    path_health_header_wait_ewma_ms: 0,
    ...overrides,
  }
}

describe('OpsConcurrencyCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('账号行展示统一可用性徽章，并按错误、临时不可调度、限流、路径健康的优先级收敛', async () => {
    mockGetConcurrencyStats.mockResolvedValue({
      ...emptyConcurrencyResponse,
      account: {
        11: makeConcurrencyAccount(),
        12: makeConcurrencyAccount({
          account_id: 12,
          account_name: 'openai-12',
          load_percentage: 24,
          current_in_use: 2,
        }),
        13: makeConcurrencyAccount({
          account_id: 13,
          account_name: 'openai-13',
          load_percentage: 90,
          current_in_use: 3,
        }),
        14: makeConcurrencyAccount({
          account_id: 14,
          account_name: 'openai-14',
          load_percentage: 60,
          current_in_use: 4,
        }),
        15: makeConcurrencyAccount({
          account_id: 15,
          account_name: 'openai-15',
          load_percentage: 80,
          current_in_use: 0,
        }),
      },
      group: {
        7: {
          group_id: 7,
          group_name: 'group-7',
          platform: 'openai',
          current_in_use: 10,
          max_capacity: 20,
          load_percentage: 50,
          waiting_in_queue: 0,
        },
      },
      platform: {
        openai: {
          platform: 'openai',
          current_in_use: 10,
          max_capacity: 20,
          load_percentage: 50,
          waiting_in_queue: 0,
        },
      },
    })

    mockGetAccountAvailabilityStats.mockResolvedValue({
      enabled: true,
      platform: {},
      group: {},
      account: {
        11: makeAvailabilityAccount(),
        12: makeAvailabilityAccount({
          account_id: 12,
          account_name: 'openai-12',
          status: 'active',
          temp_unschedulable_until: '2099-03-15T00:00:00Z',
        }),
        13: makeAvailabilityAccount({
          account_id: 13,
          account_name: 'openai-13',
          is_rate_limited: true,
          rate_limit_remaining_sec: 180,
        }),
        14: makeAvailabilityAccount({
          account_id: 14,
          account_name: 'openai-14',
          path_health_state: 'open_circuit',
          path_health_cooldown_until: '2099-03-15T00:00:00Z',
          path_health_last_failure_reason: 'upstream timeout',
        }),
        15: makeAvailabilityAccount({
          account_id: 15,
          account_name: 'openai-15',
          has_error: true,
          error_message: 'backend failure',
        }),
      },
    })

    const wrapper = mount(OpsConcurrencyCard, {
      props: {
        groupIdFilter: 7,
        refreshToken: 0,
      },
    })

    await flushPromises()

    const rows = wrapper.findAll('div.rounded-lg.bg-gray-50')
    expect(rows).toHaveLength(5)

    const byName = new Map(rows.map(row => {
      const text = row.text()
      const match = text.match(/openai-\d+/)
      return [match?.[0] || '', text]
    }))

    expect(byName.get('openai-15')).toContain('admin.ops.accountAvailability.accountError')
    expect(byName.get('openai-13')).toContain('admin.accounts.status.rateLimited')
    expect(byName.get('openai-14')).toContain('admin.ops.accountAvailability.pathHealth.open_circuit')
    expect(byName.get('openai-12')).toContain('admin.accounts.status.tempUnschedulable')
    expect(byName.get('openai-11')).toContain('admin.ops.accountAvailability.available')
  })
})
