import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
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
      t: (key: string) => key
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
      <div data-test="data-table">{{ data.map((row) => \`\${row.name}:\${row.status}:\${row.error_message || ""}:\${row.total_account_cost ?? 0}:\${row.total_requests ?? 0}\`).join("|") }}</div>
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

describe('admin AccountsView bulk edit scope', () => {
  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
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
    const wrapper = mount(AccountsView, {
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
    })

    await flushPromises()
    await wrapper.get('[data-test="edit-filtered"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="bulk-edit-modal"]').attributes('data-show')).toBe('true')
    expect(wrapper.get('[data-test="bulk-edit-modal"]').attributes('data-target-mode')).toBe('filtered')
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

    const wrapper = mount(AccountsView, {
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
    })

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

    const wrapper = mount(AccountsView, {
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
    })

    await flushPromises()
    expect(wrapper.get('[data-test="data-table"]').text()).toContain('free-one@example.com:active:')

    await wrapper.get('[data-test="refresh-token"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="data-table"]').text()).toContain('free-one@example.com:error:invalid_grant')
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

    const wrapper = mount(AccountsView, {
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
    })

    await flushPromises()
    await wrapper.get('button[title="admin.accounts.moreActions"]').trigger('click')
    await wrapper.get('[data-test="batch-test-non-apikey"]').trigger('click')
    await flushPromises()

    expect(batchTestNonAPIKeyAccounts).toHaveBeenCalledWith(expect.objectContaining({
      model_id: 'gpt-5.5',
      concurrency: 5,
      limit: 500,
    }))
    expect(wrapper.text()).toContain('admin.accounts.batchTest.submitted')
  })

  it('uses gpt-5.5 and carries the team plan filter for paid batch tests', async () => {
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

    const wrapper = mount(AccountsView, {
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
    })

    await flushPromises()
    await wrapper.get('[data-test="set-team-filter"]').trigger('click')
    await flushPromises()
    await wrapper.get('button[title="admin.accounts.moreActions"]').trigger('click')
    await wrapper.get('[data-test="batch-test-non-apikey"]').trigger('click')
    await flushPromises()

    expect(batchTestNonAPIKeyAccounts).toHaveBeenCalledWith(expect.objectContaining({
      model_id: 'gpt-5.5',
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

    const wrapper = mount(AccountsView, {
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
    })

    await flushPromises()
    await wrapper.get('[data-test="row-31"] input[type="checkbox"]').setValue(true)
    await wrapper.get('[data-test="row-33"] input[type="checkbox"]').setValue(true)
    await wrapper.get('button[title="admin.accounts.moreActions"]').trigger('click')
    await wrapper.get('[data-test="batch-test-non-apikey"]').trigger('click')
    await flushPromises()

    expect(batchTestNonAPIKeyAccounts).toHaveBeenCalledWith(expect.objectContaining({
      model_id: 'gpt-5.5',
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

    const wrapper = mount(AccountsView, {
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
    })

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

    const wrapper = mount(AccountsView, {
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
    })

    await flushPromises()
    expect(wrapper.get('[data-test="data-table"]').text()).toContain('heavy@example.com:active::7.25:18')

    await wrapper.get('[data-test="sort-total_account_cost"]').trigger('click')
    await flushPromises()

    expect(listAccounts).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'total_account_cost',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('shows account created/imported time and alive days', async () => {
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

    const wrapper = mount(AccountsView, {
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
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.columns.createdAt')
    expect(wrapper.text()).toContain('admin.accounts.accountAgeDays')

    await wrapper.get('[data-test="sort-created_at"]').trigger('click')
    await flushPromises()

    expect(listAccounts).toHaveBeenLastCalledWith(
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
