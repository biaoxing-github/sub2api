import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountProbeDialog from '../AccountProbeDialog.vue'

const { createProbeRun, getProbeRun, listProbeRuns } = vi.hoisted(() => ({
  createProbeRun: vi.fn(),
  getProbeRun: vi.fn(),
  listProbeRuns: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      listProbeRuns,
      createProbeRun,
      getProbeRun
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
    createProbeRun.mockReset()
    getProbeRun.mockReset()
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

  it('创建测速后立即展示后台执行提示', async () => {
    listProbeRuns.mockResolvedValue([])
    createProbeRun.mockResolvedValueOnce({
      id: 99,
      account_id: 181,
      mode: 'quick',
      status: 'running',
      model: 'gpt-5.4',
      request_count: 1,
      success_count: 0,
      failure_count: 0,
      created_at: '2026-05-25T10:00:00Z'
    })

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
    const runButton = wrapper.findAll('button').find((button) => button.text().includes('admin.accounts.probe.run'))
    expect(runButton).toBeTruthy()
    await runButton!.trigger('click')
    await flushPromises()

    expect(createProbeRun).toHaveBeenCalledWith(181, {
      mode: 'quick',
      model: 'gpt-5.4',
      request_mode: 'non_stream',
      codex_stability: false,
      long_context: false
    })
    expect(wrapper.text()).toContain('admin.accounts.probe.started')
    expect(wrapper.text()).toContain('running')
  })

  it('可以选择流式上游测速模式', async () => {
    listProbeRuns.mockResolvedValue([])
    createProbeRun.mockResolvedValueOnce({
      id: 100,
      account_id: 181,
      mode: 'quick',
      request_mode: 'stream',
      status: 'running',
      model: 'gpt-5.4',
      request_count: 1,
      success_count: 0,
      failure_count: 0,
      created_at: '2026-05-25T10:00:00Z'
    })

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
    const streamButton = wrapper.findAll('button').find((button) => button.text().includes('admin.accounts.probe.requestModes.stream'))
    expect(streamButton).toBeTruthy()
    await streamButton!.trigger('click')
    const runButton = wrapper.findAll('button').find((button) => button.text().includes('admin.accounts.probe.run'))
    await runButton!.trigger('click')
    await flushPromises()

    expect(createProbeRun).toHaveBeenCalledWith(181, expect.objectContaining({
      request_mode: 'stream'
    }))
    expect(wrapper.text()).toContain('admin.accounts.probe.requestModes.stream')
  })

  it('历史列表和详情展示每次测速自己的模型', async () => {
    listProbeRuns.mockResolvedValueOnce([
      {
        id: 201,
        account_id: 181,
        mode: 'quick',
        request_mode: 'non_stream',
        status: 'success',
        model: 'gpt-4.1-mini',
        success_count: 1,
        failure_count: 0,
        avg_latency_ms: 123,
        created_at: '2026-05-25T10:00:00Z'
      }
    ])
    getProbeRun.mockResolvedValueOnce({
      id: 201,
      account_id: 181,
      mode: 'quick',
      request_mode: 'non_stream',
      status: 'success',
      model: 'gpt-4.1-mini',
      success_count: 1,
      failure_count: 0,
      avg_latency_ms: 123,
      total_tokens: 16,
      created_at: '2026-05-25T10:00:00Z'
    })

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

    expect(wrapper.text()).toContain('admin.accounts.probe.model')
    expect(wrapper.text()).toContain('gpt-4.1-mini')

    const historyButton = wrapper.findAll('button').find((button) => button.text().includes('gpt-4.1-mini'))
    expect(historyButton).toBeTruthy()
    await historyButton!.trigger('click')
    await flushPromises()

    expect(getProbeRun).toHaveBeenCalledWith(181, 201)
    expect(wrapper.text()).toContain('gpt-4.1-mini')
  })
})
