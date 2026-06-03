import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountProbeReportsView from '../AccountProbeReportsView.vue'

const { listAccounts, listAccountProbeRuns, getAccountProbeRun, batchAccountProbeRuns, batchAccountModelProbeRuns, deleteAccountProbeRuns, listAccountProbeRanking, createScheduledTestPlan } = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listAccountProbeRuns: vi.fn(),
  getAccountProbeRun: vi.fn(),
  batchAccountProbeRuns: vi.fn(),
  batchAccountModelProbeRuns: vi.fn(),
  deleteAccountProbeRuns: vi.fn(),
  listAccountProbeRanking: vi.fn(),
  createScheduledTestPlan: vi.fn(),
}))

vi.mock('@/api/admin/accounts', () => ({
  default: {
    list: listAccounts,
    listAccountProbeRuns,
    getAccountProbeRun,
    batchAccountProbeRuns,
    batchAccountModelProbeRuns,
    deleteAccountProbeRuns,
    listAccountProbeRanking,
  },
  list: listAccounts,
  listAccountProbeRuns,
  getAccountProbeRun,
  batchAccountProbeRuns,
  batchAccountModelProbeRuns,
  deleteAccountProbeRuns,
  listAccountProbeRanking,
}))

vi.mock('@/api/admin/scheduledTests', () => ({
  create: createScheduledTestPlan,
  default: {
    create: createScheduledTestPlan,
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'pagination.pageOf') return `Page ${params?.page} of ${params?.total}`
        return key
      },
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<section><div data-test="filters"><slot name="filters" /></div><div data-test="table"><slot name="table" /></div><div data-test="pagination"><slot name="pagination" /></div></section>',
}
const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  methods: {
    onChange(event: Event) {
      const value = (event.target as HTMLSelectElement).value
      this.$emit('update:modelValue', value)
      this.$emit('change', value)
    },
  },
  template: '<select :value="modelValue ?? undefined" @change="onChange"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>',
}
const PaginationStub = {
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: '<div data-test="pager">{{ total }}</div>',
}
const BaseDialogStub = {
  props: ['show', 'title'],
  emits: ['close'],
  template: '<section v-if="show" data-test="base-dialog"><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>',
}

describe('AccountProbeReportsView', () => {
  beforeEach(() => {
    listAccounts.mockReset()
    listAccountProbeRuns.mockReset()
    getAccountProbeRun.mockReset()
    batchAccountProbeRuns.mockReset()
    batchAccountModelProbeRuns.mockReset()
    deleteAccountProbeRuns.mockReset()
    listAccountProbeRanking.mockReset()
    createScheduledTestPlan.mockReset()
    listAccountProbeRanking.mockResolvedValue([])
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  afterEach(() => {
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('loads and renders account probe report rows', async () => {
    listAccountProbeRanking.mockResolvedValueOnce([
      {
        account_id: 12,
        account_name: 'rayapi-free',
        run_count: 2,
        average_score: 94,
        latest_score: 96,
        grade: 'excellent',
        grade_label: '优秀',
        latest_run_id: 91,
        latest_status: 'success',
        latest_created_at: '2026-05-26T10:00:00Z',
        latest_model: 'gpt-4.1-mini',
        average_success_rate: 1,
        average_latency_ms: 830,
        score_history: [
          { run_id: 90, score: 92, grade: 'excellent', grade_label: '优秀', status: 'success', model: 'gpt-4.1-mini', mode: 'standard', request_mode: 'stream', success_rate: 1, avg_latency_ms: 900, p95_ms: 1100, total_tokens: 2000, created_at: '2026-05-25T10:00:00Z' },
          { run_id: 91, score: 96, grade: 'excellent', grade_label: '优秀', status: 'success', model: 'gpt-4.1-mini', mode: 'standard', request_mode: 'stream', success_rate: 1, avg_latency_ms: 830, p95_ms: 1000, total_tokens: 2048, created_at: '2026-05-26T10:00:00Z' },
        ],
      },
    ])
    listAccountProbeRuns.mockResolvedValue({
      items: [
        {
          id: 91,
          account_id: 12,
          account_name: 'rayapi-free',
          status: 'success',
          mode: 'standard',
          request_mode: 'stream',
          model: 'gpt-4.1-mini',
          score: 92,
          grade: 'A',
          success_rate: 0.95,
          avg_latency_ms: 830,
          p95_ms: 1440,
          first_token_ms: 310,
          total_tokens: 2048,
          created_at: '2026-05-26T10:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      summary: {
        total_runs: 1,
        avg_score: 92,
        success_rate: 0.95,
      },
    })

    const wrapper = mount(AccountProbeReportsView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Pagination: PaginationStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(listAccountProbeRuns).toHaveBeenCalledWith(1, 10, expect.objectContaining({
      sort_by: 'created_at',
      sort_order: 'desc',
    }), expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(wrapper.text()).toContain('rayapi-free')
    expect(wrapper.text()).toContain('gpt-4.1-mini')
    expect(wrapper.text()).toContain('92')
    expect(wrapper.text()).toContain('95.0%')
    expect(wrapper.text()).toContain('1,440ms')
    expect(wrapper.text()).toContain('admin.accountProbeReports.rankingTitle')
    expect(wrapper.text()).toContain('rayapi-free')
    expect(wrapper.text()).toContain('94')
  })

  it('filters report list by ranking item and shows score history', async () => {
    listAccountProbeRanking.mockResolvedValueOnce([
      {
        account_id: 12,
        account_name: 'rayapi-free',
        run_count: 2,
        average_score: 94,
        latest_score: 96,
        grade: 'excellent',
        grade_label: '优秀',
        latest_run_id: 91,
        latest_status: 'success',
        latest_created_at: '2026-05-26T10:00:00Z',
        latest_model: 'gpt-4.1-mini',
        average_success_rate: 1,
        average_latency_ms: 830,
        score_history: [
          { run_id: 90, score: 92, grade: 'excellent', grade_label: '优秀', status: 'success', model: 'gpt-4.1-mini', mode: 'standard', request_mode: 'stream', success_rate: 1, avg_latency_ms: 900, p95_ms: 1100, total_tokens: 2000, created_at: '2026-05-25T10:00:00Z' },
          { run_id: 91, score: 96, grade: 'excellent', grade_label: '优秀', status: 'success', model: 'gpt-4.1-mini', mode: 'standard', request_mode: 'stream', success_rate: 1, avg_latency_ms: 830, p95_ms: 1000, total_tokens: 2048, created_at: '2026-05-26T10:00:00Z' },
        ],
      },
    ])
    listAccountProbeRuns.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, summary: {} })

    const wrapper = mount(AccountProbeReportsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Pagination: PaginationStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="probe-ranking-item"]').trigger('click')
    await flushPromises()

    expect(listAccountProbeRuns).toHaveBeenLastCalledWith(1, 20, expect.objectContaining({ account_id: '12' }), expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(wrapper.text()).toContain('admin.accountProbeReports.selectedRanking')
    expect(wrapper.text()).toContain('96')
  })

  it('opens a detail drawer with score items, penalty items and samples', async () => {
    listAccountProbeRuns.mockResolvedValue({
      items: [
        {
          id: 91,
          account_id: 12,
          account_name: 'rayapi-free',
          status: 'success',
          mode: 'standard',
          model: 'gpt-4.1-mini',
          created_at: '2026-05-26T10:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      summary: {},
    })
    getAccountProbeRun.mockResolvedValue({
      id: 91,
      account_id: 12,
      account_name: 'rayapi-free',
      status: 'success',
      mode: 'standard',
      model: 'gpt-4.1-mini',
      created_at: '2026-05-26T10:00:00Z',
      score_items: [{ key: 'success_rate', label: 'Success rate', value: 40, max: 40 }],
      penalty_items: [{ key: 'timeout', label: 'Timeout', value: -5 }],
      samples: [{
        id: 1,
        run_id: 91,
        request_index: 1,
        status: 'success',
        latency_ms: 830,
        output_text: '{"sum":83,"code":"BETA"}',
        validation_evidence: [{
          key: 'json_arithmetic',
          label: 'JSON 算术',
          expected: '{"sum":83,"code":"BETA"}',
          observed: '{"sum":83,"code":"BETA"}',
          passed: true,
          score: 10,
          max_score: 10,
        }],
        created_at: '2026-05-26T10:00:01Z',
      }],
    })

    const wrapper = mount(AccountProbeReportsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Pagination: PaginationStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="open-probe-detail"]').trigger('click')
    await flushPromises()

    expect(getAccountProbeRun).toHaveBeenCalledWith(91, expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(document.body.textContent).toContain('Success rate')
    expect(document.body.textContent).toContain('Timeout')
    expect(document.body.textContent).toContain('830ms')
    expect(document.body.textContent).toContain('JSON 算术')
    expect(document.body.textContent).toContain('{"sum":83,"code":"BETA"}')
    expect(document.body.textContent).toContain('10 / 10')
  })

  it('starts a batch probe for selected accounts with selected options', async () => {
    listAccountProbeRuns.mockResolvedValue({
      items: [
        {
          id: 91,
          account_id: 12,
          account_name: 'rayapi-free',
          status: 'success',
          mode: 'standard',
          model: 'gpt-4.1-mini',
          created_at: '2026-05-26T10:00:00Z',
        },
        {
          id: 92,
          account_id: 13,
          account_name: 'rayapi-plus',
          status: 'success',
          mode: 'standard',
          model: 'gpt-4.1-mini',
          created_at: '2026-05-26T10:01:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      summary: {},
    })
    listAccounts.mockResolvedValue({
      items: [
        {
          id: 12,
          name: 'rayapi-free',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          api_key_items: [{ fingerprint: 'k1', masked: 'sk-...free' }],
        },
        {
          id: 99,
          name: 'new-api-key-not-yet-probed',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          api_key_items: [{ fingerprint: 'k2', masked: 'sk-...new' }],
        },
      ],
      total: 2,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    batchAccountProbeRuns.mockResolvedValue({
      runs: [],
      accepted_count: 2,
    })

    const wrapper = mount(AccountProbeReportsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="open-batch-probe-dialog"]').trigger('click')
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(1, 100, expect.objectContaining({
      platform: 'openai',
      type: 'apikey',
      sort_by: 'name',
      sort_order: 'asc',
    }), expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(wrapper.text()).toContain('new-api-key-not-yet-probed')

    const checkboxes = wrapper.findAll('input[type="checkbox"][data-test="batch-account-select"]')
    await checkboxes[0].setValue(true)
    await checkboxes[1].setValue(true)
    await wrapper.find('[data-test="batch-probe-model"]').setValue('gpt-4.1-mini')
    await wrapper.find('[data-test="batch-probe-submit"]').trigger('click')
    await flushPromises()

    expect(batchAccountProbeRuns).toHaveBeenCalledWith({
      account_ids: [12, 99],
      mode: 'standard',
      model: 'gpt-4.1-mini',
      request_mode: 'stream',
      long_context: false,
    }, expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(listAccountProbeRuns).toHaveBeenCalledTimes(2)
  })

  it('prepopulates report batch, model validation, and schedule forms with gpt-5.5', async () => {
    listAccountProbeRuns.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, summary: {} })
    listAccounts.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 100,
      pages: 0,
    })

    const wrapper = mount(AccountProbeReportsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="open-batch-probe-dialog"]').trigger('click')
    await flushPromises()
    expect((wrapper.find('[data-test="batch-probe-model"]').element as HTMLInputElement).value).toBe('gpt-5.5')

    await wrapper.find('[data-test="open-batch-model-probe-dialog"]').trigger('click')
    await flushPromises()
    expect((wrapper.find('[data-test="batch-probe-model"]').element as HTMLInputElement).value).toBe('gpt-5.5')

    await wrapper.find('[data-test="open-scheduled-probe-dialog"]').trigger('click')
    await flushPromises()
    expect((wrapper.find('[data-test="schedule-probe-model"]').element as HTMLInputElement).value).toBe('gpt-5.5')
  })

  it('starts batch model validation from the report page without changing scheduled regular probes', async () => {
    listAccountProbeRuns.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, summary: {} })
    listAccounts.mockResolvedValue({
      items: [
        {
          id: 12,
          name: 'rayapi-free',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
        },
        {
          id: 99,
          name: 'new-api-key-not-yet-probed',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
        },
      ],
      total: 2,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    batchAccountModelProbeRuns.mockResolvedValue({
      runs: [],
      accepted_count: 2,
    })

    const wrapper = mount(AccountProbeReportsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="open-batch-model-probe-dialog"]').trigger('click')
    await flushPromises()
    const checkboxes = wrapper.findAll('input[type="checkbox"][data-test="batch-account-select"]')
    await checkboxes[0].setValue(true)
    await checkboxes[1].setValue(true)
    await wrapper.find('[data-test="batch-probe-model"]').setValue('gpt-4.1-mini')
    await wrapper.find('[data-test="batch-model-probe-submit"]').trigger('click')
    await flushPromises()

    expect(batchAccountModelProbeRuns).toHaveBeenCalledWith({
      account_ids: [12, 99],
      model: 'gpt-4.1-mini',
      request_mode: 'stream',
    }, expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(batchAccountProbeRuns).not.toHaveBeenCalled()
  })

  it('creates a scheduled account probe plan', async () => {
    listAccountProbeRuns.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, summary: {} })
    createScheduledTestPlan.mockResolvedValue({ id: 7, account_id: 12, task_type: 'account_probe', model_id: 'gpt-4.1-mini', cron_expression: '*/15 * * * *' })

    const wrapper = mount(AccountProbeReportsView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Pagination: PaginationStub,
          BaseDialog: BaseDialogStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="open-scheduled-probe-dialog"]').trigger('click')
    await wrapper.find('[data-test="schedule-probe-account-id"]').setValue('12')
    await wrapper.find('[data-test="schedule-probe-model"]').setValue('gpt-4.1-mini')
    await wrapper.find('[data-test="schedule-probe-cron"]').setValue('*/15 * * * *')
    await wrapper.find('[data-test="schedule-probe-submit"]').trigger('click')
    await flushPromises()

    expect(createScheduledTestPlan).toHaveBeenCalledWith(expect.objectContaining({
      account_id: 12,
      task_type: 'account_probe',
      model_id: 'gpt-4.1-mini',
      cron_expression: '*/15 * * * *',
      probe_mode: 'standard',
      probe_request_mode: 'stream',
    }))
    expect(createScheduledTestPlan.mock.calls[0][0]).not.toHaveProperty('probe_codex_stability')
    expect(wrapper.text()).toContain('admin.accountProbeReports.scheduleCreated')
  })

  it('deletes selected finished report runs and reloads the list', async () => {
    listAccountProbeRuns
      .mockResolvedValueOnce({
        items: [
          {
            id: 91,
            account_id: 12,
            account_name: 'rayapi-free',
            status: 'success',
            mode: 'standard',
            model: 'gpt-4.1-mini',
            created_at: '2026-05-26T10:00:00Z',
          },
          {
            id: 92,
            account_id: 13,
            account_name: 'rayapi-running',
            status: 'running',
            mode: 'standard',
            model: 'gpt-4.1-mini',
            created_at: '2026-05-26T10:01:00Z',
          },
        ],
        total: 2,
        page: 1,
        page_size: 20,
        summary: {},
      })
      .mockResolvedValueOnce({
        items: [],
        total: 0,
        page: 1,
        page_size: 20,
        summary: {},
      })
    deleteAccountProbeRuns.mockResolvedValue({
      requested_count: 1,
      deleted_count: 1,
      skipped_running_count: 0,
    })

    const wrapper = mount(AccountProbeReportsView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Pagination: PaginationStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="probe-run-select"]').setValue(true)
    await wrapper.find('[data-test="delete-selected-probe-runs"]').trigger('click')
    await flushPromises()

    expect(window.confirm).toHaveBeenCalledWith('admin.accountProbeReports.deleteConfirm')
    expect(deleteAccountProbeRuns).toHaveBeenCalledWith([91], expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(listAccountProbeRuns).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('admin.accountProbeReports.deleteSucceeded')
  })

  it('does not expose unused pending status and refreshes while runs are active', async () => {
    vi.useFakeTimers()
    listAccountProbeRuns
      .mockResolvedValueOnce({
        items: [
          {
            id: 101,
            account_id: 12,
            account_name: 'rayapi-running',
            status: 'running',
            mode: 'standard',
            model: 'gpt-4.1-mini',
            created_at: '2026-05-26T10:00:00Z',
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
        summary: {},
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: 101,
            account_id: 12,
            account_name: 'rayapi-running',
            status: 'success',
            mode: 'standard',
            model: 'gpt-4.1-mini',
            created_at: '2026-05-26T10:00:00Z',
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
        summary: {},
      })

    try {
      const wrapper = mount(AccountProbeReportsView, {
        global: {
          stubs: {
            AppLayout: AppLayoutStub,
            TablePageLayout: TablePageLayoutStub,
            Select: SelectStub,
            Pagination: PaginationStub,
            Icon: true,
          },
        },
      })
      await flushPromises()

      expect(wrapper.text()).not.toContain('admin.accountProbeReports.statuses.pending')
      expect(listAccountProbeRuns).toHaveBeenCalledTimes(1)

      await vi.advanceTimersByTimeAsync(3000)
      await flushPromises()

      expect(listAccountProbeRuns).toHaveBeenCalledTimes(2)
      expect(wrapper.text()).toContain('admin.accountProbeReports.statuses.success')
    } finally {
      vi.useRealTimers()
    }
  })
})
