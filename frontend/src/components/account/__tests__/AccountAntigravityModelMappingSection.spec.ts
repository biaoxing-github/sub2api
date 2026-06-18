import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

import AccountAntigravityModelMappingSection from '../AccountAntigravityModelMappingSection.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const AccountModelMappingListStub = defineComponent({
  name: 'AccountModelMappingList',
  props: {
    modelMappings: {
      type: Array,
      required: true
    },
    showSyncButton: {
      type: Boolean,
      default: false
    },
    syncLoading: {
      type: Boolean,
      default: false
    },
    syncDisabled: {
      type: Boolean,
      default: false
    }
  },
  emits: ['update:modelMappings', 'sync'],
  template: `
    <button
      type="button"
      data-testid="mapping-list"
      @click="$emit('update:modelMappings', [{ from: 'ag-*', to: 'ag-model' }]); $emit('sync')"
    >
      mappings {{ modelMappings.length }} sync {{ showSyncButton }} {{ syncLoading }} {{ syncDisabled }}
    </button>
  `
})

describe('AccountAntigravityModelMappingSection', () => {
  it('renders the shared antigravity title and forwards mapping and sync events', async () => {
    const wrapper = mount(AccountAntigravityModelMappingSection, {
      props: {
        modelMappings: [{ from: 'old-*', to: 'old-model' }],
        presetMappings: [],
        showSyncButton: true,
        syncLoading: true,
        syncDisabled: false
      },
      global: {
        stubs: {
          AccountModelMappingList: AccountModelMappingListStub
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.modelRestriction')
    expect(wrapper.text()).toContain('sync true true false')

    await wrapper.get('[data-testid="mapping-list"]').trigger('click')
    expect(wrapper.emitted('update:modelMappings')?.[0]).toEqual([
      [{ from: 'ag-*', to: 'ag-model' }]
    ])
    expect(wrapper.emitted('sync')?.length).toBe(1)
  })
})
