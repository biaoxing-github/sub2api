import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import SettingsTabNavigation, {
  type SettingsTabNavigationItem
} from '@/components/admin/settings/SettingsTabNavigation.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const tabs: SettingsTabNavigationItem[] = [
  { key: 'general', icon: 'home' },
  { key: 'gateway', icon: 'server' },
  { key: 'payment', icon: 'creditCard' }
]

describe('SettingsTabNavigation', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
  })

  it('renders accessible tabs and forwards select events', async () => {
    const wrapper = mount(SettingsTabNavigation, {
      props: {
        tabs,
        activeTab: 'general',
        ariaLabel: 'settings'
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.get('[role="tablist"]').attributes('aria-label')).toBe('settings')
    expect(wrapper.get('#settings-tab-general').attributes('aria-selected')).toBe('true')
    expect(wrapper.get('#settings-tab-gateway').attributes('aria-selected')).toBe('false')

    await wrapper.get('#settings-tab-gateway').trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual(['gateway'])
  })

  it('handles roving tab keyboard selection inside the navigation', async () => {
    const wrapper = mount(SettingsTabNavigation, {
      attachTo: document.body,
      props: {
        tabs,
        activeTab: 'general',
        ariaLabel: 'settings'
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    const requestAnimationFrame = vi.fn((callback: FrameRequestCallback) => {
      callback(0)
      return 1
    })
    vi.stubGlobal('requestAnimationFrame', requestAnimationFrame)

    await wrapper.get('#settings-tab-general').trigger('keydown', { key: 'ArrowRight' })
    expect(wrapper.emitted('select')?.[0]).toEqual(['gateway'])

    await wrapper.setProps({ activeTab: 'gateway' })
    expect(document.activeElement).toBe(wrapper.get('#settings-tab-gateway').element)

    await wrapper.get('#settings-tab-gateway').trigger('keydown', { key: 'End' })
    expect(wrapper.emitted('select')?.[1]).toEqual(['payment'])

    await wrapper.setProps({ activeTab: 'payment' })
    await wrapper.get('#settings-tab-payment').trigger('keydown', { key: 'ArrowRight' })
    expect(wrapper.emitted('select')?.[2]).toEqual(['general'])

    await wrapper.get('#settings-tab-payment').trigger('keydown', { key: 'Escape' })
    expect(wrapper.emitted('select')).toHaveLength(3)
  })
})
