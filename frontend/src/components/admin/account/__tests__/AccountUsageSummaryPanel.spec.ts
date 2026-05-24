import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountUsageSummaryPanel from '../AccountUsageSummaryPanel.vue'
import type { AccountPoolUsageSummary } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: 'zh-CN',
    t: (key: string, params?: Record<string, unknown>) => {
      const labels: Record<string, string> = {
        'admin.accounts.usageSummary.accounts': '账号',
        'admin.accounts.usageSummary.fiveHourRemaining': '5 小时剩余',
        'admin.accounts.usageSummary.sevenDayRemaining': '7 天剩余',
        'admin.accounts.usageSummary.upstreamActualBalance': '真实余额',
        'admin.accounts.usageSummary.upstreamUsableBalance': '可用余额',
        'admin.accounts.usageSummary.missingCodexSnapshots': '缺 Codex 快照',
        'admin.accounts.usageSummary.missingUpstreamBalanceSnapshots': '缺余额快照',
        'admin.accounts.usageSummary.generatedAt': `更新于 ${params?.time ?? ''}`,
        'admin.accounts.usageSummary.expand': '展开',
        'admin.accounts.usageSummary.notApplicable': '不适用',
      }
      return labels[key] ?? key
    },
  }),
}))

const zeroWindow = {
  used_cost: 0,
  estimated_limit_cost: 0,
  utilization: 0,
  used_percent_sum: 0,
  remaining_percent_sum: 0,
  accounts_in_window: 0,
  requests: 0,
  accounts_with_snapshot: 0,
  accounts_with_limit_estimate: 0,
}

function makeSummary(overrides: Partial<AccountPoolUsageSummary> = {}): AccountPoolUsageSummary {
  return {
    generated_at: '2026-05-24T08:00:00Z',
    total_accounts: 3,
    schedulable_accounts: 3,
    rate_limited_accounts: 0,
    missing_snapshot_accounts: 1,
    missing_codex_snapshot_accounts: 1,
    five_hour: { ...zeroWindow },
    seven_day: { ...zeroWindow },
    upstream_balance: {
      available: 12,
      used: 0,
      total: 12,
      account_count: 2,
      key_count: 1,
      ok_key_count: 1,
      failed_key_count: 0,
      missing_accounts: 2,
    },
    plans: [],
    ...overrides,
  }
}

describe('AccountUsageSummaryPanel', () => {
  it('拆开显示 Codex 快照缺失和上游余额快照缺失', () => {
    const wrapper = mount(AccountUsageSummaryPanel, {
      props: {
        summary: makeSummary(),
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('缺 Codex 快照')
    expect(wrapper.text()).toContain('缺余额快照')
    expect(wrapper.text()).toContain('1')
    expect(wrapper.text()).toContain('2')
  })
})
