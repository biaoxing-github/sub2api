import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'

import OAuthAuthorizationFlow from '../OAuthAuthorizationFlow.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: {
    en: {}
  },
  missingWarn: false,
  fallbackWarn: false
})

const mountFlow = () => mount(OAuthAuthorizationFlow, {
  props: {
    addMethod: 'oauth',
    platform: 'openai',
    showCookieOption: false,
    showRefreshTokenOption: false,
    showMobileRefreshTokenOption: false,
    showCodexSessionImportOption: true,
    showAgentIdentityOption: true
  },
  global: {
    plugins: [createPinia(), i18n],
    stubs: {
      Icon: true
    }
  }
})

describe('OAuthAuthorizationFlow Agent Identity import', () => {
  it('switches to the Agent Identity-specific import content', async () => {
    const wrapper = mountFlow()

    await wrapper.get('input[value="agent_identity"]').setValue(true)

    expect(wrapper.text()).toContain('admin.accounts.oauth.openai.agentIdentityDesc')
    expect(wrapper.get('textarea').attributes('placeholder')).toBe('admin.accounts.oauth.openai.agentIdentityPlaceholder')
  })

  it('uses the existing Codex session import event for Agent Identity content', async () => {
    const wrapper = mountFlow()
    const content = '{"auth_mode":"agentIdentity"}'

    await wrapper.get('input[value="agent_identity"]').setValue(true)
    await wrapper.get('textarea').setValue(content)
    await wrapper.get('button.btn-primary').trigger('click')

    expect(wrapper.emitted('import-codex-session')).toEqual([[content]])
  })
})
