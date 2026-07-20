import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('payment plan validity locale keys', () => {
  it('uses unit-neutral English labels', () => {
    expect(en.payment.admin.validity).toBe('Validity')
    expect(en.payment.admin.validityRequired).toBe('Validity must be greater than 0')
  })

  it('uses unit-neutral Chinese labels', () => {
    expect(zh.payment.admin.validity).toBe('有效期')
    expect(zh.payment.admin.validityRequired).toBe('有效期必须大于 0')
  })
})
