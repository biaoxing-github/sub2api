import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountFormTabs from '../AccountFormTabs.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('AccountFormTabs', () => {
  it('switches tabs by click and keyboard without submitting the form', async () => {
    const wrapper = mount(AccountFormTabs, {
      props: { modelValue: 'basic' }
    })

    const basic = wrapper.get('[data-testid="account-form-tab-basic"]')
    const advanced = wrapper.get('[data-testid="account-form-tab-advanced"]')

    expect(basic.attributes('aria-selected')).toBe('true')
    expect(advanced.attributes('type')).toBe('button')

    await advanced.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['advanced'])

    await basic.trigger('keydown', { key: 'ArrowRight' })
    expect(wrapper.emitted('update:modelValue')?.[1]).toEqual(['advanced'])
  })
})
