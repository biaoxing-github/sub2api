import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import SettingsSaveBar from '@/components/admin/settings/SettingsSaveBar.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('SettingsSaveBar', () => {
  it('renders a submit button with idle and saving labels', async () => {
    const wrapper = mount(SettingsSaveBar, {
      props: {
        saving: false,
        loadFailed: false
      }
    })

    const button = wrapper.get('[data-testid="settings-save-button"]')
    expect(button.attributes('type')).toBe('submit')
    expect(button.text()).toContain('admin.settings.saveSettings')
    expect(button.attributes('disabled')).toBeUndefined()

    await wrapper.setProps({ saving: true })
    expect(button.text()).toContain('admin.settings.saving')
    expect(button.attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="settings-save-spinner"]').exists()).toBe(true)
  })

  it('disables the submit button when loading settings failed', () => {
    const wrapper = mount(SettingsSaveBar, {
      props: {
        saving: false,
        loadFailed: true
      }
    })

    expect(wrapper.get('[data-testid="settings-save-button"]').attributes('disabled')).toBeDefined()
  })
})
