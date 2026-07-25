import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import NewApiRedeemTool from './NewApiRedeemTool.vue'

const { getOverview, getRun, getRunLogs, refreshAccount, linkAPIKey } = vi.hoisted(() => ({
  getOverview: vi.fn(),
  getRun: vi.fn(),
  getRunLogs: vi.fn(),
  refreshAccount: vi.fn(),
  linkAPIKey: vi.fn()
}))

vi.mock('@/api/admin/newapiRedeem', () => ({
  newapiRedeemAPI: {
    getOverview,
    getRun,
    getRunLogs,
    importAccounts: vi.fn(),
    deleteAccount: vi.fn(),
    refreshAccount,
    createAPIKey: vi.fn(),
    updateAPIKeyGroup: vi.fn(),
    revealAPIKey: vi.fn(),
    linkAPIKey,
    uploadVoucherFiles: vi.fn(),
    deleteVoucherFile: vi.fn(),
    startRedemption: vi.fn(),
    cancelRun: vi.fn()
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, values?: Record<string, string | number>) => {
        if (key === 'admin.newapiRedeem.taskProgress') {
          return `verified ${values?.completed} / ${values?.total}`
        }
        return key
      }
    })
  }
})

describe('NewApiRedeemTool', () => {
  beforeEach(() => {
    getOverview.mockReset()
    getRun.mockReset()
    getRunLogs.mockReset()
    refreshAccount.mockReset()
    linkAPIKey.mockReset()
    refreshAccount.mockResolvedValue({})
    linkAPIKey.mockResolvedValue({ target_account_name: 'Sub2API Cun', key_count: 2 })
    getOverview.mockResolvedValue({
      base_url: 'https://www.cun.ai',
      accounts: [{
        id: 'account-1',
        user_id: '14690',
        access_key_masked: 'abcd***wxyz',
        username: 'worker-1',
        email: 'worker@example.com',
        group: 'default',
        quota: '500000',
        used_quota: '0',
        api_keys: [{
          id: 7,
          name: 'primary',
          group: 'default',
          masked_key: 'sk-abcd****wxyz',
          referenced_accounts: [{ id: 21, name: 'Sub2API Cun', referenced: true }],
          target_accounts: [{ id: 21, name: 'Sub2API Cun', referenced: true }]
        }],
        available_groups: ['default'],
        redeemed_codes: [{
          file_id: 'marked-file-1',
          file_name: 'already-used.txt',
          result: 'already_redeemed',
          redeemed_at: '2026-07-24T09:00:00Z'
        }],
        status: 'ready',
        browser_fingerprint: 'Chrome 100 / Windows'
      }, {
        id: 'account-2',
        user_id: '14719',
        access_key_masked: 'efgh***stuv',
        username: 'worker-2',
        group: 'default',
        quota: '750000',
        used_quota: '250000',
        api_keys: [],
        available_groups: ['default'],
        redeemed_codes: [],
        status: 'ready',
        browser_fingerprint: 'Firefox 120 / Linux'
      }],
      files: [{ id: 'file-1', name: 'codes.txt', code_count: 8, uploaded_at: '2026-07-24T10:00:00Z' }],
      runs: [{
        id: 'run-1',
        status: 'running',
        started_at: '2026-07-24T10:01:00Z',
        total_pairs: 12,
        completed_pairs: 4,
        attempts: 5,
        successes: 1,
        message: '并行验证中',
        current_file_id: 'file-1',
        current_file_name: 'codes.txt',
        current_node: 'node-a',
        current_exit_ip: '1.1.1.1',
        switch_count: 0,
        logs: [{
          at: '2026-07-24T10:02:00Z',
          file_id: 'file-1',
          file_name: 'codes.txt',
          account_id: 'account-1',
          user_id: '14690',
          browser_fingerprint: 'Chrome 100 / Windows',
          code: 'code-1',
          result: 'already_redeemed',
          message: '已达到上限'
        }]
      }]
    })
    getRunLogs.mockResolvedValue({
      id: 'run-1',
      logs: [{
        at: '2026-07-24T10:02:00Z',
        file_id: 'file-1',
        file_name: 'codes.txt',
        account_id: 'account-1',
        user_id: '14690',
        browser_fingerprint: 'Chrome 100 / Windows',
        code: 'code-1',
        result: 'already_redeemed',
        message: '已达到上限'
      }]
    })
  })

  afterEach(() => {
    vi.clearAllTimers()
  })

  it('keeps polling logs out of the DOM until the first 100 rows are requested', async () => {
    const wrapper = mount(NewApiRedeemTool, {
      global: {
        stubs: {
          Icon: { template: '<span />' }
        }
      }
    })
    await flushPromises()

    expect(getOverview).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('verified 4 / 12')
    expect(wrapper.text().match(/Chrome 100 \/ Windows/g)).toHaveLength(1)
    expect(wrapper.text()).toContain('并行验证中')
    expect(wrapper.text()).toContain('already-used.txt')
    expect(wrapper.text()).not.toContain('code-1')

    await wrapper.get('[data-test="view-run-logs"]').trigger('click')
    await flushPromises()

    expect(getRunLogs).toHaveBeenCalledWith('run-1', 100)
    expect(wrapper.text()).toContain('code-1')
    expect(wrapper.text().match(/Chrome 100 \/ Windows/g)).toHaveLength(2)
    wrapper.unmount()
  })

  it('shows the total balance and allows refreshing accounts during a running redemption', async () => {
    const wrapper = mount(NewApiRedeemTool, {
      global: {
        stubs: {
          Icon: { template: '<span />' }
        }
      }
    })
    await flushPromises()

    expect(wrapper.get('[data-test="total-balance"]').text()).toContain('2.50')
    const refreshAll = wrapper.get('[data-test="refresh-all-accounts"]')
    expect(refreshAll.attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-test="refresh-account"]').attributes('disabled')).toBeUndefined()

    await refreshAll.trigger('click')
    await flushPromises()

    expect(refreshAccount).toHaveBeenCalledTimes(2)
    expect(refreshAccount).toHaveBeenCalledWith('account-1')
    expect(refreshAccount).toHaveBeenCalledWith('account-2')
    wrapper.unmount()
  })

  it('shows Sub2API references and supports append and confirmed replace', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mount(NewApiRedeemTool, {
      global: {
        stubs: {
          Icon: { template: '<span />' }
        }
      }
    })
    await flushPromises()

    expect(wrapper.get('[data-test="api-key-references"]').text()).toContain('Sub2API Cun')
    await wrapper.get('[data-test="append-key"]').trigger('click')
    await flushPromises()
    expect(linkAPIKey).toHaveBeenCalledWith('account-1', 7, 21, 'append')

    await wrapper.get('[data-test="replace-key"]').trigger('click')
    await flushPromises()
    expect(confirm).toHaveBeenCalledOnce()
    expect(linkAPIKey).toHaveBeenCalledWith('account-1', 7, 21, 'replace')
    confirm.mockRestore()
    wrapper.unmount()
  })
})
