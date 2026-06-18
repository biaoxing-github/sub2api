import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

import AccountQuotaControlSection from '../AccountQuotaControlSection.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const QuotaLimitCardStub = defineComponent({
  name: 'QuotaLimitCard',
  props: {
    totalLimit: {
      type: Number,
      default: null
    }
  },
  emits: ['update:totalLimit', 'update:quotaNotifyDailyEnabled'],
  template: `
    <button
      type="button"
      data-testid="quota-card"
      @click="$emit('update:totalLimit', 88); $emit('update:quotaNotifyDailyEnabled', true)"
    >
      quota {{ totalLimit }}
    </button>
  `
})

describe('AccountQuotaControlSection', () => {
  it('renders the selected hint and forwards quota card updates', async () => {
    const wrapper = mount(AccountQuotaControlSection, {
      props: {
        hintKey: 'admin.accounts.quotaLimitHint',
        totalLimit: 12,
        dailyLimit: null,
        weeklyLimit: null,
        dailyResetMode: null,
        dailyResetHour: null,
        weeklyResetMode: null,
        weeklyResetDay: null,
        weeklyResetHour: null,
        resetTimezone: null,
        quotaNotifyGlobalEnabled: true,
        quotaNotifyDailyEnabled: false,
        quotaNotifyDailyThreshold: null,
        quotaNotifyDailyThresholdType: null,
        quotaNotifyWeeklyEnabled: false,
        quotaNotifyWeeklyThreshold: null,
        quotaNotifyWeeklyThresholdType: null,
        quotaNotifyTotalEnabled: false,
        quotaNotifyTotalThreshold: null,
        quotaNotifyTotalThresholdType: null
      },
      global: {
        stubs: {
          QuotaLimitCard: QuotaLimitCardStub
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.quotaControl.title')
    expect(wrapper.text()).toContain('admin.accounts.quotaLimitHint')

    await wrapper.get('[data-testid="quota-card"]').trigger('click')
    expect(wrapper.emitted('update:totalLimit')?.[0]).toEqual([88])
    expect(wrapper.emitted('update:quotaNotifyDailyEnabled')?.[0]).toEqual([true])
  })
})
