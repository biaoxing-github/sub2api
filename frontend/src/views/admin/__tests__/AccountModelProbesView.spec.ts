import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountModelProbesView from '../AccountModelProbesView.vue'

const { listAccounts, listAccountProbeRuns, getAccountProbeRun, createAccountModelProbeRun, batchAccountModelProbeRuns } = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listAccountProbeRuns: vi.fn(),
  getAccountProbeRun: vi.fn(),
  createAccountModelProbeRun: vi.fn(),
  batchAccountModelProbeRuns: vi.fn(),
}))

vi.mock('@/api/admin/accounts', () => ({
  default: {
    list: listAccounts,
    listAccountProbeRuns,
    getAccountProbeRun,
    createAccountModelProbeRun,
    batchAccountModelProbeRuns,
  },
  list: listAccounts,
  listAccountProbeRuns,
  getAccountProbeRun,
  createAccountModelProbeRun,
  batchAccountModelProbeRuns,
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

describe('AccountModelProbesView', () => {
  beforeEach(() => {
    listAccounts.mockReset()
    listAccountProbeRuns.mockReset()
    getAccountProbeRun.mockReset()
    createAccountModelProbeRun.mockReset()
    batchAccountModelProbeRuns.mockReset()
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
    await wrapper.find('[data-test="run-model-probe"]').trigger('submit')
    await flushPromises()

    expect(createAccountModelProbeRun).toHaveBeenCalledWith({
      account_id: 13,
      model: 'gpt-4.1-mini',
      request_mode: 'stream',
    }, expect.any(Object))
  })

  it('starts a batch model probe with all API key accounts selected by default', async () => {
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
    batchAccountModelProbeRuns.mockResolvedValue({
      runs: [],
      accepted_count: 2,
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
    expect(wrapper.text()).toContain('new-api-key-not-yet-probed')
    expect(wrapper.findAll('input[type="checkbox"][data-test="batch-account-select"]').every(input => (input.element as HTMLInputElement).checked)).toBe(true)

    await wrapper.find('[data-test="batch-model-probe-model"]').setValue('gpt-5.5')
    await wrapper.find('[data-test="batch-model-probe-submit"]').trigger('click')
    await flushPromises()

    expect(batchAccountModelProbeRuns).toHaveBeenCalledWith({
      account_ids: [12, 99],
      model: 'gpt-5.5',
      request_mode: 'non_stream',
    }, expect.any(Object))
    expect(listAccountProbeRuns).toHaveBeenCalledTimes(2)
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
          latency_ms: 120,
          tokens: 26,
          output_text: '{"sum":83,"code":"BETA"}',
          validation_evidence: [
            {
              key: 'json_arithmetic',
              label: 'JSON 算术',
              expected: '{"sum":83,"code":"BETA"}',
              observed: '{"sum":83,"code":"BETA"}',
              passed: true,
              score: 10,
              max_score: 10,
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
    expect(detailDialog?.text()).toContain('{"sum":83,"code":"BETA"}')
    expect(detailDialog?.text()).toContain('10 / 10')

    await detailDialog?.find('button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('JSON 算术')
  })

})
