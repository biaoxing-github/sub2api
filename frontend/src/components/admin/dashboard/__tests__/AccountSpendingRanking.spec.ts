import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountSpendingRanking from '../AccountSpendingRanking.vue'

const getAccountSpendingRanking = vi.fn()

vi.mock('@/api/admin/dashboard', () => ({
  getAccountSpendingRanking: (...args: unknown[]) => getAccountSpendingRanking(...args)
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

const rankingResponse = (period: 'today' | '24h' | '7d') => ({
  ranking: [
    { account_id: 2, account_name: 'Account B', platform: 'openai', account_cost: 8.25, requests: 4, tokens: 500 },
    { account_id: 1, account_name: 'Account A', platform: 'claude', account_cost: 3.5, requests: 2, tokens: 200 }
  ],
  total_account_cost: 11.75,
  total_requests: 6,
  total_tokens: 700,
  period,
  start_time: '2026-07-31T00:00:00+08:00',
  end_time: '2026-07-31T12:00:00+08:00'
})

describe('AccountSpendingRanking', () => {
  beforeEach(() => {
    getAccountSpendingRanking.mockReset()
    getAccountSpendingRanking.mockResolvedValue(rankingResponse('today'))
  })

  it('loads today ranking in descending order with summary totals', async () => {
    const wrapper = mount(AccountSpendingRanking, {
      global: { stubs: { LoadingSpinner: true } }
    })
    await flushPromises()

    expect(getAccountSpendingRanking).toHaveBeenCalledWith({ period: 'today', limit: 20 })
    expect(wrapper.text()).toContain('Account B')
    expect(wrapper.text()).toContain('$11.75')
    expect(wrapper.findAll('tbody tr')).toHaveLength(2)
    expect(wrapper.find('tbody tr').text()).toContain('Account B')
  })

  it('reloads when switching to the rolling 24 hour period', async () => {
    const wrapper = mount(AccountSpendingRanking, {
      global: { stubs: { LoadingSpinner: true } }
    })
    await flushPromises()

    const buttons = wrapper.findAll('button[role="tab"]')
    await buttons[1].trigger('click')
    await flushPromises()

    expect(getAccountSpendingRanking).toHaveBeenLastCalledWith({ period: '24h', limit: 20 })
  })
})
