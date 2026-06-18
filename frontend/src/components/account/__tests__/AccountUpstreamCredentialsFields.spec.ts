import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountUpstreamCredentialsFields from '../AccountUpstreamCredentialsFields.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('AccountUpstreamCredentialsFields', () => {
  it('emits base URL and API key updates for required create fields', async () => {
    const wrapper = mount(AccountUpstreamCredentialsFields, {
      props: {
        baseUrl: '',
        apiKey: '',
        apiKeyHintKey: 'admin.accounts.upstream.apiKeyHint',
        required: true
      }
    })

    const inputs = wrapper.findAll('input')

    expect(inputs[0].attributes('required')).toBeDefined()
    expect(inputs[1].attributes('required')).toBeDefined()
    expect(wrapper.text()).toContain('admin.accounts.upstream.apiKeyHint')

    await inputs[0].setValue('https://cloudcode-pa.googleapis.com')
    await inputs[1].setValue('sk-upstream')

    expect(wrapper.emitted('update:baseUrl')?.[0]).toEqual(['https://cloudcode-pa.googleapis.com'])
    expect(wrapper.emitted('update:apiKey')?.[0]).toEqual(['sk-upstream'])
  })

  it('uses the keep-existing hint when editing optional upstream API key', () => {
    const wrapper = mount(AccountUpstreamCredentialsFields, {
      props: {
        baseUrl: 'https://cloudcode-pa.googleapis.com',
        apiKey: '',
        apiKeyHintKey: 'admin.accounts.leaveEmptyToKeep'
      }
    })

    const inputs = wrapper.findAll('input')

    expect(inputs[0].attributes('required')).toBeUndefined()
    expect(inputs[1].attributes('required')).toBeUndefined()
    expect(wrapper.text()).toContain('admin.accounts.leaveEmptyToKeep')
  })
})
