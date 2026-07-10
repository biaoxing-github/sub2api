import { describe, expect, it } from 'vitest'

import { durationSeverity, firstTokenSeverity } from '../latencyHealth'

describe('latencyHealth', () => {
  it('按 10s/30s/60s 边界划分首字延迟', () => {
    expect(firstTokenSeverity(9_999)).toBe('good')
    expect(firstTokenSeverity(10_000)).toBe('warn')
    expect(firstTokenSeverity(30_000)).toBe('slow')
    expect(firstTokenSeverity(60_000)).toBe('critical')
  })

  it('按 1min/3min/5min 边界划分总耗时', () => {
    expect(durationSeverity(59_999)).toBe('good')
    expect(durationSeverity(60_000)).toBe('warn')
    expect(durationSeverity(180_000)).toBe('slow')
    expect(durationSeverity(300_000)).toBe('critical')
  })
})
