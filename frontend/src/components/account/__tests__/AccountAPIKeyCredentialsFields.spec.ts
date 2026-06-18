import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

import AccountAPIKeyCredentialsFields from '../AccountAPIKeyCredentialsFields.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const SelectStub = defineComponent({
  name: 'Select',
  props: {
    modelValue: {
      type: String,
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
      data-testid="api-keys-edit-mode"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
})

describe('AccountAPIKeyCredentialsFields', () => {
  it('emits create-mode base URL, request URL, balance URL and API key edits', async () => {
    const wrapper = mount(AccountAPIKeyCredentialsFields, {
      props: {
        platform: 'openai',
        baseUrl: 'https://api.openai.com',
        requestBaseUrlsText: '',
        balanceBaseUrl: '',
        apiKey: '',
        apiKeysText: '',
        baseUrlHint: 'base hint',
        apiKeyHint: 'key hint',
        mode: 'create'
      },
      global: {
        stubs: {
          Icon: true,
          Select: SelectStub
        }
      }
    })

    const inputs = wrapper.findAll('input')
    const textareas = wrapper.findAll('textarea')

    expect(wrapper.text()).toContain('admin.accounts.apiKeyRequired')
    expect(inputs[0].attributes('placeholder')).toBe('https://api.openai.com')
    expect(inputs[1].attributes('placeholder')).toBe('https://api.openai.com')
    expect(inputs[2].attributes('placeholder')).toBe('sk-proj-...')

    await inputs[0].setValue('https://gateway.example.com')
    await textareas[0].setValue('https://a.example.com\nhttps://b.example.com')
    await inputs[1].setValue('https://balance.example.com')
    await inputs[2].setValue('sk-proj-new')
    await textareas[1].setValue('sk-a\nsk-b')

    expect(wrapper.emitted('update:baseUrl')?.[0]).toEqual(['https://gateway.example.com'])
    expect(wrapper.emitted('update:requestBaseUrlsText')?.[0]).toEqual([
      'https://a.example.com\nhttps://b.example.com'
    ])
    expect(wrapper.emitted('update:balanceBaseUrl')?.[0]).toEqual(['https://balance.example.com'])
    expect(wrapper.emitted('update:apiKey')?.[0]).toEqual(['sk-proj-new'])
    expect(wrapper.emitted('update:apiKeysText')?.[0]).toEqual(['sk-a\nsk-b'])
  })

  it('renders edit-mode existing keys and emits edit mode, delete and restore actions', async () => {
    const wrapper = mount(AccountAPIKeyCredentialsFields, {
      props: {
        platform: 'anthropic',
        baseUrl: 'https://api.anthropic.com',
        requestBaseUrlsText: '',
        balanceBaseUrl: '',
        apiKey: '',
        apiKeysText: '',
        baseUrlHint: 'base hint',
        mode: 'edit',
        existingApiKeyItems: [
          { fingerprint: 'fp-active', masked: 'sk-...active' },
          { fingerprint: 'fp-disabled', masked: 'sk-...disabled', disabled: true, reason: 'quota' }
        ],
        existingApiKeySummary: '2 total, 1 disabled',
        apiKeysEditMode: 'append',
        apiKeysEditModeOptions: [
          { value: 'append', label: 'append' },
          { value: 'replace', label: 'replace' }
        ],
        apiKeysEditModeHint: 'append hint',
        deletingApiKeyFingerprint: '',
        restoringApiKeyFingerprint: ''
      },
      global: {
        stubs: {
          Icon: true,
          Select: SelectStub
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.apiKey')
    expect(wrapper.text()).toContain('admin.accounts.leaveEmptyToKeep')
    expect(wrapper.text()).toContain('sk-...active')
    expect(wrapper.text()).toContain('sk-...disabled')
    expect(wrapper.text()).toContain('2 total, 1 disabled')
    expect(wrapper.findAll('textarea')).toHaveLength(2)
    expect(wrapper.findAll('input')).toHaveLength(2)

    await wrapper.get('[data-testid="api-keys-edit-mode"]').setValue('replace')
    await wrapper.get('button[title="admin.accounts.restoreApiKey"]').trigger('click')
    const deleteButtons = wrapper.findAll('button[title="admin.accounts.deleteApiKey"]')
    expect(deleteButtons).toHaveLength(2)
    await deleteButtons[1].trigger('click')

    expect(wrapper.emitted('update:apiKeysEditMode')?.[0]).toEqual(['replace'])
    expect(wrapper.emitted('restoreApiKey')?.[0]).toEqual(['fp-disabled'])
    expect(wrapper.emitted('deleteApiKey')?.[0]).toEqual(['fp-disabled'])
  })

  it('renders current key state details for cooling keys', () => {
    const wrapper = mount(AccountAPIKeyCredentialsFields, {
      props: {
        platform: 'openai',
        baseUrl: 'https://api.openai.com',
        requestBaseUrlsText: '',
        balanceBaseUrl: '',
        apiKey: '',
        apiKeysText: '',
        baseUrlHint: 'base hint',
        mode: 'edit',
        existingApiKeyItems: [
          { fingerprint: 'fp-active', masked: 'sk-...active', status: 'active' },
          {
            fingerprint: 'fp-cooling',
            masked: 'sk-...cooling',
            disabled: true,
            status: 'cooling',
            reason: 'rate_limited',
            disabled_until: '2026-05-22T00:30:00Z',
            disabled_count: 2
          }
        ],
        existingApiKeySummary: '2 total, 1 cooling'
      },
      global: {
        stubs: {
          Icon: true,
          Select: SelectStub
        }
      }
    })

    const cooling = wrapper.get('[data-testid="api-key-state-fp-cooling"]')
    expect(cooling.text()).toContain('admin.accounts.apiKeyStatusCooling')
    expect(cooling.text()).toContain('rate_limited')
    expect(cooling.text()).toContain('2026-05-22T00:30:00Z')
    expect(cooling.text()).toContain('admin.accounts.apiKeyDisabledCount')
    expect(cooling.text()).toContain('2')

    const active = wrapper.get('[data-testid="api-key-state-fp-active"]')
    expect(active.text()).toContain('admin.accounts.apiKeyStatusActive')
  })
})
