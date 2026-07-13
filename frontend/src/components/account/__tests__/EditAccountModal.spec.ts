import { describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

const { updateAccountMock, deleteAccountAPIKeyMock, restoreAccountAPIKeyStateMock, checkMixedChannelRiskMock } = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  deleteAccountAPIKeyMock: vi.fn(),
  restoreAccountAPIKeyStateMock: vi.fn(),
  checkMixedChannelRiskMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isSimpleMode: true
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      update: updateAccountMock,
      deleteAccountAPIKey: deleteAccountAPIKeyMock,
      restoreAccountAPIKeyState: restoreAccountAPIKeyStateMock,
      checkMixedChannelRisk: checkMixedChannelRiskMock
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({})
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

import EditAccountModal from '../EditAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const ModelWhitelistSelectorStub = defineComponent({
  name: 'ModelWhitelistSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <div>
      <button
        type="button"
        data-testid="rewrite-to-snapshot"
        @click="$emit('update:modelValue', ['gpt-5.2-2025-12-11'])"
      >
        rewrite
      </button>
      <span data-testid="model-whitelist-value">
        {{ Array.isArray(modelValue) ? modelValue.join(',') : '' }}
      </span>
    </div>
  `
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: {
      type: [String, Number, Boolean, null],
      default: ''
    },
    options: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <select
      v-bind="$attrs"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
})

const AccountErrorHandlingCardStub = defineComponent({
  name: 'AccountErrorHandlingCard',
  props: {
    rules: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:rules'],
  template: `
    <div data-testid="account-error-handling-card">
      <span data-testid="account-error-handling-rules">
        {{ rules.map(rule => rule.name + ':' + rule.action + ':' + rule.status_codes + ':' + rule.keywords).join('|') }}
      </span>
      <button
        type="button"
        data-testid="replace-error-handling-rules"
        @click="$emit('update:rules', [
          {
            enabled: true,
            name: 'Disable 500',
            priority: 1,
            status_codes: '500',
            error_codes: '',
            error_types: '',
            keywords: '',
            action: 'error_disabled',
            durationMinutes: null,
            reset_strategy: 'daily',
            duration_hours: null,
            daily_reset_hour: null,
            weekly_reset_day: null,
            weekly_reset_hour: null,
            description: 'disable 500'
          },
          {
            enabled: true,
            name: 'Temp 503',
            priority: 2,
            status_codes: '503',
            error_codes: '',
            error_types: '',
            keywords: 'overloaded, slow',
            action: 'temp_unschedulable',
            durationMinutes: 45,
            reset_strategy: 'daily',
            duration_hours: null,
            daily_reset_hour: null,
            weekly_reset_day: null,
            weekly_reset_hour: null,
            description: 'temp 503'
          }
        ])"
      >
        replace rules
      </button>
    </div>
  `
})

function buildAccount() {
  return {
    id: 1,
    name: 'OpenAI Key',
    notes: '',
    platform: 'openai',
    type: 'apikey',
    credentials: {
      api_key: 'sk-test',
      base_url: 'https://api.openai.com',
      model_mapping: {
        'gpt-5.2': 'gpt-5.2'
      }
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function buildAnthropicAPIKeyAccount() {
  const account = buildAccount()
  account.id = 3
  account.name = 'Anthropic Key'
  account.platform = 'anthropic'
  account.type = 'apikey'
  account.credentials = {
    api_key: 'sk-ant-test',
    base_url: 'https://api.anthropic.com',
    model_mapping: {
      'claude-opus-4-7': 'claude-opus-4-7[1m]'
    }
  }
  account.extra = {}
  return account
}

function buildVertexAccount() {
  return {
    id: 2,
    name: 'Vertex SA',
    notes: '',
    platform: 'gemini',
    type: 'service_account',
    credentials: {
      service_account_json: '{"type":"service_account","client_email":"sa@example.iam.gserviceaccount.com","private_key":"-----BEGIN PRIVATE KEY-----\\nMIIE\\n-----END PRIVATE KEY-----\\n"}',
      project_id: 'demo-project',
      client_email: 'sa@example.iam.gserviceaccount.com',
      location: 'us-central1',
      tier_id: 'vertex'
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function mountModal(account = buildAccount()) {
  return mount(EditAccountModal, {
    props: {
      show: true,
      account,
      proxies: [],
      groups: []
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Select: SelectStub,
        Icon: true,
        ProxySelector: true,
        GroupSelector: true,
        ModelWhitelistSelector: ModelWhitelistSelectorStub,
        AccountErrorHandlingCard: AccountErrorHandlingCardStub
      }
    }
  })
}

describe('EditAccountModal', () => {
  it('separates core, advanced, and model settings into three tabs', async () => {
    const wrapper = mountModal()
    await flushPromises()

    expect(wrapper.get('[data-testid="account-form-tab-basic"]').attributes('aria-selected')).toBe('true')
    expect(wrapper.find('[data-testid="account-name-field"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="account-notes-field"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('admin.accounts.openai.requestBaseUrls')
    expect(wrapper.get('[data-testid="model-whitelist-value"]').isVisible()).toBe(false)

    await wrapper.get('[data-testid="account-form-tab-advanced"]').trigger('click')
    await nextTick()

    expect(wrapper.find('[data-testid="account-name-field"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="account-notes-field"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('admin.accounts.openai.requestBaseUrls')
    expect(wrapper.get('[data-testid="model-whitelist-value"]').isVisible()).toBe(false)

    await wrapper.get('[data-testid="account-form-tab-models"]').trigger('click')
    await nextTick()

    expect(wrapper.find('[data-testid="account-name-field"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="account-notes-field"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="model-whitelist-value"]').isVisible()).toBe(true)
    expect(wrapper.text()).not.toContain('admin.accounts.openai.requestBaseUrls')
  })

  it('loads unified account error handling rules and saves legacy compatibility fields', async () => {
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      error_handling_rules: [
        {
          enabled: true,
          name: 'Temp quota 429',
          priority: 1,
          status_codes: [429],
          keywords: ['quota exceeded'],
          action: 'temp_unschedulable',
          durationMinutes: 30,
          description: 'quota temp'
        }
      ]
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.get('[data-testid="account-error-handling-rules"]').text()).toContain('Temp quota 429:temp_unschedulable:429:quota exceeded')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const credentials = updateAccountMock.mock.calls[0]?.[1]?.credentials
    expect(credentials?.error_handling_rules).toEqual([
      {
        enabled: true,
        name: 'Temp quota 429',
        priority: 1,
        status_codes: [429],
        keywords: ['quota exceeded'],
        action: 'temp_unschedulable',
        durationMinutes: 30,
        description: 'quota temp'
      }
    ])
    expect(credentials?.temp_unschedulable_enabled).toBe(true)
    expect(credentials?.temp_unschedulable_rules).toEqual([
      {
        error_code: 429,
        keywords: ['quota exceeded'],
        duration_minutes: 30,
        description: 'quota temp'
      }
    ])
    expect(credentials?.custom_error_codes_enabled).toBeUndefined()
    expect(credentials?.custom_error_codes).toBeUndefined()
  })

  it('saves edited unified account error handling rules with custom code and temp-unsched legacy fields', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="replace-error-handling-rules"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const credentials = updateAccountMock.mock.calls[0]?.[1]?.credentials
    expect(credentials?.error_handling_rules).toEqual([
      {
        enabled: true,
        name: 'Disable 500',
        priority: 1,
        status_codes: [500],
        action: 'error_disabled',
        description: 'disable 500'
      },
      {
        enabled: true,
        name: 'Temp 503',
        priority: 2,
        status_codes: [503],
        keywords: ['overloaded', 'slow'],
        action: 'temp_unschedulable',
        durationMinutes: 45,
        description: 'temp 503'
      }
    ])
    expect(credentials?.custom_error_codes_enabled).toBe(true)
    expect(credentials?.custom_error_codes).toEqual([500])
    expect(credentials?.temp_unschedulable_enabled).toBe(true)
    expect(credentials?.temp_unschedulable_rules).toEqual([
      {
        error_code: 503,
        keywords: ['overloaded', 'slow'],
        duration_minutes: 45,
        description: 'temp 503'
      }
    ])
  })

  it('round-trips legacy custom error codes and temp-unsched rules through the unified edit flow', async () => {
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      custom_error_codes_enabled: true,
      custom_error_codes: [500],
      temp_unschedulable_enabled: true,
      temp_unschedulable_rules: [
        {
          error_code: 503,
          keywords: ['overloaded'],
          duration_minutes: 15,
          description: 'legacy temp'
        }
      ]
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.get('[data-testid="account-error-handling-rules"]').text()).toContain('Legacy custom error 500:error_disabled:500:')
    expect(wrapper.get('[data-testid="account-error-handling-rules"]').text()).toContain('legacy temp:temp_unschedulable:503:overloaded')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const credentials = updateAccountMock.mock.calls[0]?.[1]?.credentials
    expect(credentials?.error_handling_rules).toEqual([
      {
        enabled: true,
        name: 'Legacy custom error 500',
        priority: 1,
        status_codes: [500],
        action: 'error_disabled',
        description: 'Generated from custom_error_codes'
      },
      {
        enabled: true,
        name: 'legacy temp',
        priority: 2,
        status_codes: [503],
        keywords: ['overloaded'],
        action: 'temp_unschedulable',
        durationMinutes: 15
      }
    ])
    expect(credentials?.custom_error_codes_enabled).toBe(true)
    expect(credentials?.custom_error_codes).toEqual([500])
    expect(credentials?.temp_unschedulable_enabled).toBe(true)
    expect(credentials?.temp_unschedulable_rules).toEqual([
      {
        error_code: 503,
        keywords: ['overloaded'],
        duration_minutes: 15,
        description: ''
      }
    ])
  })

  it('deletes an existing API key by fingerprint', async () => {
    const account = {
      ...buildAccount(),
      api_key_items: [
        { fingerprint: 'fp-delete', masked: 'sk-...lete' },
        { fingerprint: 'fp-keep', masked: 'sk-...keep' }
      ]
    }
    const updatedAccount = {
      ...account,
      api_key_items: [{ fingerprint: 'fp-keep', masked: 'sk-...keep' }]
    }
    deleteAccountAPIKeyMock.mockReset()
    deleteAccountAPIKeyMock.mockResolvedValue(updatedAccount)

    const wrapper = mountModal(account)

    await wrapper.get('button[title="admin.accounts.deleteApiKey"]').trigger('click')
    await flushPromises()

    expect(deleteAccountAPIKeyMock).toHaveBeenCalledWith(1, 'fp-delete')
    expect(wrapper.emitted('updated')?.[0]).toEqual([updatedAccount])
  })

  it('restores a disabled existing API key by fingerprint', async () => {
    const account = {
      ...buildAccount(),
      api_key_items: [
        { fingerprint: 'fp-disabled', masked: 'sk-...bled', disabled: true, reason: 'rate_limited' },
        { fingerprint: 'fp-active', masked: 'sk-...tive' }
      ]
    }
    const updatedAccount = {
      ...account,
      api_key_items: [
        { fingerprint: 'fp-disabled', masked: 'sk-...bled' },
        { fingerprint: 'fp-active', masked: 'sk-...tive' }
      ]
    }
    restoreAccountAPIKeyStateMock.mockReset()
    restoreAccountAPIKeyStateMock.mockResolvedValue(updatedAccount)

    const wrapper = mountModal(account)

    await wrapper.get('button[title="admin.accounts.restoreApiKey"]').trigger('click')
    await flushPromises()

    expect(restoreAccountAPIKeyStateMock).toHaveBeenCalledWith(1, 'fp-disabled')
    expect(wrapper.emitted('updated')?.[0]).toEqual([updatedAccount])
  })

  it('renders current API key cooling status details', () => {
    const account = {
      ...buildAccount(),
      api_key_items: [
        { fingerprint: 'fp-active', masked: 'sk-...tive', status: 'active' },
        {
          fingerprint: 'fp-cooling',
          masked: 'sk-...ling',
          disabled: true,
          status: 'cooling',
          reason: 'rate_limited',
          last_error: 'API returned 429: quota exceeded',
          disabled_until: '2026-05-22T00:30:00Z',
          disabled_count: 2
        }
      ]
    }

    const wrapper = mountModal(account)

    const cooling = wrapper.get('[data-testid="api-key-state-fp-cooling"]')
    expect(cooling.text()).toContain('admin.accounts.apiKeyStatusCooling')
    expect(cooling.text()).toContain('rate_limited')
    expect(cooling.text()).toContain('API returned 429: quota exceeded')
    expect(cooling.text()).toContain('2026-05-22T00:30:00Z')
    expect(cooling.text()).toContain('admin.accounts.apiKeyDisabledCount')
    expect(cooling.text()).toContain('2')

    const active = wrapper.get('[data-testid="api-key-state-fp-active"]')
    expect(active.text()).toContain('admin.accounts.apiKeyStatusActive')
  })

  it('reopening the same account rehydrates the OpenAI whitelist from props', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2-2025-12-11')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'gpt-5.2': 'gpt-5.2'
    })
  })

  it('preserves model mappings when editing the whitelist', async () => {
    const account = buildAccount()
    account.credentials.model_mapping = {
      'gpt-5.2': 'gpt-5.2',
      'gpt-latest': 'gpt-5.2'
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'gpt-5.2-2025-12-11': 'gpt-5.2-2025-12-11',
      'gpt-latest': 'gpt-5.2'
    })
  })

  it('submits OpenAI compact mode and compact-only model mapping', async () => {
    const account = buildAccount()
    account.extra = {
      openai_compact_mode: 'force_on'
    }
    account.credentials = {
      ...account.credentials,
      compact_model_mapping: {
        'gpt-5.4': 'gpt-5.4-openai-compact'
      }
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_compact_mode).toBe('force_on')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.compact_model_mapping).toEqual({
      'gpt-5.4': 'gpt-5.4-openai-compact'
    })
  })

  it('submits OpenAI APIKey Responses support override mode', async () => {
    const account = buildAccount()
    account.extra = {
      openai_responses_mode: 'force_chat_completions',
      openai_responses_supported: false
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="openai-responses-mode-select"]').setValue('force_responses')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_responses_mode).toBe('force_responses')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_responses_supported).toBe(false)
  })

  it('shows and saves upstream balance fields for Anthropic API key accounts', async () => {
    const account = {
      ...buildAccount(),
      platform: 'anthropic',
      type: 'apikey',
      credentials: {
        api_key: 'sk-ant-existing',
        base_url: 'https://api.anthropic.com',
        upstream_auth_username: 'alice@example.com',
        upstream_auth_password_set: true,
        upstream_common_rate_multiplier: 0.42,
        upstream_common_rate_group_name: 'claude',
        upstream_balance_endpoint_paths: ['/v1/usage']
      },
      extra: {}
    } as any
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.text()).toContain('admin.accounts.upstream.authUsername')
    expect(wrapper.text()).toContain('admin.accounts.upstream.commonRateMultiplier')
    expect(wrapper.text()).toContain('admin.accounts.upstream.balanceEndpointPaths')

    const inputs = wrapper.findAll('input')
    const username = inputs.find(input => input.attributes('autocomplete') === 'username')
    expect(username).toBeTruthy()
    await username!.setValue('bob@example.com')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const credentials = updateAccountMock.mock.calls[0]?.[1]?.credentials
    expect(credentials?.upstream_auth_username).toBe('bob@example.com')
    expect(credentials?.upstream_manual_rate_multiplier).toBe(0.42)
    expect(credentials?.upstream_manual_rate_group_name).toBe('claude')
    expect(credentials?.upstream_common_rate_multiplier).toBeUndefined()
    expect(credentials?.upstream_common_rate_group_name).toBeUndefined()
    expect(credentials?.upstream_balance_endpoint_paths).toEqual(['/v1/usage'])
  })

  it('loads and saves account availability schedule extra', async () => {
    const account = buildAccount()
    account.extra = {
      keep_me: true,
      availability_schedule: {
        enabled: true,
        timezone: 'UTC',
        mode: 'allow_windows',
        windows: [{ daysOfWeek: [1], start: '09:00', end: '17:00' }],
        exceptions: [{ date: '2026-06-08', action: 'deny' }]
      }
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.text()).toContain('admin.accounts.availabilitySchedule.title')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.keep_me).toBe(true)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.availability_schedule).toEqual({
      enabled: true,
      timezone: 'UTC',
      mode: 'allow_windows',
      windows: [{ daysOfWeek: [1], start: '09:00', end: '17:00' }],
      exceptions: [{ date: '2026-06-08', action: 'deny' }]
    })
  })

  it('loads and saves anthropic account Claude CLI version override', async () => {
    const account = buildAnthropicAPIKeyAccount()
    account.credentials = {
      ...account.credentials,
      claude_cli_version: '2.1.126'
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('[data-testid="account-form-tab-advanced"]').trigger('click')

    const versionInput = wrapper.get('[data-testid="claude-cli-version-input"]')
    expect((versionInput.element as HTMLInputElement).value).toBe('2.1.126')

    await versionInput.setValue('2.1.183')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.claude_cli_version).toBe('2.1.183')
  })

  it('clears OpenAI APIKey Responses override when set back to auto', async () => {
    const account = buildAccount()
    account.extra = {
      openai_responses_mode: 'force_chat_completions',
      openai_responses_supported: true
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="openai-responses-mode-select"]').setValue('auto')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('openai_responses_mode')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_responses_supported).toBe(true)
  })

  it('submits account-level Codex image generation bridge override', async () => {
    const account = buildAccount()
    account.extra = {
      codex_image_generation_bridge: false,
      codex_image_generation_bridge_enabled: true
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('button[data-testid="codex-image-bridge-enabled"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.codex_image_generation_bridge).toBe(true)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('codex_image_generation_bridge_enabled')
  })

  it('submits Anthropic APIKey 1M context switch state', async () => {
    const account = buildAnthropicAPIKeyAccount()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const contextButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('admin.accounts.anthropic.context1MDisabled'))

    expect(contextButton).toBeTruthy()
    await contextButton!.trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.anthropic_context_1m_enabled).toBe(true)
  })

  it('allows saving apikey account when backend redacted api_key but credentials_status reports it exists', async () => {
    // 新前端 + 新后端：响应已脱敏，credentials 里没有 api_key，credentials_status.has_api_key=true
    const account = buildAccount()
    account.credentials = {
      base_url: 'https://api.openai.com',
      model_mapping: { 'gpt-5.2': 'gpt-5.2' }
    }
    account.credentials_status = { has_api_key: true }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    // 用户未输入新 key 时，payload 不应带 api_key，由后端合并保留旧值
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('api_key')
  })

  it('applies readonly load factor suggestion only after clicking apply', async () => {
    const account = buildAccount()
    account.load_factor = 3
    account.load_factor_advice = {
      suggested_load_factor: 12,
      reasons: ['成功率 90%'],
      availability_radar: {
        status: 'fast_stable',
        label: '快且稳'
      },
      path_health_samples: 8
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const loadFactorInput = wrapper.get<HTMLInputElement>('[data-testid="load-factor-input"]')

    expect(loadFactorInput.element.value).toBe('3')
    expect(wrapper.text()).toContain('admin.accounts.applyLoadFactorSuggestion')

    await wrapper.get('button[title="成功率 90%"]').trigger('click')
    await nextTick()
    expect(loadFactorInput.element.value).toBe('12')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.load_factor).toBe(12)
  })

  it('allows saving apikey account against legacy backend without credentials_status', async () => {
    // 新前端 + 旧后端：credentials_status 缺失，但 credentials.api_key 仍是明文，应允许保存
    const account = buildAccount()
    // 显式确保没有 credentials_status
    expect(account.credentials_status).toBeUndefined()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    // 旧后端响应未脱敏，原 api_key 会随 currentCredentials 一起传回去（旧行为，等价于无操作）
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.api_key).toBe('sk-test')
  })

  it('submits manual upstream rate without using fetched common rate cache', async () => {
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      upstream_manual_rate_multiplier: 7.5,
      upstream_manual_rate_group_name: 'manual-v2'
    }
    account.extra = {
      upstream_common_rate_multiplier: 0.2,
      upstream_common_rate_group_name: 'login-group'
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const credentials = updateAccountMock.mock.calls[0]?.[1]?.credentials
    expect(credentials?.upstream_manual_rate_multiplier).toBe(7.5)
    expect(credentials?.upstream_manual_rate_group_name).toBe('manual-v2')
    expect(credentials?.upstream_common_rate_multiplier).toBeUndefined()
    expect(credentials?.upstream_common_rate_group_name).toBeUndefined()
  })

  it('loads and saves OpenAI Codex CLI User-Agent credentials', async () => {
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      openai_codex_cli_user_agent:
        'Codex Desktop/0.142.2 (Windows 10.0.26200; x86_64) unknown (Codex Desktop; 26.623.30605)'
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('[data-testid="account-form-tab-advanced"]').trigger('click')
    const userAgentInput = wrapper.get('[data-testid="openai-codex-cli-user-agent-input"]')
    expect((userAgentInput.element as HTMLInputElement).value).toBe(
      'Codex Desktop/0.142.2 (Windows 10.0.26200; x86_64) unknown (Codex Desktop; 26.623.30605)'
    )

    await userAgentInput.setValue(
      'Codex Desktop/0.150.0 (Windows 10.0.26200; x86_64) unknown (Codex Desktop; 26.700.10000)'
    )
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const credentials = updateAccountMock.mock.calls[0]?.[1]?.credentials
    expect(credentials?.openai_codex_cli_user_agent).toBe(
      'Codex Desktop/0.150.0 (Windows 10.0.26200; x86_64) unknown (Codex Desktop; 26.700.10000)'
    )
  })

  it('blocks apikey save when neither credentials_status nor legacy api_key indicates existence', async () => {
    const account = buildAccount()
    account.credentials = {
      base_url: 'https://api.openai.com'
    }
    // 既没有 credentials_status 也没有旧的 api_key
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).not.toHaveBeenCalled()
  })

  it('allows saving Vertex SA account when backend redacted service_account_json but credentials_status reports it exists', async () => {
    // 新前端 + 新后端：响应已脱敏，credentials 里没有 service_account_json，credentials_status.has_service_account_json=true
    const account = buildVertexAccount()
    account.credentials = {
      project_id: 'demo-project',
      client_email: 'sa@example.iam.gserviceaccount.com',
      location: 'us-central1',
      tier_id: 'vertex'
    }
    account.credentials_status = { has_service_account_json: true }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.project_id).toBe('demo-project')
  })

  it('allows saving Vertex SA account against legacy backend without credentials_status', async () => {
    // 新前端 + 旧后端：credentials_status 缺失，但 credentials.service_account_json 仍是明文，应允许保存
    const account = buildVertexAccount()
    expect(account.credentials_status).toBeUndefined()
    expect(account.credentials.service_account_json).toBeTruthy()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
  })

  it('blocks Vertex SA save when neither credentials_status nor legacy json indicates existence', async () => {
    const account = buildVertexAccount()
    account.credentials = {
      project_id: 'demo-project',
      client_email: 'sa@example.iam.gserviceaccount.com',
      location: 'us-central1',
      tier_id: 'vertex'
    }
    // 既没有 credentials_status 也没有旧的 service_account_json
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).not.toHaveBeenCalled()
  })
})
