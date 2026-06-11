import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountSchedulingPoolView from '../AccountSchedulingPoolView.vue'

const { listSchedulingPool, setSchedulable, manualProbeAccount } = vi.hoisted(() => ({
  listSchedulingPool: vi.fn(),
  setSchedulable: vi.fn(),
  manualProbeAccount: vi.fn(),
}))

const { getAllGroups } = vi.hoisted(() => ({
  getAllGroups: vi.fn(),
}))

vi.mock('@/api/admin/accounts', () => ({
  default: {
    listSchedulingPool,
    setSchedulable,
    manualProbeAccount,
  },
  listSchedulingPool,
  setSchedulable,
  manualProbeAccount,
}))

vi.mock('@/api/admin/groups', () => ({
  default: {
    getAll: getAllGroups,
  },
  getAll: getAllGroups,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key.endsWith('.disabledSuccess')) return `disabled ${params?.name}`
        return key
      },
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<section><div data-test="filters"><slot name="filters" /></div><div data-test="table"><slot name="table" /></div></section>',
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

describe('AccountSchedulingPoolView', () => {
  beforeEach(() => {
    listSchedulingPool.mockReset()
    setSchedulable.mockReset()
    manualProbeAccount.mockReset()
    getAllGroups.mockReset()
    getAllGroups.mockResolvedValue([
      { id: 2, name: '自用', platform: 'openai', status: 'active' },
      { id: 3, name: '备用', platform: 'anthropic', status: 'active' },
    ])
    listSchedulingPool.mockResolvedValue({
      items: [
        {
          account: {
            id: 101,
            name: 'ready-pool',
            platform: 'openai',
            type: 'apikey',
            status: 'active',
            schedulable: true,
            priority: 20,
            concurrency: 5,
            load_factor: 3,
            error_message: null,
            rate_limited_at: null,
            rate_limit_reset_at: null,
            overload_until: null,
            temp_unschedulable_until: null,
            temp_unschedulable_reason: null,
            session_window_start: null,
            session_window_end: null,
            session_window_status: null,
            proxy_id: null,
            expires_at: null,
            auto_pause_on_expired: false,
            created_at: '2026-06-09T10:00:00Z',
            updated_at: '2026-06-09T10:00:00Z',
            derived_health: { state: 'line_degraded', label: '线路降级', last_failure_reason: 'unexpected_eof' },
          },
          pool_status: 'degraded',
          pool_reasons: ['path_health:degraded:unexpected_eof'],
          runtime_block: { reason: '429', until: '2026-06-09T10:10:00Z' },
          path_health: { state: 'degraded', last_failure_reason: 'unexpected_eof' },
          path_health_available: true,
          derived_health: { state: 'line_degraded', label: '线路降级', last_failure_reason: 'unexpected_eof' },
          effective_load_factor: 3,
        },
        {
          account: {
            id: 102,
            name: 'unstable-pool',
            platform: 'openai',
            type: 'apikey',
            status: 'active',
            schedulable: true,
            priority: 10,
            concurrency: 2,
            load_factor: 2,
            load_factor_advice: {
              suggested_load_factor: 1,
              reasons: ['成功率 50%', '线路状态 degraded'],
              availability_radar: {
                status: 'unstable',
                label: '不稳定',
                reasons: ['最近探测失败率升高'],
              },
              path_health_samples: 4,
            },
            error_message: null,
            rate_limited_at: null,
            rate_limit_reset_at: null,
            overload_until: null,
            temp_unschedulable_until: null,
            temp_unschedulable_reason: null,
            session_window_start: null,
            session_window_end: null,
            session_window_status: null,
            proxy_id: null,
            expires_at: null,
            auto_pause_on_expired: false,
            created_at: '2026-06-09T10:00:00Z',
            updated_at: '2026-06-09T10:00:00Z',
            derived_health: { state: 'light_abnormal', label: '轻微异常', reason: 'probe_failed' },
          },
          pool_status: 'schedulable',
          pool_reasons: [],
          runtime_block: null,
          path_health: { state: 'healthy' },
          path_health_available: true,
          derived_health: { state: 'light_abnormal', label: '轻微异常', reason: 'probe_failed' },
          effective_load_factor: 2,
        },
      ],
      total: 2,
      schedulable_count: 1,
      degraded_count: 1,
      blocked_count: 0,
      filtered_count: 0,
      generated_at: '2026-06-09T10:00:00Z',
    })
    setSchedulable.mockResolvedValue({})
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders scheduling pool health and disables scheduling from the row', async () => {
    const wrapper = mount(AccountSchedulingPoolView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(getAllGroups).toHaveBeenCalled()
    expect(listSchedulingPool).toHaveBeenCalledWith(expect.objectContaining({ group: '2', platform: 'openai', transport: 'http_sse' }), expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(wrapper.text()).toContain('ready-pool')
    expect(wrapper.text()).toContain('线路降级')
    expect(wrapper.text()).toContain('path_health:degraded:unexpected_eof')
    expect(wrapper.text()).toContain('429')

    await wrapper.find('[data-test="disable-scheduling"]').trigger('click')
    await flushPromises()

    expect(window.confirm).toHaveBeenCalled()
    expect(setSchedulable).toHaveBeenCalledWith(101, false)
    expect(listSchedulingPool).toHaveBeenCalledTimes(2)
  })

  it('renders availability radar abnormalities and reasons in the scheduling pool', async () => {
    const wrapper = mount(AccountSchedulingPoolView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('unstable-pool')
    expect(wrapper.text()).toContain('轻微异常')
    expect(wrapper.text()).toContain('不稳定')
    expect(wrapper.text()).toContain('最近探测失败率升高')
    expect(wrapper.text()).toContain('成功率 50%')
  })

  it('queries anthropic scheduling pool with the selected group', async () => {
    const wrapper = mount(AccountSchedulingPoolView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.find('[data-test="platform-filter"]').setValue('anthropic')
    await flushPromises()

    expect(listSchedulingPool).toHaveBeenLastCalledWith(expect.objectContaining({ group: '2', platform: 'anthropic' }), expect.objectContaining({ signal: expect.any(AbortSignal) }))
  })

  it('shows manual probe button for anthropic accounts and calls probe API', async () => {
    listSchedulingPool.mockResolvedValue({
      items: [
        {
          account: {
            id: 201,
            name: 'anthropic-account',
            platform: 'anthropic',
            type: 'apikey',
            status: 'active',
            schedulable: true,
            priority: 20,
            concurrency: 5,
            load_factor: 3,
          },
          pool_status: 'schedulable',
          pool_reasons: [],
          derived_health: { state: 'normal', label: '正常' },
          effective_load_factor: 3,
        },
      ],
      total: 1,
      schedulable_count: 1,
      degraded_count: 0,
      blocked_count: 0,
      filtered_count: 0,
      generated_at: '2026-06-11T10:00:00Z',
    })
    manualProbeAccount.mockResolvedValue({
      success: true,
      result: {
        success: true,
        message: 'Probe succeeded',
        latency_ms: 125,
      },
    })

    const wrapper = mount(AccountSchedulingPoolView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Select: SelectStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="manual-probe"]').exists()).toBe(true)

    await wrapper.find('[data-test="manual-probe"]').trigger('click')
    await flushPromises()

    expect(manualProbeAccount).toHaveBeenCalledWith(201, { model: 'claude-opus-4-8' })
    expect(listSchedulingPool).toHaveBeenCalledTimes(2)
  })
})
