import { describe, expect, it, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { nextTick } from 'vue'
import { useAppStore, useAuthStore } from '@/stores'
import VersionBadge from '@/components/common/VersionBadge.vue'

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      locale: { value: 'en' },
      setLocaleMessage: vi.fn(),
      t: (key: string) => key
    }
  }),
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/api/admin/system', () => ({
  checkUpdates: vi.fn(),
  performUpdate: vi.fn(),
  restartService: vi.fn(),
  default: {
    checkUpdates: vi.fn(),
    performUpdate: vi.fn(),
    restartService: vi.fn()
  }
}))

describe('VersionBadge', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('在管理员版本下拉中同时展示主版本和镜像子版本', async () => {
    const authStore = useAuthStore()
    const appStore = useAppStore()

    ;(authStore as any).user = { id: 1, role: 'admin' }
    appStore.currentVersion = '0.1.134'
    ;(appStore as any).imageVersion = 'v0.1.134.3'
    appStore.latestVersion = '0.1.134'
    appStore.hasUpdate = false
    appStore.buildType = 'release'
    appStore.versionLoaded = true

    const wrapper = mount(VersionBadge, {
      global: {
        stubs: {
          Icon: true,
          transition: false
        }
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('v0.1.134')
    expect(wrapper.text()).toContain('version.imageVersion')
    expect(wrapper.text()).toContain('v0.1.134.3')
  })
})
