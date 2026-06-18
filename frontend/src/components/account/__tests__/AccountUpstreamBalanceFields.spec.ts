import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountUpstreamBalanceFields from '../AccountUpstreamBalanceFields.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('AccountUpstreamBalanceFields', () => {
  it('emits upstream auth, rate, endpoint and manual balance updates', async () => {
    const wrapper = mount(AccountUpstreamBalanceFields, {
      props: {
        authUsername: '',
        authPassword: '',
        commonRateMultiplier: null,
        commonRateGroupName: '',
        balanceEndpointPathsText: '/v1/usage',
        manualBalanceTotal: null,
        hasExistingAuthPassword: true,
        showManualBalance: true
      }
    })

    const inputs = wrapper.findAll('input')
    const endpoints = wrapper.get('textarea')

    expect(inputs[1].attributes('placeholder')).toBe('admin.accounts.leaveEmptyToKeep')

    await inputs[0].setValue('balance-user')
    await inputs[1].setValue('secret')
    await inputs[2].setValue('1.25')
    await inputs[3].setValue('priority')
    await endpoints.setValue('/a\n/b')
    await inputs[4].setValue('99.5')

    expect(wrapper.emitted('update:authUsername')?.[0]).toEqual(['balance-user'])
    expect(wrapper.emitted('update:authPassword')?.[0]).toEqual(['secret'])
    expect(wrapper.emitted('update:commonRateMultiplier')?.[0]).toEqual([1.25])
    expect(wrapper.emitted('update:commonRateGroupName')?.[0]).toEqual(['priority'])
    expect(wrapper.emitted('update:balanceEndpointPathsText')?.[0]).toEqual(['/a\n/b'])
    expect(wrapper.emitted('update:manualBalanceTotal')?.[0]).toEqual([99.5])
  })
})
