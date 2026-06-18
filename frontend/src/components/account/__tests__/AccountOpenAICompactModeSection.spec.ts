import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

import AccountOpenAICompactModeSection from '../AccountOpenAICompactModeSection.vue'

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
      required: true
    },
    options: {
      type: Array,
      required: true
    }
  },
  emits: ['update:modelValue'],
  template: `
    <button type="button" data-testid="compact-mode" @click="$emit('update:modelValue', 'compact_only')">
      mode {{ modelValue }} options {{ options.length }}
    </button>
  `
})

const AccountModelMappingListStub = defineComponent({
  name: 'AccountModelMappingList',
  props: {
    modelMappings: {
      type: Array,
      required: true
    }
  },
  emits: ['update:modelMappings'],
  template: `
    <button
      type="button"
      data-testid="compact-mapping"
      @click="$emit('update:modelMappings', [{ from: 'gpt-5', to: 'gpt-5-mini' }])"
    >
      mappings {{ modelMappings.length }}
    </button>
  `
})

describe('AccountOpenAICompactModeSection', () => {
  it('forwards compact mode and compact mapping updates', async () => {
    const wrapper = mount(AccountOpenAICompactModeSection, {
      props: {
        compactMode: 'auto',
        compactModeOptions: [
          { value: 'auto', label: 'Auto' },
          { value: 'compact_only', label: 'Compact' }
        ],
        compactModelMappings: []
      },
      global: {
        stubs: {
          Select: SelectStub,
          AccountModelMappingList: AccountModelMappingListStub
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.openai.compactMode')
    await wrapper.get('[data-testid="compact-mode"]').trigger('click')
    await wrapper.get('[data-testid="compact-mapping"]').trigger('click')

    expect(wrapper.emitted('update:compactMode')?.[0]).toEqual(['compact_only'])
    expect(wrapper.emitted('update:compactModelMappings')?.[0]).toEqual([
      [{ from: 'gpt-5', to: 'gpt-5-mini' }]
    ])
  })

  it('shows edit status and last checked text when provided', () => {
    const wrapper = mount(AccountOpenAICompactModeSection, {
      props: {
        compactMode: 'auto',
        compactModeOptions: [{ value: 'auto', label: 'Auto' }],
        compactModelMappings: [],
        statusKey: 'admin.accounts.openai.compactStatusSupported',
        lastCheckedText: '2026-06-17 12:00:00'
      },
      global: {
        stubs: {
          Select: SelectStub,
          AccountModelMappingList: AccountModelMappingListStub
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.openai.compactStatusSupported')
    expect(wrapper.text()).toContain('admin.accounts.openai.compactLastChecked')
    expect(wrapper.text()).toContain('2026-06-17 12:00:00')
  })
})
