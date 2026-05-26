import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountProbeReportsView from '../AccountProbeReportsView.vue'

const { listAccountProbeRuns, getAccountProbeRun, batchAccountProbeRuns } = vi.hoisted(() => ({
  listAccountProbeRuns: vi.fn(),
  getAccountProbeRun: vi.fn(),
  batchAccountProbeRuns: vi.fn(),
}))

vi.mock('@/api/admin/accounts', () => ({
  default: {
    listAccountProbeRuns,
    getAccountProbeRun,
    batchAccountProbeRuns,
  },
  listAccountProbeRuns,
  getAccountProbeRun,
  batchAccountProbeRuns,
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

describe('AccountProbeReportsView', () => {
  beforeEach(() => {
    listAccountProbeRuns.mockReset()
    getAccountProbeRun.mockReset()
    batchAccountProbeRuns.mockReset()
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('loads and renders account probe report rows', async () => {
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
      samples: [{ id: 1, run_id: 91, request_index: 1, status: 'success', latency_ms: 830, created_at: '2026-05-26T10:00:01Z' }],
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
          Icon: true,
        },
      },
    })
    await flushPromises()

    const checkboxes = wrapper.findAll('input[type="checkbox"][data-test="probe-row-select"]')
    await checkboxes[0].setValue(true)
    await checkboxes[1].setValue(true)
    await wrapper.find('[data-test="batch-probe-model"]').setValue('gpt-4.1-mini')
    await wrapper.find('[data-test="batch-probe-submit"]').trigger('click')
    await flushPromises()

    expect(batchAccountProbeRuns).toHaveBeenCalledWith({
      account_ids: [12, 13],
      mode: 'standard',
      model: 'gpt-4.1-mini',
      request_mode: 'stream',
      codex_stability: false,
      long_context: false,
    }, expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(listAccountProbeRuns).toHaveBeenCalledTimes(2)
  })
})
