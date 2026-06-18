import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountOptionSelector from '../AccountOptionSelector.vue'

const options = [
  {
    value: 'oauth-based',
    label: 'OAuth',
    description: 'Use browser auth',
    icon: 'key' as const,
    testId: 'option-oauth'
  },
  {
    value: 'apikey',
    label: 'API Key',
    description: 'Use direct key',
    icon: 'cloud' as const,
    testId: 'option-apikey'
  }
]

describe('AccountOptionSelector', () => {
  it('emits card option updates and keeps selected item neutral', async () => {
    const wrapper = mount(AccountOptionSelector, {
      props: {
        modelValue: 'oauth-based',
        options,
        columns: 2
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.get('[data-testid="option-oauth"]').classes()).toContain('border-slate-400')

    await wrapper.get('[data-testid="option-apikey"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['apikey'])
  })

  it('emits segmented option updates and ignores disabled options', async () => {
    const wrapper = mount(AccountOptionSelector, {
      props: {
        modelValue: 'openai',
        variant: 'segmented',
        options: [
          { value: 'openai', label: 'OpenAI', icon: 'bolt', testId: 'option-openai' },
          { value: 'gemini', label: 'Gemini', icon: 'sparkles', disabled: true, testId: 'option-gemini' }
        ]
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.get('[data-testid="option-openai"]').classes()).toContain('text-slate-700')

    await wrapper.get('[data-testid="option-gemini"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})
