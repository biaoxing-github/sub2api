import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { UserSubscription } from '@/types'
import SubscriptionsView from '../SubscriptionsView.vue'

const { listSubscriptions, restoreSubscription, getAllGroups, searchUsers, showError, showSuccess } = vi.hoisted(() => ({
  listSubscriptions: vi.fn(),
  restoreSubscription: vi.fn(),
  getAllGroups: vi.fn(),
  searchUsers: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    subscriptions: {
      list: listSubscriptions,
      restore: restoreSubscription,
    },
    groups: {
      getAll: getAllGroups,
    },
    usage: {
      searchUsers,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const revokedSubscription: UserSubscription = {
  id: 42,
  user_id: 7,
  group_id: 9,
  status: 'revoked',
  starts_at: '2026-07-01T00:00:00Z',
  expires_at: '2026-08-01T00:00:00Z',
  revoked_at: '2026-07-10T08:30:00Z',
  daily_usage_usd: 0,
  weekly_usage_usd: 0,
  monthly_usage_usd: 0,
  daily_window_start: null,
  weekly_window_start: null,
  monthly_window_start: null,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-10T08:30:00Z',
  user: {
    id: 7,
    email: 'revoked@example.com',
    username: 'revoked-user',
    role: 'user',
    balance: 0,
    concurrency: 1,
    status: 'active',
    allowed_groups: [],
    balance_notify_enabled: false,
    balance_notify_threshold: null,
    balance_notify_extra_emails: [],
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-01T00:00:00Z',
  },
}

const DataTableStub = {
  props: ['data'],
  template: `
    <div data-test="subscription-table">
      <div v-for="row in data" :key="row.id">
        <slot name="cell-revoked_at" :row="row" :value="row.revoked_at" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `,
}

const ConfirmDialogStub = {
  props: ['show', 'title'],
  emits: ['confirm', 'cancel'],
  template: `
    <section v-if="show" data-test="confirm-dialog">
      <h2>{{ title }}</h2>
      <button data-test="confirm-dialog-confirm" @click="$emit('confirm')">confirm</button>
    </section>
  `,
}

describe('SubscriptionsView restore action', () => {
  beforeEach(() => {
    listSubscriptions.mockReset()
    restoreSubscription.mockReset()
    getAllGroups.mockReset()
    searchUsers.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    listSubscriptions.mockResolvedValue({
      items: [revokedSubscription],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getAllGroups.mockResolvedValue([])
    restoreSubscription.mockResolvedValue({ ...revokedSubscription, status: 'active', revoked_at: null })
  })

  it('restores a revoked subscription after confirmation and refreshes the list', async () => {
    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          BaseDialog: true,
          ConfirmDialog: ConfirmDialogStub,
          Select: true,
          GroupBadge: true,
          EmptyState: true,
          GroupOptionItem: true,
          Icon: true,
          Teleport: true,
          'router-link': true,
        },
      },
    })
    await flushPromises()

    const restoreButton = wrapper.findAll('button').find(button => button.text() === 'admin.subscriptions.restore')
    expect(restoreButton).toBeDefined()
    await restoreButton!.trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="confirm-dialog-confirm"]').trigger('click')
    await flushPromises()

    expect(restoreSubscription).toHaveBeenCalledWith(42)
    expect(showSuccess).toHaveBeenCalledWith('admin.subscriptions.subscriptionRestored')
    expect(listSubscriptions).toHaveBeenCalledTimes(2)
  })

  it('shows the revocation timestamp for a revoked subscription', async () => {
    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          BaseDialog: true,
          ConfirmDialog: ConfirmDialogStub,
          Select: true,
          GroupBadge: true,
          EmptyState: true,
          GroupOptionItem: true,
          Icon: true,
          Teleport: true,
          'router-link': true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="subscription-revoked-at"]').exists()).toBe(true)
  })
})
