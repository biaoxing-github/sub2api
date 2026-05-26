import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountAvailabilityRadarBadge from '../AccountAvailabilityRadarBadge.vue'
import type { AccountLoadFactorAdvice } from '@/types'

function advice(status: AccountLoadFactorAdvice['availability_radar']['status'], label: string, suggested?: number | null): AccountLoadFactorAdvice {
  return {
    suggested_load_factor: suggested,
    reasons: ['样本 8', '成功率 90%'],
    availability_radar: {
      status,
      label,
      reasons: ['样本 8']
    },
    path_health_samples: 8
  }
}

describe('AccountAvailabilityRadarBadge', () => {
  it.each([
    ['fast_stable', '快且稳'],
    ['slow_usable', '可用偏慢'],
    ['unstable', '不稳定'],
    ['balance_risk', '余额风险'],
    ['cooldown', '冷却中'],
    ['needs_probe', '待探测']
  ] as const)('renders %s radar label', (status, label) => {
    const wrapper = mount(AccountAvailabilityRadarBadge, {
      props: {
        advice: advice(status, label)
      }
    })

    expect(wrapper.text()).toContain(label)
  })

  it('renders readonly load factor suggestion when available', () => {
    const wrapper = mount(AccountAvailabilityRadarBadge, {
      props: {
        advice: advice('fast_stable', '快且稳', 12)
      }
    })

    expect(wrapper.text()).toContain('建议 12')
    expect(wrapper.attributes('title')).toBeUndefined()
    expect(wrapper.find('[title]').attributes('title')).toContain('成功率 90%')
  })
})
