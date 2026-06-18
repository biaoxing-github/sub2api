import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountModelMappingList from '../AccountModelMappingList.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showInfo: vi.fn()
  })
}))

const presetMappings = [
  { label: 'Claude', from: 'claude-*', to: 'claude-sonnet-4', color: 'bg-slate-100' }
]

describe('AccountModelMappingList', () => {
  it('emits cloned mapping updates for row edits add remove and presets', async () => {
    const wrapper = mount(AccountModelMappingList, {
      props: {
        modelMappings: [{ from: 'old', to: 'new' }],
        presetMappings
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    await wrapper.get('[data-testid="mapping-from-0"]').setValue('from-updated')
    expect(wrapper.emitted('update:modelMappings')?.[0]).toEqual([
      [{ from: 'from-updated', to: 'new' }]
    ])

    await wrapper.get('[data-testid="mapping-to-0"]').setValue('to-updated')
    expect(wrapper.emitted('update:modelMappings')?.[1]).toEqual([
      [{ from: 'old', to: 'to-updated' }]
    ])

    await wrapper.get('[data-testid="add-model-mapping"]').trigger('click')
    expect(wrapper.emitted('update:modelMappings')?.[2]).toEqual([
      [
        { from: 'old', to: 'new' },
        { from: '', to: '' }
      ]
    ])

    await wrapper.get('[data-testid="remove-model-mapping-0"]').trigger('click')
    expect(wrapper.emitted('update:modelMappings')?.[3]).toEqual([[]])

    await wrapper.get('[data-testid="preset-mapping-claude-*"]').trigger('click')
    expect(wrapper.emitted('update:modelMappings')?.[4]).toEqual([
      [
        { from: 'old', to: 'new' },
        { from: 'claude-*', to: 'claude-sonnet-4' }
      ]
    ])
  })

  it('shows wildcard validation errors and emits sync requests', async () => {
    const wrapper = mount(AccountModelMappingList, {
      props: {
        modelMappings: [{ from: 'bad*pattern', to: 'target-*' }],
        presetMappings: [],
        validateWildcard: true,
        showSyncButton: true,
        syncLoading: true,
        syncDisabled: false
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.wildcardOnlyAtEnd')
    expect(wrapper.text()).toContain('admin.accounts.targetNoWildcard')

    const syncButton = wrapper.get('[data-testid="sync-upstream-models"]')
    expect(syncButton.attributes('disabled')).toBeDefined()

    await wrapper.setProps({ syncLoading: false })
    await syncButton.trigger('click')
    expect(wrapper.emitted('sync')?.length).toBe(1)
  })
})
