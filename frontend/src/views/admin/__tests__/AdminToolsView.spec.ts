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

  it('defaults to the NewApi checkin iframe', () => {
    const wrapper = mount(AdminToolsView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' }
        }
      }
    })

    expect(wrapper.get('[role="tab"][aria-selected="true"]').text()).toBe('admin.tools.tabs.newapi')
    expect(wrapper.get('iframe').attributes('src')).toBe('/newapi-checkin/index.html')
  })

  it('selects the token cost iframe from query and updates query on click', async () => {
    routeState.query = { tab: 'token-cost' }
    const wrapper = mount(AdminToolsView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' }
        }
      }
    })

    expect(wrapper.get('iframe').attributes('src')).toBe('/token-cost/index.html')

    await wrapper.get('#admin-tools-tab-newapi').trigger('click')
    expect(replace).toHaveBeenCalledWith({
      query: {
        tab: 'newapi'
      }
    })
  })
})
