import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import GroupSelector from '../GroupSelector.vue'
import type { AdminGroup } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key === 'common.selectedCount') return `selected:${params?.count ?? 0}`
      return key
    }
  })
}))

const groups = [
  {
    id: 1,
    name: 'OpenAI shared',
    description: '',
    platform: 'openai',
    subscription_type: 'standard',
    rate_multiplier: 1,
    account_count: 2
  },
  {
    id: 2,
    name: 'Grok direct',
    description: '',
    platform: 'grok',
    subscription_type: 'standard',
    rate_multiplier: 1,
    account_count: 1
  },
  {
    id: 3,
    name: 'Anthropic only',
    description: '',
    platform: 'anthropic',
    subscription_type: 'standard',
    rate_multiplier: 1,
    account_count: 3
  }
] as AdminGroup[]

describe('GroupSelector', () => {
  it('allows Grok accounts to select OpenAI-compatible groups', () => {
    const wrapper = mount(GroupSelector, {
      props: {
        modelValue: [],
        groups,
        platform: 'grok'
      },
      global: {
        stubs: {
          Icon: true,
          GroupBadge: {
            props: ['name'],
            template: '<span data-testid="group-badge">{{ name }}</span>'
          }
        }
      }
    })

    const names = wrapper.findAll('[data-testid="group-badge"]').map((node) => node.text())
    expect(names).toEqual(['OpenAI shared', 'Grok direct'])
  })
})
