import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountTestModal from '../AccountTestModal.vue'

const { getAvailableModels, copyToClipboard } = vi.hoisted(() => ({
  getAvailableModels: vi.fn(),
  copyToClipboard: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getAvailableModels
    }
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.accounts.imagePromptDefault': 'Generate a cute orange cat astronaut sticker on a clean pastel background.'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (key === 'admin.accounts.imageReceived' && params?.count) {
          return `received-${params.count}`
        }
        if (key === 'admin.accounts.firstTokenLatency') return `first-token-${params?.ms}ms`
        if (key === 'admin.accounts.testLatency') return `total-latency-${params?.ms}ms`
        return messages[key] || key
      }
    })
  }
})

function createStreamResponse(lines: string[]) {
  const encoder = new TextEncoder()
  const chunks = lines.map((line) => encoder.encode(line))
  let index = 0

  return {
    ok: true,
    body: {
      getReader: () => ({
        read: vi.fn().mockImplementation(async () => {
          if (index < chunks.length) {
            return { done: false, value: chunks[index++] }
          }
          return { done: true, value: undefined }
        })
      })
    }
  } as Response
}

function mountModal() {
  return mount(AccountTestModal, {
    props: {
      show: false,
      account: {
        id: 42,
        name: 'Gemini Image Test',
        platform: 'gemini',
        type: 'apikey',
        status: 'active'
      }
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: { template: '<div class="select-stub"></div>' },
        TextArea: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: '<textarea class="textarea-stub" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        },
        Icon: true,
        AccountProbeDialog: {
          props: ['show', 'account', 'modelId'],
          template: '<div class="probe-dialog-stub" :data-show="String(show)" :data-account-id="account?.id" :data-model-id="modelId"></div>'
        }
      }
    }
  })
}

describe('AccountTestModal', () => {
  beforeEach(() => {
    getAvailableModels.mockResolvedValue([
      { id: 'gemini-2.0-flash', display_name: 'Gemini 2.0 Flash' },
      { id: 'gemini-2.5-flash-image', display_name: 'Gemini 2.5 Flash Image' },
      { id: 'gemini-3.1-flash-image', display_name: 'Gemini 3.1 Flash Image' }
    ])
    copyToClipboard.mockReset()
    Object.defineProperty(globalThis, 'localStorage', {
      value: {
        getItem: vi.fn((key: string) => (key === 'auth_token' ? 'test-token' : null)),
        setItem: vi.fn(),
        removeItem: vi.fn(),
        clear: vi.fn()
      },
      configurable: true
    })
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_start","model":"gemini-2.5-flash-image"}\n',
        'data: {"type":"image","image_url":"data:image/png;base64,QUJD","mime_type":"image/png"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('gemini 图片模型测试会携带提示词并渲染图片预览', async () => {
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()

    const promptInput = wrapper.find('textarea.textarea-stub')
    expect(promptInput.exists()).toBe(true)
    await promptInput.setValue('draw a tiny orange cat astronaut')

    const buttons = wrapper.findAll('button')
    const startButton = buttons.find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()

    await startButton!.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toEqual({
      model_id: 'gemini-3.1-flash-image',
      prompt: 'draw a tiny orange cat astronaut'
    })

    const preview = wrapper.find('img[alt="test-image-1"]')
    expect(preview.exists()).toBe(true)
    expect(preview.attributes('src')).toBe('data:image/png;base64,QUJD')
  })

  it('OpenAI API Key 点击上游测速时打开独立测速弹窗', async () => {
    getAvailableModels.mockResolvedValueOnce([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])

    const wrapper = mount(AccountTestModal, {
      props: {
        show: false,
        account: {
          id: 128,
          name: 'encore',
          platform: 'openai',
          type: 'apikey',
          status: 'active'
        }
      } as any,
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          Select: { template: '<div class="select-stub"></div>' },
          TextArea: true,
          Icon: true,
          AccountProbeDialog: {
            props: ['show', 'account', 'modelId'],
            template: '<div class="probe-dialog-stub" :data-show="String(show)" :data-account-id="account?.id" :data-model-id="modelId"></div>'
          }
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    const toggle = wrapper.findAll('button').find((button) => button.text().includes('admin.accounts.probe.show'))
    expect(toggle).toBeTruthy()
    await toggle!.trigger('click')
    await flushPromises()

    expect(wrapper.emitted('close')).toBeUndefined()
    const probeDialog = wrapper.find('.probe-dialog-stub')
    expect(probeDialog.attributes('data-show')).toBe('true')
    expect(probeDialog.attributes('data-account-id')).toBe('128')
    expect(probeDialog.attributes('data-model-id')).toBe('gpt-5.4')
  })

  it('测试连接展示首字耗时并在缺少后端总耗时时使用本地耗时', async () => {
    getAvailableModels.mockResolvedValueOnce([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])
    const encoder = new TextEncoder()
    let now = 1_000
    let index = 0
    const chunks = [
      encoder.encode('data: {"type":"content","text":"ok","first_token_ms":234}\n'),
      encoder.encode('data: {"type":"test_complete","success":true}\n')
    ]
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      body: {
        getReader: () => ({
          read: vi.fn().mockImplementation(async () => {
            if (index === 1) now = 2_750
            if (index < chunks.length) {
              return { done: false, value: chunks[index++] }
            }
            return { done: true, value: undefined }
          })
        })
      }
    } as Response) as any

    const wrapper = mount(AccountTestModal, {
      props: {
        show: false,
        account: {
          id: 129,
          name: 'openai-key',
          platform: 'openai',
          type: 'apikey',
          status: 'active'
        }
      } as any,
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          Select: { template: '<div class="select-stub"></div>' },
          TextArea: true,
          Icon: true,
          AccountProbeDialog: true
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()
    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    vi.spyOn(Date, 'now').mockImplementation(() => now)
    await (wrapper.vm as any).startTest()
    await flushPromises()

    expect(wrapper.text()).toContain('first-token-234ms')
    expect(wrapper.text()).toContain('total-latency-1750ms')
  })

  it('free OpenAI accounts default to gpt-5.5 when testing', async () => {
    getAvailableModels.mockResolvedValueOnce([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' },
      { id: 'gpt-5.5', display_name: 'GPT-5.5' }
    ])

    const wrapper = mount(AccountTestModal, {
      props: {
        show: false,
        account: {
          id: 129,
          name: 'free-oauth',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          credentials: { plan_type: 'free' }
        }
      } as any,
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          Select: { template: '<div class="select-stub"></div>' },
          TextArea: true,
          Icon: true,
          AccountProbeDialog: true
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect((wrapper.vm as any).selectedModelId).toBe('gpt-5.5')
  })

  it('paid OpenAI accounts also default to gpt-5.5 when testing', async () => {
    getAvailableModels.mockResolvedValueOnce([
      { id: 'gpt-5.5', display_name: 'GPT-5.5' },
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])

    const wrapper = mount(AccountTestModal, {
      props: {
        show: false,
        account: {
          id: 130,
          name: 'team-oauth',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          credentials: { plan_type: 'team' }
        }
      } as any,
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          Select: { template: '<div class="select-stub"></div>' },
          TextArea: true,
          Icon: true,
          AccountProbeDialog: true
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect((wrapper.vm as any).selectedModelId).toBe('gpt-5.5')
  })
})
