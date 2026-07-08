import { mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminToolsView from '@/views/admin/AdminToolsView.vue'

const routeState = reactive<{ query: Record<string, string> }>({ query: {} })
const replace = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ replace })
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

describe('AdminToolsView', () => {
  beforeEach(() => {
    routeState.query = {}
    replace.mockReset()
  })

  it('defaults to the native NewApi checkin tool', () => {
    const wrapper = mount(AdminToolsView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          NewApiCheckinTool: { template: '<section data-testid="newapi-tool" />' },
          TokenCostTool: { template: '<section data-testid="token-cost-tool" />' }
        }
      }
    })

    expect(wrapper.get('[role="tab"][aria-selected="true"]').text()).toBe('admin.tools.tabs.newapi')
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.get('[data-testid="newapi-tool"]').exists()).toBe(true)
  })

  it('selects the native token cost tool from query and updates query on click', async () => {
    routeState.query = { tab: 'token-cost' }
    const wrapper = mount(AdminToolsView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          NewApiCheckinTool: { template: '<section data-testid="newapi-tool" />' },
          TokenCostTool: { template: '<section data-testid="token-cost-tool" />' }
        }
      }
    })

    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.get('[data-testid="token-cost-tool"]').exists()).toBe(true)

    await wrapper.get('#admin-tools-tab-newapi').trigger('click')
    expect(replace).toHaveBeenCalledWith({
      query: {
        tab: 'newapi'
      }
    })
  })
})
