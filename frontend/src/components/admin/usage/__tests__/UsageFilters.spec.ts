import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UsageFilters from '../UsageFilters.vue'

const messages: Record<string, string> = {
  'admin.usage.upstreamModelAudit': 'Upstream model audit',
  'admin.usage.allUpstreamModelAudit': 'All response model states',
  'admin.usage.upstreamModelMismatchOnly': 'Mismatched only',
  'admin.usage.upstreamModelMatchedOnly': 'Matched only',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => messages[key] ?? key }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    usage: {
      searchUsers: vi.fn(),
      searchApiKeys: vi.fn(),
      getModels: vi.fn().mockResolvedValue([]),
      getGroups: vi.fn().mockResolvedValue([]),
    },
    accounts: { list: vi.fn() },
  },
}))

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  template: '<div class="select-stub">{{ options.map((option) => `${String(option.value)}:${option.label}`).join("|") }}</div>',
}

describe('admin UsageFilters upstream model audit', () => {
  it('offers all, mismatch true, and mismatch false values in usage mode', () => {
    const wrapper = mount(UsageFilters, {
      props: {
        modelValue: { upstream_model_mismatch: null },
        exporting: false,
        startDate: '',
        endDate: '',
        mode: 'usage',
      },
      global: { stubs: { Select: SelectStub } },
    })

    expect(wrapper.text()).toContain('Upstream model audit')
    expect(wrapper.text()).toContain('null:All response model states')
    expect(wrapper.text()).toContain('true:Mismatched only')
    expect(wrapper.text()).toContain('false:Matched only')
  })
})
