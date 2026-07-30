import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getById,
  batchRefresh,
  batchTestNonAPIKeyAccounts,
  listBatchTestNonAPIKeyRuns,
  getBatchTestNonAPIKeyRun,
  getUsageSummary,
  getActionItems,
  getAllProxies,
  getAllGroups
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getById: vi.fn(),
  batchRefresh: vi.fn(),
  batchTestNonAPIKeyAccounts: vi.fn(),
  listBatchTestNonAPIKeyRuns: vi.fn(),
  getBatchTestNonAPIKeyRun: vi.fn(),
  getUsageSummary: vi.fn(),
  getActionItems: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getById,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh,
      batchTestNonAPIKeyAccounts,
      listBatchTestNonAPIKeyRuns,
      getBatchTestNonAPIKeyRun,
      getUsageSummary,
      getActionItems,
      toggleSchedulable: vi.fn()
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => {
        if (key === 'admin.accounts.usageSummary') {
          return 'USAGE_SUMMARY_ROOT_KEY'
        }
        if (key === 'admin.accounts.usageSummary.title') {
          return 'Usage Summary Title'
        }
        return key
      },
      locale: { value: 'zh-CN' }
    })
  }
})

const DataTableStub = {
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div>
      <button
        v-for="column in columns"
        :key="column.key"
        :data-test="'sort-' + column.key"
        @click="$emit('sort', column.key, 'desc')"
      >
        {{ column.label }}
      </button>
      <div v-for="row in data" :key="row.id" :data-test="'row-' + row.id">
        <slot name="cell-select" :row="row" />
        <template v-for="column in columns" :key="column.key">
          <slot :name="'cell-' + column.key" :row="row" :value="row[column.key]" />
        </template>
      </div>
      <div data-test="data-table">{{ data.map((row) => \`\${row.name}:\${row.status}:\${row.error_message || ""}:\${row.total_account_cost ?? 0}:\${row.total_requests ?? 0}:\${(row.api_key_items || []).map((item) => item.last_error || "").join(",")}\`).join("|") }}</div>
    </div>
  `
}

const AccountBulkActionsBarStub = {
  props: ['selectedIds'],
  emits: ['edit-filtered', 'refresh-token'],
  template: `
    <div>
      <button data-test="edit-filtered" @click="$emit('edit-filtered')">edit filtered</button>
      <button data-test="refresh-token" @click="$emit('refresh-token')">refresh token</button>
    </div>
  `
}

const BulkEditAccountModalStub = {
  props: ['show', 'target'],
  template: '<div data-test="bulk-edit-modal" :data-show="String(show)" :data-target-mode="target?.mode ?? \'\'"></div>'
}

const AccountActionMenuTestStub = {
  props: ['account'],
  emits: ['test'],
  template: '<button data-test="menu-test-account" @click="$emit(\'test\', account)">test account</button>'
}

const AccountTestModalEmitStub = {
  props: ['show', 'account'],
  emits: ['tested'],
  template: '<button v-if="show" data-test="account-test-finished" @click="$emit(\'tested\')">{{ account?.name }}</button>'
}

const DEFAULT_HIDDEN_ACCOUNT_COLUMNS = [
  'platform_type',
  'capacity',
  'status',
  'schedulable',
  'today_stats',
  'groups',
  'usage',
  'total_account_cost',
  'total_requests',
  'upstream_balance',
  'proxy',
  'priority',
  'rate_multiplier',
  'created_at',
  'last_used_at',
  'expires_at',
  'notes'
]

const showAccountColumns = (...visibleKeys: string[]) => {
  const visible = new Set(visibleKeys)
  localStorage.setItem(
    'account-hidden-columns-v2',
    JSON.stringify(DEFAULT_HIDDEN_ACCOUNT_COLUMNS.filter((key) => !visible.has(key)))
  )
}

let mountedWrappers: VueWrapper[] = []

const trackWrapper = <T extends VueWrapper>(wrapper: T): T => {
  mountedWrappers.push(wrapper)
  return wrapper
}

const mountAccountsView = () =>
  trackWrapper(mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: { template: '<div></div>' },
        AccountBulkActionsBar: AccountBulkActionsBarStub,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: BulkEditAccountModalStub,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        Icon: true
      }
    }
  }))

describe('admin AccountsView bulk edit scope', () => {
  afterEach(() => {
    for (const wrapper of mountedWrappers) {
      wrapper.unmount()
    }
    mountedWrappers = []
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getById.mockReset()
    batchRefresh.mockReset()
    batchTestNonAPIKeyAccounts.mockReset()
    listBatchTestNonAPIKeyRuns.mockReset()
    getBatchTestNonAPIKeyRun.mockReset()
    getUsageSummary.mockReset()
    getActionItems.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()

    listAccounts.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    })
    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getById.mockResolvedValue(null)
    batchRefresh.mockResolvedValue({ total: 0, success: 0, failed: 0, errors: [] })
    batchTestNonAPIKeyAccounts.mockResolvedValue({ id: 1, status: 'running', model_id: 'gpt-5.4', concurrency: 2, limit: 500, total: 0, success_count: 0, failed_count: 0, unauthorized_count: 0, created_at: '2026-05-26T10:00:00Z' })
    listBatchTestNonAPIKeyRuns.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    getBatchTestNonAPIKeyRun.mockResolvedValue({ id: 1, status: 'running', model_id: 'gpt-5.4', concurrency: 2, limit: 500, total: 0, success_count: 0, failed_count: 0, unauthorized_count: 0, created_at: '2026-05-26T10:00:00Z', items: [] })
    getUsageSummary.mockResolvedValue(null)
    getActionItems.mockResolvedValue({ items: [] })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('opens bulk edit in filtered-results mode from the bulk actions dropdown', async () => {
    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()
    await wrapper.get('[data-test="edit-filtered"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="bulk-edit-modal"]').attributes('data-show')).toBe('true')
    expect(wrapper.get('[data-test="bulk-edit-modal"]').attributes('data-target-mode')).toBe('filtered')
  })

  it('uses the usage summary title leaf key in the collapsed info banner', async () => {
    getUsageSummary.mockResolvedValue({
      generated_at: '2026-06-16T10:00:00Z',
      total_accounts: 1,
      schedulable_accounts: 1,
      rate_limited_accounts: 0,
      missing_snapshot_accounts: 0,
      five_hour: {},
      seven_day: {},
      upstream_balance: {},
      openai_upstream_balance: {},
      anthropic_upstream_balance: {},
      plans: []
    })

    const wrapper = mountAccountsView()

    await flushPromises()

    expect(wrapper.text()).toContain('Usage Summary Title')
    expect(wrapper.text()).not.toContain('USAGE_SUMMARY_ROOT_KEY')
  })

  it('keeps first-screen account summaries collapsed until the summary row is opened', async () => {
    getUsageSummary.mockResolvedValue({
      generated_at: '2026-06-16T10:00:00Z',
      total_accounts: 1,
      schedulable_accounts: 1,
      rate_limited_accounts: 0,
      missing_snapshot_accounts: 0,
      five_hour: {},
      seven_day: {},
      upstream_balance: {},
      openai_upstream_balance: {},
      anthropic_upstream_balance: {},
      plans: []
    })
    getActionItems.mockResolvedValue({
      items: [
        {
          id: 1,
          severity: 'critical',
          title: 'Needs attention',
          description: 'Action item detail'
        }
      ]
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    expect(wrapper.text()).toContain('Usage Summary Title')
    expect(wrapper.text()).toContain('1 admin.accounts.actionItems.title')
    const bannerDetails = wrapper.get('[data-test="account-info-banner-details"]')
    expect(bannerDetails.isVisible()).toBe(false)
    expect((bannerDetails.element as HTMLElement).style.display).toBe('none')

    await wrapper.get('[data-test="account-info-banner-toggle"]').trigger('click')
    await flushPromises()

    const expandedBannerDetails = wrapper.get('[data-test="account-info-banner-details"]')
    expect((expandedBannerDetails.element as HTMLElement).style.display).toBe('')
  })

  it('links API Key account names to the upstream origin only', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 101,
          name: 'upstream-key',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          credentials: { base_url: 'https://upstream.example.com/v1/chat/completions?token=hidden' },
          extra: {}
        },
        {
          id: 102,
          name: 'oauth-account',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          credentials: { base_url: 'https://oauth.example.com/private/path' },
          extra: {}
        }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    const apiKeyRow = wrapper.get('[data-test="row-101"]')
    const homepageLink = apiKeyRow.get('a')
    expect(homepageLink.attributes('href')).toBe('https://upstream.example.com')
    expect(homepageLink.attributes('rel')).toBe('noopener noreferrer')
    expect(wrapper.get('[data-test="row-102"]').find('a').exists()).toBe(false)
  })

  it('shows account-level token refresh errors in a dialog', async () => {
    window.confirm = vi.fn(() => true)
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: 'free-one@example.com',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          credentials: { plan_type: 'free' },
          extra: {},
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    batchRefresh.mockResolvedValue({
      total: 1,
      success: 0,
      failed: 1,
      errors: [{ account_id: 1, error: 'invalid_grant' }]
    })

    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          BaseDialog: { template: '<section data-test="base-dialog"><slot /><slot name="footer" /></section>' },
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()
    await wrapper.get('[data-test="refresh-token"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('free-one@example.com')
    expect(wrapper.text()).toContain('invalid_grant')
  })

  it('immediately patches refreshed account error status returned by batch refresh', async () => {
    window.confirm = vi.fn(() => true)
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: 'free-one@example.com',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          credentials: { plan_type: 'free' },
          error_message: null,
          extra: {},
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listAccounts.mockImplementation(() => new Promise(() => {}))
    batchRefresh.mockResolvedValue({
      total: 1,
      success: 0,
      failed: 1,
      errors: [{ account_id: 1, error: 'invalid_grant' }],
      accounts: [
        {
          id: 1,
          name: 'free-one@example.com',
          platform: 'openai',
          type: 'oauth',
          status: 'error',
          schedulable: true,
          credentials: { plan_type: 'free' },
          error_message: 'invalid_grant',
          extra: {},
        },
      ]
    })

    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          BaseDialog: { template: '<section data-test="base-dialog"><slot /><slot name="footer" /></section>' },
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()
    expect(wrapper.get('[data-test="data-table"]').text()).toContain('free-one@example.com:active:')

    await wrapper.get('[data-test="refresh-token"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="data-table"]').text()).toContain('free-one@example.com:error:invalid_grant')
  })

  it('refreshes the tested account so API key cooldown details appear on the row', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: 'openai-key@example.com',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          credentials: {},
          api_key_items: [
            { fingerprint: 'fp-bad', masked: 'sk-...bad', status: 'active' }
          ],
          extra: {},
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getById.mockResolvedValue({
      id: 1,
      name: 'openai-key@example.com',
      platform: 'openai',
      type: 'apikey',
      status: 'active',
      schedulable: true,
      credentials: {},
      api_key_items: [
        {
          fingerprint: 'fp-bad',
          masked: 'sk-...bad',
          status: 'cooling',
          disabled: true,
          reason: 'invalid_api_key',
          last_error: 'API returned 401: invalid key',
          disabled_until: '2026-06-23T10:30:00Z',
        }
      ],
      extra: {},
    })

    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: AccountActionMenuTestStub,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: AccountTestModalEmitStub,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()
    await wrapper.get('button[title="common.more"]').trigger('click')
    await wrapper.get('[data-test="menu-test-account"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="account-test-finished"]').trigger('click')
    await flushPromises()

    expect(getById).toHaveBeenCalledWith(1)
    expect(wrapper.get('[data-test="data-table"]').text()).toContain('API returned 401: invalid key')
  })

  it('runs batch connectivity tests for non-api-key accounts from tools menu', async () => {
    batchTestNonAPIKeyAccounts.mockResolvedValue({
      id: 88,
      status: 'running',
      model_id: 'gpt-5.4',
      concurrency: 2,
      limit: 500,
      total: 1,
      success_count: 0,
      failed_count: 0,
      unauthorized_count: 0,
      created_at: '2026-05-26T10:00:00Z',
    })
    getBatchTestNonAPIKeyRun.mockImplementation(async (_runId: number, filters?: { category?: string }) => ({
      id: 88,
      status: 'partial',
      model_id: 'gpt-5.4',
      concurrency: 2,
      limit: 500,
      total: 1,
      success_count: 0,
      failed_count: 1,
      unauthorized_count: 1,
      created_at: '2026-05-26T10:00:00Z',
      rate_limited_count: 1,
      items: filters?.category === 'unauthorized'
        ? [{
            account_id: 7,
            account_name: 'dropped-oauth@example.com',
            platform: 'openai',
            type: 'oauth',
            status: 'failed',
            category: 'unauthorized',
            error_message: 'Authentication failed (401)',
            latency_ms: 42,
          }]
        : [
            {
              account_id: 6,
              account_name: 'limited-oauth@example.com',
              platform: 'openai',
              type: 'oauth',
              status: 'failed',
              category: 'rate_limited',
              error_message: 'API returned 429',
              latency_ms: 50,
            },
            {
              account_id: 7,
              account_name: 'dropped-oauth@example.com',
              platform: 'openai',
              type: 'oauth',
              status: 'failed',
              category: 'unauthorized',
              error_message: 'Authentication failed (401)',
              latency_ms: 42,
            },
          ],
    }))

    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          BaseDialog: { template: '<section data-test="base-dialog"><slot /><slot name="footer" /></section>' },
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()
    await wrapper.get('button[title="admin.accounts.moreActions"]').trigger('click')
    await wrapper.get('[data-test="batch-test-non-apikey"]').trigger('click')
    await flushPromises()

    expect(batchTestNonAPIKeyAccounts).toHaveBeenCalledWith(expect.objectContaining({
      model_id: 'gpt-5.6-terra',
      concurrency: 5,
      limit: 500,
    }))
    expect(wrapper.text()).toContain('admin.accounts.batchTest.submitted')
  })

  it('uses gpt-5.6-terra and carries the team plan filter for paid batch tests', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 41,
          name: 'team-one@example.com',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          credentials: { plan_type: 'team' },
          extra: {},
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          BaseDialog: { template: '<section data-test="base-dialog"><slot /><slot name="footer" /></section>' },
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: {
            props: ['filters'],
            emits: ['update:filters', 'change'],
            template: '<button data-test="set-team-filter" @click="$emit(\'update:filters\', { ...filters, platform: \'openai\', plan_type: \'team\' }); $emit(\'change\')">team</button>'
          },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()
    await wrapper.get('[data-test="set-team-filter"]').trigger('click')
    await flushPromises()
    await wrapper.get('button[title="admin.accounts.moreActions"]').trigger('click')
    await wrapper.get('[data-test="batch-test-non-apikey"]').trigger('click')
    await flushPromises()

    expect(batchTestNonAPIKeyAccounts).toHaveBeenCalledWith(expect.objectContaining({
      model_id: 'gpt-5.6-terra',
      platform: 'openai',
      plan_type: 'team',
      concurrency: 5,
      limit: 500,
    }))
  })

  it('runs batch connectivity tests only for selected non-api-key accounts', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 31,
          name: 'selected-one@example.com',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          credentials: {},
          extra: {},
        },
        {
          id: 33,
          name: 'selected-two@example.com',
          platform: 'anthropic',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          credentials: {},
          extra: {},
        },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          BaseDialog: { template: '<section data-test="base-dialog"><slot /><slot name="footer" /></section>' },
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()
    await wrapper.get('[data-test="row-31"] input[type="checkbox"]').setValue(true)
    await wrapper.get('[data-test="row-33"] input[type="checkbox"]').setValue(true)
    await wrapper.get('button[title="admin.accounts.moreActions"]').trigger('click')
    await wrapper.get('[data-test="batch-test-non-apikey"]').trigger('click')
    await flushPromises()

    expect(batchTestNonAPIKeyAccounts).toHaveBeenCalledWith(expect.objectContaining({
      model_id: 'gpt-5.6-terra',
      account_ids: [31, 33],
      concurrency: 5,
      limit: 500,
    }))
    expect(batchTestNonAPIKeyAccounts.mock.calls[0][0]).not.toHaveProperty('platform')
  })

  it('opens non-api-key batch test records from tools menu', async () => {
    listBatchTestNonAPIKeyRuns.mockResolvedValue({
      items: [{
        id: 88,
        status: 'partial',
        model_id: 'gpt-5.4',
        concurrency: 2,
        limit: 500,
        total: 1,
        success_count: 0,
        failed_count: 1,
        unauthorized_count: 1,
        created_at: '2026-05-26T10:00:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 20,
    })
    getBatchTestNonAPIKeyRun.mockImplementation(async (_runId: number, filters?: { category?: string }) => ({
      id: 88,
      status: 'partial',
      model_id: 'gpt-5.4',
      concurrency: 2,
      limit: 500,
      total: 1,
      success_count: 0,
      failed_count: 1,
      unauthorized_count: 1,
      created_at: '2026-05-26T10:00:00Z',
      rate_limited_count: 1,
      items: filters?.category === 'unauthorized'
        ? [{
            account_id: 7,
            account_name: 'dropped-oauth@example.com',
            platform: 'openai',
            type: 'oauth',
            status: 'failed',
            category: 'unauthorized',
            error_message: 'Authentication failed (401)',
            latency_ms: 42,
          }]
        : [
            {
              account_id: 6,
              account_name: 'limited-oauth@example.com',
              platform: 'openai',
              type: 'oauth',
              status: 'failed',
              category: 'rate_limited',
              error_message: 'API returned 429',
              latency_ms: 50,
            },
            {
              account_id: 7,
              account_name: 'dropped-oauth@example.com',
              platform: 'openai',
              type: 'oauth',
              status: 'failed',
              category: 'unauthorized',
              error_message: 'Authentication failed (401)',
              latency_ms: 42,
            },
          ],
    }))

    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          BaseDialog: { template: '<section data-test="base-dialog"><slot /><slot name="footer" /></section>' },
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()
    await wrapper.get('button[title="admin.accounts.moreActions"]').trigger('click')
    await wrapper.get('[data-test="batch-test-records"]').trigger('click')
    await flushPromises()

    expect(listBatchTestNonAPIKeyRuns).toHaveBeenCalledWith(1, 20)
    expect(getBatchTestNonAPIKeyRun).toHaveBeenCalledWith(88)
    expect(wrapper.text()).toContain('limited-oauth@example.com')
    expect(wrapper.text()).toContain('dropped-oauth@example.com')
    expect(wrapper.text()).toContain('Authentication failed (401)')

    await wrapper.get('[data-test="batch-test-record-filter-unauthorized"]').trigger('click')
    await flushPromises()

    expect(getBatchTestNonAPIKeyRun).toHaveBeenLastCalledWith(88, { category: 'unauthorized' })
    expect(wrapper.text()).toContain('dropped-oauth@example.com')
    expect(wrapper.text()).not.toContain('limited-oauth@example.com')
  })

  it('passes total account cost sorting to the server and displays usage totals', async () => {
    showAccountColumns('total_account_cost')
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 2,
          name: 'heavy@example.com',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          credentials: { plan_type: 'plus' },
          total_account_cost: 7.25,
          total_requests: 18,
          extra: {},
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listAccounts.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    })

    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()
    expect(wrapper.get('[data-test="data-table"]').text()).toContain('heavy@example.com:active::7.25:18')

    await wrapper.get('[data-test="sort-total_account_cost"]').trigger('click')
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'total_account_cost',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('requests highest account cost first when sorting by today stats', async () => {
    showAccountColumns('today_stats')
    getBatchTodayStats.mockResolvedValueOnce({
      stats: {
        '1': { requests: 5, tokens: 1000, cost: 3.5, standard_cost: 4, user_cost: 5 }
      }
    })
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: 'today-heavy@example.com',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          credentials: {},
          extra: {}
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listAccounts.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    })

    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()
    await wrapper.get('[data-test="today-usage-ranking"]').trigger('click')
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'today_stats',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('shows account created/imported time and alive days', async () => {
    showAccountColumns('created_at')
    vi.setSystemTime(new Date('2026-05-27T10:00:00Z'))
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 9,
          name: 'aged-oauth@example.com',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          credentials: {},
          created_at: '2026-05-24T09:00:00Z',
          extra: {},
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = trackWrapper(mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    }))

    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.columns.createdAt')
    expect(wrapper.text()).toContain('admin.accounts.accountAgeDays')

    await wrapper.get('[data-test="sort-created_at"]').trigger('click')
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'created_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
    vi.useRealTimers()
  })
})
