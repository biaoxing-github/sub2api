import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountPoolModeSection from '../AccountPoolModeSection.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params
        ? `${key}:${params.default}:${params.max}`
        : key
    })
  }
})

describe('AccountPoolModeSection', () => {
  it('hides retry count when pool mode is disabled', () => {
    const wrapper = mount(AccountPoolModeSection, {
      props: {
        enabled: false,
        retryCount: 3,
        defaultRetryCount: 3,
        maxRetryCount: 10
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.poolMode')
    expect(wrapper.find('input[type="number"]').exists()).toBe(false)
  })

  it('emits enabled and retry count updates', async () => {
    const wrapper = mount(AccountPoolModeSection, {
      props: {
        enabled: true,
        retryCount: 3,
        defaultRetryCount: 3,
        maxRetryCount: 10
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('update:enabled')?.[0]).toEqual([false])

    const input = wrapper.get('input[type="number"]')
    await input.setValue('7')
    expect(wrapper.emitted('update:retryCount')?.[0]).toEqual([7])
  })
})
