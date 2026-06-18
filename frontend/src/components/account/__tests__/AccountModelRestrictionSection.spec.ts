import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

import AccountModelRestrictionSection from '../AccountModelRestrictionSection.vue'

const showInfoMock = vi.fn()

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showInfo: showInfoMock
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params?.count != null
          ? `${key}:${params.count}`
          : params?.model != null
            ? `${key}:${params.model}`
            : key
    })
  }
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
    <button
      type="button"
      data-testid="replace-whitelist"
      @click="$emit('update:modelValue', ['gpt-5.2'])"
    >
      {{ modelValue.join(',') }}
    </button>
  `
})

const baseProps = {
  platform: 'openai',
  mode: 'whitelist' as const,
  allowedModels: [] as string[],
  modelMappings: [] as Array<{ from: string; to: string }>,
  presetMappings: [
    { label: 'GPT 5.2', from: 'gpt-latest', to: 'gpt-5.2', color: 'bg-slate-100' }
  ]
}

const mountSection = (props = {}) =>
  mount(AccountModelRestrictionSection, {
    props: {
      ...baseProps,
      ...props
    },
    global: {
      stubs: {
        ModelWhitelistSelector: ModelWhitelistSelectorStub,
        Icon: true
      }
    }
  })

describe('AccountModelRestrictionSection', () => {
  it('emits mode and whitelist updates', async () => {
    const wrapper = mountSection()

    await wrapper.get('[data-testid="model-mode-mapping"]').trigger('click')
    expect(wrapper.emitted('update:mode')?.[0]).toEqual(['mapping'])

    await wrapper.get('[data-testid="replace-whitelist"]').trigger('click')
    expect(wrapper.emitted('update:allowedModels')?.[0]).toEqual([['gpt-5.2']])
  })

  it('emits cloned mapping updates for add edit remove and preset actions', async () => {
    const wrapper = mountSection({
      mode: 'mapping',
      modelMappings: [{ from: 'gpt-old', to: 'gpt-new' }]
    })

    await wrapper.get('[data-testid="add-model-mapping"]').trigger('click')
    expect(wrapper.emitted('update:modelMappings')?.[0]).toEqual([
      [
        { from: 'gpt-old', to: 'gpt-new' },
        { from: '', to: '' }
      ]
    ])

    await wrapper.get('[data-testid="mapping-from-0"]').setValue('gpt-latest')
    expect(wrapper.emitted('update:modelMappings')?.[1]).toEqual([
      [{ from: 'gpt-latest', to: 'gpt-new' }]
    ])

    await wrapper.get('[data-testid="mapping-to-0"]').setValue('gpt-5.2')
    expect(wrapper.emitted('update:modelMappings')?.[2]).toEqual([
      [{ from: 'gpt-old', to: 'gpt-5.2' }]
    ])

    await wrapper.get('[data-testid="remove-model-mapping-0"]').trigger('click')
    expect(wrapper.emitted('update:modelMappings')?.[3]).toEqual([[]])

    await wrapper.get('[data-testid="preset-mapping-gpt-latest"]').trigger('click')
    expect(wrapper.emitted('update:modelMappings')?.[4]).toEqual([
      [
        { from: 'gpt-old', to: 'gpt-new' },
        { from: 'gpt-latest', to: 'gpt-5.2' }
      ]
    ])
  })

  it('shows duplicate preset feedback without emitting a mapping update', async () => {
    showInfoMock.mockClear()
    const wrapper = mountSection({
      mode: 'mapping',
      modelMappings: [{ from: 'gpt-latest', to: 'gpt-5.2' }]
    })

    await wrapper.get('[data-testid="preset-mapping-gpt-latest"]').trigger('click')

    expect(showInfoMock).toHaveBeenCalledWith('admin.accounts.mappingExists:gpt-latest')
    expect(wrapper.emitted('update:modelMappings')).toBeUndefined()
  })

  it('only shows the disabled hint when model restriction is disabled', () => {
    const wrapper = mountSection({
      disabled: true,
      disabledHintKey: 'admin.accounts.openai.modelRestrictionDisabledByPassthrough'
    })

    expect(wrapper.text()).toContain('admin.accounts.openai.modelRestrictionDisabledByPassthrough')
    expect(wrapper.find('[data-testid="model-mode-mapping"]').exists()).toBe(false)
    expect(wrapper.findComponent(ModelWhitelistSelectorStub).exists()).toBe(false)
  })
})
