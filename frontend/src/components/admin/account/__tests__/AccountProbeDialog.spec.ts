import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountProbeDialog from '../AccountProbeDialog.vue'

const { listProbeRuns } = vi.hoisted(() => ({
  listProbeRuns: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      listProbeRuns,
      createProbeRun: vi.fn(),
      getProbeRun: vi.fn()
    }
  }
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

describe('AccountProbeDialog', () => {
  beforeEach(() => {
    listProbeRuns.mockReset()
  })

  it('历史接口返回 null 时仍展示空历史状态', async () => {
    listProbeRuns.mockResolvedValueOnce(null)

    const wrapper = mount(AccountProbeDialog, {
      props: {
        show: false,
        modelId: 'gpt-5.4',
        account: {
          id: 181,
          name: 'foyeapi',
          platform: 'openai',
          type: 'apikey',
          status: 'active'
        }
      } as any,
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          Icon: true
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(listProbeRuns).toHaveBeenCalledWith(181)
    expect(wrapper.text()).toContain('admin.accounts.probe.noHistory')
    expect(wrapper.text()).toContain('admin.accounts.probe.run')
  })
})
