import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AccountTableFilters from '../AccountTableFilters.vue'

const selectCalls: Array<{ modelValue: unknown; options: Array<{ value: string; label: string }> }> = []

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => {
      const labels: Record<string, string> = {
        'admin.accounts.allStatus': '全部状态',
        'admin.accounts.status.active': '正常',
        'admin.accounts.status.unschedulable': '关闭调度',
        'admin.accounts.status.inactive': '停用',
        'admin.accounts.status.error': '错误',
        'admin.accounts.status.rateLimited': '限流中',
        'admin.accounts.status.tempUnschedulable': '临时不可调度',
      }
      return labels[key] ?? key
    },
  }),
}))

describe('AccountTableFilters', () => {
  it('状态筛选包含关闭调度并排在停用前', () => {
    selectCalls.length = 0

    mount(AccountTableFilters, {
      props: {
        searchQuery: '',
        filters: {
          platform: '',
          type: '',
          plan_type: '',
          status: '',
          privacy_mode: '',
          group: '',
        },
        groups: [],
      },
      global: {
        stubs: {
          SearchInput: true,
          Select: {
            props: ['modelValue', 'options'],
            setup(props) {
              selectCalls.push({
                modelValue: props.modelValue,
                options: props.options,
              })
              return () => null
            },
          },
        },
      },
    })

    const statusSelect = selectCalls.find((call) => call.options.some((option) => option.value === 'unschedulable'))

    expect(statusSelect).toBeTruthy()
    expect(statusSelect?.options.map((option) => option.value)).toEqual([
      '',
      'active',
      'unschedulable',
      'inactive',
      'error',
      'rate_limited',
      'temp_unschedulable',
    ])
    expect(statusSelect?.options.find((option) => option.value === 'unschedulable')?.label).toBe('关闭调度')
  })
})
