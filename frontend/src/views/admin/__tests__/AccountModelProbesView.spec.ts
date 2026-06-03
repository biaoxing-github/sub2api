import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountModelProbesView from '../AccountModelProbesView.vue'

const { listAccountProbeRuns, getAccountProbeRun, createAccountModelProbeRun } = vi.hoisted(() => ({
  listAccountProbeRuns: vi.fn(),
  getAccountProbeRun: vi.fn(),
  createAccountModelProbeRun: vi.fn(),
}))

vi.mock('@/api/admin/accounts', () => ({
  default: {
    listAccountProbeRuns,
    getAccountProbeRun,
    createAccountModelProbeRun,
  },
  listAccountProbeRuns,
  getAccountProbeRun,
  createAccountModelProbeRun,
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

describe('AccountModelProbesView', () => {
  beforeEach(() => {
    listAccountProbeRuns.mockReset()
    getAccountProbeRun.mockReset()
    createAccountModelProbeRun.mockReset()
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
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="model-probe-detail-101"]').trigger('click')
    await flushPromises()

    expect(getAccountProbeRun).toHaveBeenCalledWith(101, expect.any(Object))
    expect(wrapper.text()).toContain('JSON 算术')
    expect(wrapper.text()).toContain('{"sum":83,"code":"BETA"}')
    expect(wrapper.text()).toContain('10 / 10')
  })
})
