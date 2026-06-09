import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountSchedulingPoolView from '../AccountSchedulingPoolView.vue'

const { listSchedulingPool, setSchedulable } = vi.hoisted(() => ({
  listSchedulingPool: vi.fn(),
  setSchedulable: vi.fn(),
}))

vi.mock('@/api/admin/accounts', () => ({
  default: {
    listSchedulingPool,
    setSchedulable,
  },
  listSchedulingPool,
  setSchedulable,
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
      ],
      total: 1,
      schedulable_count: 0,
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

    expect(listSchedulingPool).toHaveBeenCalledWith(expect.objectContaining({ transport: 'http_sse' }), expect.objectContaining({ signal: expect.any(AbortSignal) }))
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
})
