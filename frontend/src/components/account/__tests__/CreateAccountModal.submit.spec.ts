import { flushPromises, shallowMount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { createAccountMock } = vi.hoisted(() => ({
  createAccountMock: vi.fn()
}))

// 为 OAuth composable 提供最小响应式替身，避免弹窗挂载触发无关网络流程。
const createOAuthState = () => {
  const values: Record<string, unknown> = {
    authUrl: { value: '' },
    sessionId: { value: '' },
    state: { value: '' },
    loading: { value: false },
    error: { value: '' }
  }
  return new Proxy(values, {
    get(target, property: string) {
      if (!(property in target)) target[property] = vi.fn()
      return target[property]
    }
  })
}

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn(),
    showWarning: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isSimpleMode: true })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createAccountMock,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false })
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] })
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('@/composables/useQuotaNotifyState', () => ({
  useQuotaNotifyState: () => ({
    globalEnabled: false,
    state: {
      daily: { enabled: false, threshold: null, thresholdType: 'percentage' },
      weekly: { enabled: false, threshold: null, thresholdType: 'percentage' },
      total: { enabled: false, threshold: null, thresholdType: 'percentage' }
    },
    loadGlobalState: vi.fn(),
    writeToExtra: vi.fn()
  })
}))

vi.mock('@/composables/useAccountOAuth', () => ({
  useAccountOAuth: () => createOAuthState()
}))
vi.mock('@/composables/useOpenAIOAuth', () => ({ useOpenAIOAuth: () => createOAuthState() }))
vi.mock('@/composables/useGeminiOAuth', () => ({ useGeminiOAuth: () => createOAuthState() }))
vi.mock('@/composables/useAntigravityOAuth', () => ({ useAntigravityOAuth: () => createOAuthState() }))
vi.mock('@/composables/useGrokOAuth', () => ({ useGrokOAuth: () => createOAuthState() }))

vi.mock('@/composables/useModelWhitelist', async () => {
  const actual = await vi.importActual<typeof import('@/composables/useModelWhitelist')>(
    '@/composables/useModelWhitelist'
  )
  return {
    ...actual,
    fetchAntigravityDefaultMappings: vi.fn().mockResolvedValue([])
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

import AccountOptionSelector from '../AccountOptionSelector.vue'
import CreateAccountModal from '../CreateAccountModal.vue'

const BaseDialogStub = defineComponent({
  props: { show: Boolean },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const AccountBasicInfoFieldsStub = defineComponent({
  emits: ['update:name', 'update:notes'],
  template: `
    <input
      data-testid="account-name-input"
      @input="$emit('update:name', $event.target.value)"
    />
  `
})

const AccountAPIKeyCredentialsFieldsStub = defineComponent({
  props: { baseUrl: String },
  emits: [
    'update:baseUrl',
    'update:requestBaseUrlsText',
    'update:balanceBaseUrl',
    'update:apiKey',
    'update:apiKeysText',
    'update:claudeCliVersion',
    'update:openaiCodexCliUserAgent'
  ],
  template: `
    <div>
      <span data-testid="base-url">{{ baseUrl }}</span>
      <input
        data-testid="api-key-input"
        @input="$emit('update:apiKey', $event.target.value)"
      />
    </div>
  `
})

describe('CreateAccountModal Grok submission', () => {
  beforeEach(() => {
    createAccountMock.mockReset()
    createAccountMock.mockResolvedValue({ id: 1 })
  })

  it('submits a Grok API-key account with the canonical platform and xAI endpoint', async () => {
    const wrapper = shallowMount(CreateAccountModal, {
      props: { show: true, proxies: [], groups: [] },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          AccountOptionSelector,
          AccountBasicInfoFields: AccountBasicInfoFieldsStub,
          AccountAPIKeyCredentialsFields: AccountAPIKeyCredentialsFieldsStub
        }
      }
    })

    // 按真实 UI 选项驱动平台和鉴权方式，覆盖表单 watch 的重置行为。
    const clickOption = async (label: string) => {
      const option = wrapper.findAll('button').find((button) => button.text().startsWith(label))
      expect(option, `missing option ${label}`).toBeDefined()
      await option!.trigger('click')
      await nextTick()
    }

    await clickOption('Grok')
    await clickOption('API Key')
    await wrapper.get('[data-testid="account-name-input"]').setValue('grok-create-regression')
    await wrapper.get('[data-testid="api-key-input"]').setValue('xai-test-key')

    expect(wrapper.get('[data-testid="base-url"]').text()).toBe('https://api.x.ai/v1')

    await wrapper.get('#create-account-form').trigger('submit')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'grok-create-regression',
      platform: 'grok',
      type: 'apikey',
      credentials: expect.objectContaining({
        api_key: 'xai-test-key',
        base_url: 'https://api.x.ai/v1'
      })
    }))
  })
})
