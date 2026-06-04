import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountModelProbesView from '../AccountModelProbesView.vue'

const { listAccounts, listAccountProbeRuns, getAccountProbeRun, createAccountModelProbeRun, createBazaarLinkModelProbeRun, batchAccountModelProbeRuns, deleteAccountProbeRuns } = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listAccountProbeRuns: vi.fn(),
  getAccountProbeRun: vi.fn(),
  createAccountModelProbeRun: vi.fn(),
  createBazaarLinkModelProbeRun: vi.fn(),
  batchAccountModelProbeRuns: vi.fn(),
  deleteAccountProbeRuns: vi.fn(),
}))

vi.mock('@/api/admin/accounts', () => ({
  default: {
    list: listAccounts,
    listAccountProbeRuns,
    getAccountProbeRun,
    createAccountModelProbeRun,
    createBazaarLinkModelProbeRun,
    batchAccountModelProbeRuns,
    deleteAccountProbeRuns,
  },
  list: listAccounts,
  listAccountProbeRuns,
  getAccountProbeRun,
  createAccountModelProbeRun,
  createBazaarLinkModelProbeRun,
  batchAccountModelProbeRuns,
  deleteAccountProbeRuns,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const BaseDialogStub = {
  props: ['show', 'title'],
  emits: ['close'],
  template: '<section v-if="show" data-test="base-dialog"><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>',
}
const PaginationStub = {
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: '<div data-test="model-probe-pagination"><button data-test="go-page-2" @click="$emit(\'update:page\', 2)">page 2</button><button data-test="set-page-size-50" @click="$emit(\'update:pageSize\', 50)">size 50</button></div>',
}

describe('AccountModelProbesView', () => {
  beforeEach(() => {
    listAccounts.mockReset()
    listAccountProbeRuns.mockReset()
    getAccountProbeRun.mockReset()
    createAccountModelProbeRun.mockReset()
    createBazaarLinkModelProbeRun.mockReset()
    batchAccountModelProbeRuns.mockReset()
    deleteAccountProbeRuns.mockReset()
    vi.stubGlobal('confirm', vi.fn(() => true))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('loads manual model probe runs and starts a new manual probe', async () => {
    listAccountProbeRuns.mockResolvedValue({
      items: [
        {
          id: 101,
          account_id: 12,
          account_name: 'rayapi',
          mode: 'model_validation',
          status: 'success',
          model: 'gpt-4.1-mini',
          request_mode: 'stream',
          score: 95,
          grade_label: '疑似掺水',
          created_at: '2026-06-03T12:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    createAccountModelProbeRun.mockResolvedValue({
      id: 102,
      account_id: 13,
      mode: 'model_validation',
      status: 'running',
      model: 'gpt-4.1-mini',
      created_at: '2026-06-03T12:10:00Z',
    })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
        },
      },
    })
    await flushPromises()

    expect((wrapper.find('[data-test="model-probe-model"]').element as HTMLInputElement).value).toBe('gpt-5.5')
    expect(listAccountProbeRuns).toHaveBeenCalledWith(1, 20, expect.objectContaining({
      mode: 'model_validation',
      sort_by: 'created_at',
      sort_order: 'desc',
    }), expect.any(Object))
    expect(wrapper.text()).toContain('rayapi')
    expect(wrapper.text()).toContain('疑似掺水')

    await wrapper.find('[data-test="model-probe-account-id"]').setValue('13')
    await wrapper.find('[data-test="model-probe-model"]').setValue('gpt-4.1-mini')
    await wrapper.find('[data-test="model-probe-request-mode"]').setValue('stream')
    await wrapper.find('[data-test="model-probe-trusted-account-id"]').setValue('129')
    await wrapper.find('[data-test="run-model-probe"]').trigger('submit')
    await flushPromises()

    expect(createAccountModelProbeRun).toHaveBeenCalledWith({
      account_id: 13,
      model: 'gpt-4.1-mini',
      request_mode: 'stream',
      trusted_comparison_account_id: 129,
    }, expect.any(Object))
  })

  it('starts a batch model probe with all OpenAI API key accounts selected by default', async () => {
    listAccountProbeRuns.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
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
          status: 'inactive',
          schedulable: false,
          api_key_items: [{ fingerprint: 'k2', masked: 'sk-...new' }],
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
      skipped_count: 1,
      skipped: [{ account_id: 130, message: 'no api key available' }],
    })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="open-batch-model-probe-dialog"]').trigger('click')
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(1, 100, expect.objectContaining({
      platform: 'openai',
      type: 'apikey',
      sort_by: 'name',
      sort_order: 'asc',
    }), expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(listAccounts.mock.calls[0]?.[2]).not.toHaveProperty('status')
    expect(wrapper.text()).toContain('new-api-key-not-yet-probed')
    expect(wrapper.findAll('input[type="checkbox"][data-test="batch-account-select"]').every(input => (input.element as HTMLInputElement).checked)).toBe(true)

    await wrapper.find('[data-test="batch-model-probe-model"]').setValue('gpt-5.5')
    await wrapper.find('[data-test="batch-model-probe-trusted-account"]').setValue('12')
    await wrapper.find('[data-test="batch-model-probe-submit"]').trigger('click')
    await flushPromises()

    expect(batchAccountModelProbeRuns).toHaveBeenCalledWith({
      account_ids: [12, 99],
      model: 'gpt-5.5',
      request_mode: 'non_stream',
      trusted_comparison_account_id: 12,
    }, expect.any(Object))
    expect(listAccountProbeRuns).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('admin.accountModelProbes.batchModelAcceptedWithSkipped')
  })

  it('starts a BazaarLink full probe without sending an API key from the browser', async () => {
    listAccountProbeRuns.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })
    createBazaarLinkModelProbeRun.mockResolvedValue({
      id: 202,
      account_id: 12,
      mode: 'model_validation',
      probe_source: 'bazaarlink_api',
      status: 'running',
      model: 'anthropic/claude-opus-4.7',
      request_mode: 'full',
      created_at: '2026-06-04T10:00:00Z',
    })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="open-bazaarlink-probe-dialog"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-test="bazaarlink-probe-account-id"]').setValue('12')
    await wrapper.find('[data-test="bazaarlink-probe-model"]').setValue('anthropic/claude-opus-4.7')
    await wrapper.find('[data-test="bazaarlink-probe-mode-full"]').trigger('click')
    await wrapper.find('[data-test="bazaarlink-probe-submit"]').trigger('click')
    await flushPromises()

    expect(createBazaarLinkModelProbeRun).toHaveBeenCalledWith({
      account_id: 12,
      model: 'anthropic/claude-opus-4.7',
      mode: 'full',
    }, expect.any(Object))
    expect(createBazaarLinkModelProbeRun.mock.calls[0]?.[0]).not.toHaveProperty('apiKey')
    expect(createBazaarLinkModelProbeRun.mock.calls[0]?.[0]).not.toHaveProperty('api_key')
    expect(wrapper.text()).toContain('admin.accountModelProbes.bazaarLinkStarted')
    expect(listAccountProbeRuns).toHaveBeenCalledTimes(2)
  })

  it('renders the probe source column for BazaarLink API runs', async () => {
    listAccountProbeRuns.mockResolvedValue({
      items: [
        {
          id: 202,
          account_id: 12,
          account_name: 'rayapi',
          mode: 'model_validation',
          probe_source: 'bazaarlink_api',
          status: 'success',
          model: 'anthropic/claude-opus-4.7',
          request_mode: 'quick',
          score: 88,
          created_at: '2026-06-04T10:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('admin.accountModelProbes.probeSources.bazaarlink_api')
    expect(wrapper.text()).toContain('admin.accountModelProbes.bazaarLinkModes.quick')
  })

  it('queries model probe runs by keyword and paginates results', async () => {
    listAccountProbeRuns
      .mockResolvedValueOnce({
        items: [{
          id: 101,
          account_id: 12,
          account_name: 'rayapi',
          mode: 'model_validation',
          status: 'success',
          model: 'gpt-5.5',
          request_mode: 'non_stream',
          created_at: '2026-06-03T12:00:00Z',
        }],
        total: 60,
        page: 1,
        page_size: 20,
      })
      .mockResolvedValue({
        items: [],
        total: 60,
        page: 2,
        page_size: 20,
      })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="model-probe-keyword"]').setValue('rayapi')
    await wrapper.find('[data-test="model-probe-search"]').trigger('submit')
    await flushPromises()

    expect(listAccountProbeRuns).toHaveBeenLastCalledWith(1, 20, expect.objectContaining({
      mode: 'model_validation',
      keyword: 'rayapi',
      sort_by: 'created_at',
      sort_order: 'desc',
    }), expect.any(Object))

    await wrapper.find('[data-test="go-page-2"]').trigger('click')
    await flushPromises()

    expect(listAccountProbeRuns).toHaveBeenLastCalledWith(2, 20, expect.objectContaining({
      mode: 'model_validation',
      keyword: 'rayapi',
    }), expect.any(Object))

    await wrapper.find('[data-test="set-page-size-50"]').trigger('click')
    await flushPromises()

    expect(listAccountProbeRuns).toHaveBeenLastCalledWith(1, 50, expect.objectContaining({
      mode: 'model_validation',
      keyword: 'rayapi',
    }), expect.any(Object))
  })

  it('deletes one model probe run from the row action', async () => {
    listAccountProbeRuns
      .mockResolvedValueOnce({
        items: [{
          id: 101,
          account_id: 12,
          account_name: 'rayapi',
          mode: 'model_validation',
          status: 'success',
          model: 'gpt-5.5',
          request_mode: 'non_stream',
          created_at: '2026-06-03T12:00:00Z',
        }],
        total: 1,
        page: 1,
        page_size: 20,
      })
      .mockResolvedValueOnce({
        items: [],
        total: 0,
        page: 1,
        page_size: 20,
      })
    deleteAccountProbeRuns.mockResolvedValue({
      requested_count: 1,
      deleted_count: 1,
      skipped_running_count: 0,
    })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="model-probe-delete-101"]').trigger('click')
    await flushPromises()

    expect(window.confirm).toHaveBeenCalledWith('admin.accountModelProbes.deleteConfirm')
    expect(deleteAccountProbeRuns).toHaveBeenCalledWith([101], expect.any(Object))
    expect(listAccountProbeRuns).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('admin.accountModelProbes.deleteSucceeded')
  })

  it('bulk deletes selected non-running model probe runs on the current page', async () => {
    listAccountProbeRuns
      .mockResolvedValueOnce({
        items: [
          {
            id: 101,
            account_id: 12,
            account_name: 'rayapi',
            mode: 'model_validation',
            status: 'success',
            model: 'gpt-5.5',
            request_mode: 'non_stream',
            created_at: '2026-06-03T12:00:00Z',
          },
          {
            id: 102,
            account_id: 13,
            account_name: 'running-account',
            mode: 'model_validation',
            status: 'running',
            model: 'gpt-5.5',
            request_mode: 'stream',
            created_at: '2026-06-03T12:05:00Z',
          },
          {
            id: 103,
            account_id: 14,
            account_name: 'partial-account',
            mode: 'model_validation',
            status: 'partial',
            model: 'gpt-5.5',
            request_mode: 'stream',
            created_at: '2026-06-03T12:10:00Z',
          },
        ],
        total: 3,
        page: 1,
        page_size: 20,
      })
      .mockResolvedValueOnce({
        items: [],
        total: 0,
        page: 1,
        page_size: 20,
      })
    deleteAccountProbeRuns.mockResolvedValue({
      requested_count: 2,
      deleted_count: 2,
      skipped_running_count: 0,
    })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="select-visible-model-probe-runs"]').setValue(true)
    await wrapper.find('[data-test="delete-selected-model-probe-runs"]').trigger('click')
    await flushPromises()

    expect(deleteAccountProbeRuns).toHaveBeenCalledWith([101, 103], expect.any(Object))
    expect(listAccountProbeRuns).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('admin.accountModelProbes.deleteSucceeded')
  })

  it('loads model probe detail and renders validation evidence', async () => {
    listAccountProbeRuns.mockResolvedValue({
      items: [
        {
          id: 101,
          account_id: 12,
          account_name: 'rayapi',
          mode: 'model_validation',
          status: 'success',
          model: 'gpt-4.1-mini',
          request_mode: 'stream',
          created_at: '2026-06-03T12:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    getAccountProbeRun.mockResolvedValue({
      id: 101,
      account_id: 12,
      mode: 'model_validation',
      status: 'success',
      model: 'gpt-4.1-mini',
      samples: [
        {
          id: 1,
          run_id: 101,
          request_index: 1,
          type: 'model_validation',
          label: '模型验证：JSON 算术',
          status: 'success',
          model: 'gpt-4.1-mini',
          upstream_endpoint: 'https://api.example.test/v1/responses',
          http_status: 200,
          latency_ms: 120,
          first_token_ms: 80,
          input_tokens: 18,
          output_tokens: 4,
          tokens: 26,
          api_key_masked: 'sk-...test',
          output_text: '{"sum":83,"code":"BETA"}',
          request_prompt: '只输出严格 JSON：{"sum":数字,"code":"BETA"}。',
          request_body: '{"model":"gpt-4.1-mini","input":[{"role":"user","content":"JSON 算术"}]}',
          response_body: '{"id":"resp_123","model":"gpt-4.1-mini","output_text":"{\\"sum\\":83,\\"code\\":\\"BETA\\"}"}',
          validation_evidence: [
            {
              key: 'json_arithmetic',
              label: 'JSON 算术',
              expected: '{"sum":83,"code":"BETA"}',
              observed: '{"sum":83,"code":"BETA"}',
              passed: true,
              score: 10,
              max_score: 10,
              category: 'model_match',
              severity: 'warn',
              attempt_count: 3,
              retry_attempt_count: 2,
              attempt_status_codes: [429, 500, 200],
              response_model: 'gpt-4o-mini',
              expected_model: 'gpt-4.1-mini',
              trusted_account_id: 129,
              similarity_percent: 96,
              pair_coverage_percent: 100,
              target_pass_rate_percent: 100,
              trusted_pass_rate_percent: 100,
              message: '目标链路与可信对比链路的隐藏分布探针相似度正常',
            },
          ],
          created_at: '2026-06-03T12:00:00Z',
        },
        {
          id: 2,
          run_id: 101,
          request_index: 2,
          type: 'model_validation',
          label: '模型验证：工具调用',
          status: 'failed',
          model: 'gpt-4.1-mini',
          upstream_endpoint: 'https://api.example.test/v1/responses',
          http_status: 200,
          latency_ms: 96,
          input_tokens: 22,
          output_tokens: 0,
          tokens: 22,
          output_text: '模型实际返回：没有工具调用',
          request_prompt: '必须调用 record_model_check 工具并返回 code=ok。',
          request_body: '{"model":"gpt-4.1-mini","tools":[{"name":"record_model_check"}]}',
          response_body: '{"id":"resp_failed","output":[{"type":"message","content":"没有工具调用"}]}',
          error_code: 'model_validation_failed',
          error: '模型验证未通过：工具调用未出现',
          validation_evidence: [
            {
              key: 'tool_call',
              label: '工具调用',
              expected: 'record_model_check(code=ok,count=1)',
              observed: 'function call not observed',
              passed: false,
              score: 0,
              max_score: 10,
              message: '模型验证未通过：工具调用未出现',
            },
          ],
          created_at: '2026-06-03T12:00:00Z',
        },
      ],
      created_at: '2026-06-03T12:00:00Z',
    })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="model-probe-detail-101"]').trigger('click')
    await flushPromises()

    expect(getAccountProbeRun).toHaveBeenCalledWith(101, expect.any(Object))
    const detailDialog = wrapper.findAll('[data-test="base-dialog"]').find(dialog =>
      dialog.text().includes('admin.accountModelProbes.detailTitle')
    )
    expect(detailDialog?.exists()).toBe(true)
    expect(detailDialog?.text()).toContain('JSON 算术')
    expect(detailDialog?.text()).toContain('https://api.example.test/v1/responses')
    expect(detailDialog?.text()).toContain('200')
    expect(detailDialog?.text()).toContain('120 ms')
    expect(detailDialog?.text()).toContain('80 ms')
    expect(detailDialog?.text()).toContain('sk-...test')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.inputTokens 18')
    expect(detailDialog?.text()).toContain('{"sum":83,"code":"BETA"}')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.requestPrompt')
    expect(detailDialog?.text()).toContain('只输出严格 JSON')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.requestBody')
    expect(detailDialog?.text()).toContain('"model":"gpt-4.1-mini"')
    expect(detailDialog?.text()).not.toContain('admin.accountModelProbes.responseBody')
    expect(detailDialog?.text()).not.toContain('"id":"resp_123"')
    expect(detailDialog?.text()).not.toContain('"id":"resp_failed"')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.failureReason')
    expect(detailDialog?.text()).toContain('model_validation_failed')
    expect(detailDialog?.text()).toContain('模型验证未通过：工具调用未出现')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.modelResult')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.modelOutput')
    expect(detailDialog?.text()).toContain('模型实际返回：没有工具调用')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.parsedObserved')
    expect(detailDialog?.text()).toContain('function call not observed')
    expect(detailDialog?.text()).toContain('record_model_check(code=ok,count=1)')
    expect(detailDialog?.text()).toContain('10 / 10')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.evidenceCategory')
    expect(detailDialog?.text()).toContain('model_match')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.evidenceSeverity')
    expect(detailDialog?.text()).toContain('warn')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.responseModel')
    expect(detailDialog?.text()).toContain('gpt-4o-mini')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.expectedModel')
    expect(detailDialog?.text()).toContain('gpt-4.1-mini')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.attemptCount')
    expect(detailDialog?.text()).toContain('3')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.retryAttemptCount')
    expect(detailDialog?.text()).toContain('2')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.attemptStatusCodes')
    expect(detailDialog?.text()).toContain('429, 500, 200')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.trustedAccount')
    expect(detailDialog?.text()).toContain('#129')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.similarity')
    expect(detailDialog?.text()).toContain('96%')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.pairCoverage')
    expect(detailDialog?.text()).toContain('100%')
    expect(detailDialog?.text()).toContain('目标链路与可信对比链路的隐藏分布探针相似度正常')

    await detailDialog?.find('button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('JSON 算术')
  })

  it('renders BazaarLink probe result as structured cards without raw secret fields', async () => {
    listAccountProbeRuns.mockResolvedValue({
      items: [
        {
          id: 202,
          account_id: 12,
          account_name: 'rayapi',
          mode: 'model_validation',
          probe_source: 'bazaarlink_api',
          status: 'success',
          model: 'anthropic/claude-opus-4.7',
          request_mode: 'full',
          created_at: '2026-06-04T10:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    getAccountProbeRun.mockResolvedValue({
      id: 202,
      account_id: 12,
      mode: 'model_validation',
      probe_source: 'bazaarlink_api',
      status: 'success',
      model: 'anthropic/claude-opus-4.7',
      request_mode: 'full',
      score: 88,
      samples: [
        {
          id: 1,
          run_id: 202,
          request_index: 1,
          type: 'bazaarlink_api',
          label: 'BazaarLink 完整验证',
          status: 'success',
          model: 'anthropic/claude-opus-4.7',
          upstream_endpoint: 'https://bazaarlink.ai/api/probe/run',
          http_status: 200,
          latency_ms: 1200,
          input_tokens: 31,
          output_tokens: 17,
          tokens: 48,
          request_body: '{"baseUrl":"https://proxy.example.test/v1","apiKey":"<redacted>","modelId":"anthropic/claude-opus-4.7","quickMode":false}',
          response_body: JSON.stringify({
            runId: 'probe_run_123',
            status: 'completed',
            score: 88,
            apiKey: '<redacted>',
            identityAssessment: {
              status: 'confirmed',
              confidence: 0.91,
              claimedModel: 'anthropic/claude-opus-4.7',
              predictedFamily: 'claude',
              subModelMatchV3F: {
                modelId: 'claude-opus-4.7',
                score: 0.89,
              },
              riskFlags: [],
            },
            items: [
              {
                probeId: 'identity-basic',
                label: 'Identity Basic',
                group: 'identity',
                passed: true,
                ttftMs: 680,
                tps: 41.2,
                response: 'I am Claude Opus 4.7.',
              },
            ],
            totalInputTokens: 31,
            totalOutputTokens: 17,
          }),
          validation_evidence: [],
          created_at: '2026-06-04T10:00:00Z',
        },
      ],
      created_at: '2026-06-04T10:00:00Z',
    })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="model-probe-detail-202"]').trigger('click')
    await flushPromises()

    const detailDialog = wrapper.findAll('[data-test="base-dialog"]').find(dialog =>
      dialog.text().includes('admin.accountModelProbes.detailTitle')
    )
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.bazaarLinkResult')
    expect(detailDialog?.text()).toContain('probe_run_123')
    expect(detailDialog?.text()).toContain('confirmed')
    expect(detailDialog?.text()).toContain('91%')
    expect(detailDialog?.text()).toContain('anthropic/claude-opus-4.7')
    expect(detailDialog?.text()).toContain('claude')
    expect(detailDialog?.text()).toContain('claude-opus-4.7 (89%)')
    expect(detailDialog?.text()).toContain('Identity Basic')
    expect(detailDialog?.text()).toContain('680 ms')
    expect(detailDialog?.text()).toContain('41.2')
    expect(detailDialog?.text()).toContain('I am Claude Opus 4.7.')
    expect(detailDialog?.text()).not.toContain('sk-live-secret')
    expect(detailDialog?.text()).not.toContain('apiKey')
  })

  it('renders BazaarLink match results with V3 candidates and risk flags from a truncated response', async () => {
    listAccountProbeRuns.mockResolvedValue({
      items: [
        {
          id: 204,
          account_id: 12,
          account_name: 'rayapi',
          mode: 'model_validation',
          probe_source: 'bazaarlink_api',
          status: 'failed',
          model: 'gpt-5.5',
          request_mode: 'full',
          created_at: '2026-06-04T11:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    getAccountProbeRun.mockResolvedValue({
      id: 204,
      account_id: 12,
      mode: 'model_validation',
      probe_source: 'bazaarlink_api',
      status: 'failed',
      model: 'gpt-5.5',
      request_mode: 'full',
      score: 0,
      samples: [
        {
          id: 4,
          run_id: 204,
          request_index: 1,
          type: 'bazaarlink_api',
          label: 'BazaarLink 完整验证',
          status: 'failed',
          model: 'gpt-5.5',
          upstream_endpoint: 'https://bazaarlink.ai/api/probe/run',
          http_status: 200,
          latency_ms: 113163,
          input_tokens: 4581,
          output_tokens: 15607,
          tokens: 20188,
          request_body: '{"apiKey":"<redacted>"}',
          response_body: '{"runId":"run_242","status":"completed","score":0,"identityAssessment":{"status":"match","confidence":0.98,"claimedModel":"gpt-5.5","predictedFamily":"openai","riskFlags":["部署探針: 回應未包含任何預期關鍵字","多模態 - PDF 識別: 回應未包含任何預期關鍵字"],"v3":{"candidates":[{"displayName":"GPT-5.3 Codex","modelId":"openai/gpt-5.3-codex","family":"openai","score":0.9893329875983731},{"displayName":"GPT-5.5","modelId":"openai/gpt-5.5","family":"openai","score":0.9852066599830172},{"displayName":"GPT-5.4 Mini","modelId":"openai/gpt-5.4-mini","family":"openai","score":0.9052747274960748}]}},"items":',
          error_code: 'bazaarlink_identity_mismatch',
          error: 'BazaarLink 未确认目标模型身份',
          validation_evidence: [
            {
              key: 'bazaarlink_identity',
              label: 'BazaarLink 模型身份',
              expected: 'gpt-5.5',
              observed: '',
              passed: false,
              score: 0,
              max_score: 100,
              message: 'BazaarLink 未确认目标模型身份',
              category: 'external_api',
              severity: 'critical',
              response_model: '',
              expected_model: 'gpt-5.5',
            },
          ],
          created_at: '2026-06-04T11:00:00Z',
        },
      ],
      created_at: '2026-06-04T11:00:00Z',
    })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="model-probe-detail-204"]').trigger('click')
    await flushPromises()

    const detailDialog = wrapper.findAll('[data-test="base-dialog"]').find(dialog =>
      dialog.text().includes('admin.accountModelProbes.detailTitle')
    )
    const candidates = detailDialog?.find('[data-test="bazaarlink-v3-candidates"]')
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.statuses.success')
    expect(detailDialog?.text()).toContain('match')
    expect(detailDialog?.text()).toContain('0 / 100')
    expect(detailDialog?.text()).toContain('部署探針: 回應未包含任何預期關鍵字')
    expect(detailDialog?.text()).toContain('多模態 - PDF 識別: 回應未包含任何預期關鍵字')
    expect(candidates?.exists()).toBe(true)
    expect(candidates?.text()).toContain('GPT-5.3 Codex')
    expect(candidates?.text()).toContain('openai/gpt-5.3-codex')
    expect(candidates?.text()).toContain('98.9%')
    expect(candidates?.text()).toContain('GPT-5.5')
    expect(candidates?.text()).toContain('98.5%')
    expect(candidates?.text()).toContain('GPT-5.4 Mini')
    expect(candidates?.text()).toContain('90.5%')
    expect(detailDialog?.text()).not.toContain('bazaarlink_identity_mismatch')
    expect(detailDialog?.text()).not.toContain('BazaarLink 未确认目标模型身份')
    expect(detailDialog?.text()).not.toContain('apiKey')
  })

  it('keeps BazaarLink returned zero score when stored response is truncated', async () => {
    listAccountProbeRuns.mockResolvedValue({
      items: [
        {
          id: 203,
          account_id: 12,
          account_name: 'rayapi',
          mode: 'model_validation',
          probe_source: 'bazaarlink_api',
          status: 'success',
          model: 'gpt-5.5',
          request_mode: 'quick',
          created_at: '2026-06-04T10:30:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    getAccountProbeRun.mockResolvedValue({
      id: 203,
      account_id: 12,
      mode: 'model_validation',
      probe_source: 'bazaarlink_api',
      status: 'success',
      model: 'gpt-5.5',
      request_mode: 'quick',
      score: 0,
      samples: [
        {
          id: 2,
          run_id: 203,
          request_index: 1,
          type: 'bazaarlink_api',
          label: 'BazaarLink 快速验证',
          status: 'success',
          model: 'gpt-5.5',
          upstream_endpoint: 'https://bazaarlink.ai/api/probe/run',
          http_status: 200,
          latency_ms: 1300,
          input_tokens: 120,
          output_tokens: 34,
          tokens: 154,
          request_body: '{"apiKey":"<redacted>"}',
          response_body: '{"runId":"run_truncated","status":"completed","identityAssessment":',
          validation_evidence: [
            {
              key: 'bazaarlink_identity',
              label: 'BazaarLink 模型身份',
              expected: 'gpt-5.5',
              observed: 'status=match; confidence=0.98; family=openai; v3f=gpt-5.5',
              passed: true,
              score: 0,
              max_score: 100,
              message: 'BazaarLink 身份验证通过',
              category: 'external_api',
              severity: 'info',
              response_model: 'gpt-5.5',
              expected_model: 'gpt-5.5',
            },
          ],
          created_at: '2026-06-04T10:30:00Z',
        },
      ],
      created_at: '2026-06-04T10:30:00Z',
    })

    const wrapper = mount(AccountModelProbesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="model-probe-detail-203"]').trigger('click')
    await flushPromises()

    const detailDialog = wrapper.findAll('[data-test="base-dialog"]').find(dialog =>
      dialog.text().includes('admin.accountModelProbes.detailTitle')
    )
    expect(detailDialog?.text()).toContain('admin.accountModelProbes.bazaarLinkResult')
    expect(detailDialog?.text()).toContain('0 / 100')
    expect(detailDialog?.text()).not.toContain('98 / 100')
    expect(detailDialog?.text()).toContain('match')
    expect(detailDialog?.text()).toContain('98%')
    expect(detailDialog?.text()).toContain('openai')
    expect(detailDialog?.text()).toContain('gpt-5.5')
    expect(detailDialog?.text()).not.toContain('apiKey')
  })

})
