import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ScheduledTestsPanel from '../ScheduledTestsPanel.vue'

const { listByAccount, createPlan, showError, showSuccess } = vi.hoisted(() => ({
  listByAccount: vi.fn(),
  createPlan: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    scheduledTests: {
      listByAccount,
      create: createPlan,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const BaseDialogStub = { template: '<section><slot /></section>' }
const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  methods: {
    onChange(event: Event) {
      this.$emit('update:modelValue', (event.target as HTMLSelectElement).value)
    },
  },
  template: '<select data-test="scheduled-test-model" :value="modelValue" @change="onChange"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>',
}
const InputStub = {
  props: ['modelValue'],
  emits: ['update:modelValue'],
  methods: {
    onInput(event: Event) {
      this.$emit('update:modelValue', (event.target as HTMLInputElement).value)
    },
  },
  template: '<input :value="modelValue" @input="onInput" />',
}
const ToggleStub = {
  props: ['modelValue'],
  emits: ['update:modelValue'],
  methods: {
    onChange(event: Event) {
      this.$emit('update:modelValue', (event.target as HTMLInputElement).checked)
    },
  },
  template: '<input type="checkbox" :checked="modelValue" @change="onChange" />',
}

describe('ScheduledTestsPanel', () => {
  beforeEach(() => {
    listByAccount.mockReset()
    createPlan.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    listByAccount.mockResolvedValue([])
  })

  it('preselects gpt-5.6-terra when creating scheduled account tests', async () => {
    const wrapper = mount(ScheduledTestsPanel, {
      props: {
        show: true,
        accountId: 12,
        modelOptions: [
          { value: 'gpt-5.4', label: 'GPT-5.4' },
          { value: 'gpt-5.5', label: 'GPT-5.5' },
          { value: 'gpt-5.6-terra', label: 'GPT-5.6 Terra' },
        ],
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          HelpTooltip: { template: '<span><slot /><slot name="trigger" /></span>' },
          Select: SelectStub,
          Input: InputStub,
          Toggle: ToggleStub,
          Icon: true,
        },
      },
    })
    await flushPromises()

    const addButton = wrapper.findAll('button').find(button => button.text().includes('admin.scheduledTests.addPlan'))
    expect(addButton).toBeTruthy()
    await addButton!.trigger('click')
    await flushPromises()

    expect((wrapper.find('[data-test="scheduled-test-model"]').element as HTMLSelectElement).value).toBe('gpt-5.6-terra')
  })
})
