import { describe, expect, it } from 'vitest'

import en from '../locales/en'

describe('admin group locale keys', () => {
  it('contains the English message for failed group saves', () => {
    expect(en.admin.groups.failedToSave).toBe('Failed to save group')
  })
})
