import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountAnthropicQuotaControlSection from '../AccountAnthropicQuotaControlSection.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const baseProps = {
  windowCostEnabled: false,
  windowCostLimit: null,
  windowCostStickyReserve: null,
  sessionLimitEnabled: false,
  maxSessions: null,
  sessionIdleTimeout: null,
  rpmLimitEnabled: false,
  baseRpm: null,
  rpmStrategy: 'tiered' as const,
  rpmStickyBuffer: null,
  userMsgQueueMode: '',
  tlsFingerprintEnabled: false,
  tlsFingerprintProfileId: null,
  sessionIdMaskingEnabled: false,
  cacheTtlOverrideEnabled: false,
  cacheTtlOverrideTarget: '5m',
  customBaseUrlEnabled: false,
  customBaseUrl: '',
  tlsFingerprintProfiles: [{ id: 7, name: 'Chrome Stable' }]
}

describe('AccountAnthropicQuotaControlSection', () => {
  it('emits v-model updates for quota controls', async () => {
    const wrapper = mount(AccountAnthropicQuotaControlSection, {
      props: baseProps
    })

    await wrapper.findAll('button')[0].trigger('click')
    expect(wrapper.emitted('update:windowCostEnabled')?.[0]).toEqual([true])

    await wrapper.setProps({ windowCostEnabled: true })
    const windowCostInput = wrapper.find('input[type="number"]')
    await windowCostInput.setValue('25')
    expect(wrapper.emitted('update:windowCostLimit')?.[0]).toEqual([25])
  })

  it('shows tls profile choices only after enabling TLS fingerprint', async () => {
    const wrapper = mount(AccountAnthropicQuotaControlSection, {
      props: baseProps
    })

    expect(wrapper.find('select').exists()).toBe(false)

    const tlsToggle = wrapper.findAll('button')[6]
    await tlsToggle.trigger('click')
    expect(wrapper.emitted('update:tlsFingerprintEnabled')?.[0]).toEqual([true])

    await wrapper.setProps({ tlsFingerprintEnabled: true })
    const select = wrapper.get('select')
    expect(select.text()).toContain('Chrome Stable')
    await select.setValue('7')
    expect(wrapper.emitted('update:tlsFingerprintProfileId')?.[0]).toEqual([7])
  })

  it('keeps UMQ mode available even when RPM limit is disabled', async () => {
    const wrapper = mount(AccountAnthropicQuotaControlSection, {
      props: baseProps
    })

    const throttleButton = wrapper.findAll('button').find(button => button.text().includes('umqModeThrottle'))
    expect(throttleButton).toBeTruthy()
    await throttleButton?.trigger('click')
    expect(wrapper.emitted('update:userMsgQueueMode')?.[0]).toEqual(['throttle'])
  })
})
