import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import PlatformTypeBadge from '../PlatformTypeBadge.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('PlatformTypeBadge', () => {
  it('displays Grok accounts as Grok instead of falling through to Gemini', () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: {
        platform: 'grok',
        type: 'apikey'
      },
      global: {
        stubs: {
          PlatformIcon: true,
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('Grok')
    expect(wrapper.text()).not.toContain('Gemini')

    const badges = wrapper.findAll('.inline-flex.items-center.overflow-hidden > span')
    expect(badges).toHaveLength(2)
    expect(badges[0].classes()).toContain('bg-cyan-100')
    expect(badges[0].classes()).toContain('text-cyan-700')
    expect(badges[1].classes()).toContain('bg-cyan-100')
    expect(badges[1].classes()).toContain('text-cyan-600')
  })

  it.each(['xai', ' Grok '])('normalizes the Grok platform alias %j', (platform) => {
    const wrapper = mount(PlatformTypeBadge, {
      props: {
        platform: platform as 'grok',
        type: 'apikey'
      },
      global: {
        stubs: {
          PlatformIcon: true,
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('Grok')
    expect(wrapper.text()).not.toContain('Gemini')
    expect(wrapper.find('.bg-cyan-100').exists()).toBe(true)
  })

  it('does not mislabel an unknown platform as Gemini', () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: {
        platform: 'custom-provider' as 'grok',
        type: 'apikey'
      },
      global: {
        stubs: {
          PlatformIcon: true,
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('custom-provider')
    expect(wrapper.text()).not.toContain('Gemini')
  })

  it('keeps Gemini and existing platforms labeled correctly', async () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: {
        platform: 'gemini',
        type: 'oauth'
      },
      global: {
        stubs: {
          PlatformIcon: true,
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('Gemini')

    await wrapper.setProps({ platform: 'openai' })
    expect(wrapper.text()).toContain('OpenAI')

    await wrapper.setProps({ platform: 'anthropic' })
    expect(wrapper.text()).toContain('Anthropic')

    await wrapper.setProps({ platform: 'antigravity' })
    expect(wrapper.text()).toContain('Antigravity')
  })
})
