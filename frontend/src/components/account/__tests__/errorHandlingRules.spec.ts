import { describe, expect, it } from 'vitest'
import {
  applyLegacyErrorHandlingRules,
  loadAccountErrorHandlingRules,
  writeAccountErrorHandlingToCredentials
} from '../errorHandlingRules'

describe('account error handling rules helpers', () => {
  it('loads legacy custom codes and temporary rules as editable unified form rules', () => {
    const rules = loadAccountErrorHandlingRules({
      custom_error_codes_enabled: true,
      custom_error_codes: [401, 429],
      temp_unschedulable_enabled: true,
      temp_unschedulable_rules: [
        {
          error_code: 529,
          keywords: ['overloaded', 'too many'],
          duration_minutes: 10,
          description: 'temporary overload'
        }
      ]
    })

    expect(rules).toHaveLength(3)
    expect(rules[0]).toMatchObject({
      action: 'error_disabled',
      status_codes: '401',
      priority: 1
    })
    expect(rules[1]).toMatchObject({
      action: 'error_disabled',
      status_codes: '429',
      priority: 2
    })
    expect(rules[2]).toMatchObject({
      action: 'temp_unschedulable',
      status_codes: '529',
      keywords: 'overloaded, too many',
      durationMinutes: 10
    })
  })

  it('replaces generated legacy rules without dropping custom unified rules', () => {
    const credentials: Record<string, unknown> = {
      error_handling_rules: [
        {
          enabled: true,
          name: 'hand written retry',
          priority: 50,
          action: 'retry_next',
          status_codes: [500]
        },
        {
          enabled: true,
          name: 'old generated',
          priority: 1,
          action: 'error_disabled',
          status_codes: [401],
          source: 'legacy_account_form'
        }
      ]
    }

    applyLegacyErrorHandlingRules(credentials, {
      customErrorCodesEnabled: true,
      customErrorCodes: [403],
      tempUnschedEnabled: false,
      tempUnschedRules: []
    })

    expect(credentials.error_handling_rules).toEqual([
      {
        enabled: true,
        name: 'hand written retry',
        priority: 50,
        action: 'retry_next',
        status_codes: [500]
      },
      {
        enabled: true,
        name: 'Legacy custom error 403',
        priority: 1,
        action: 'error_disabled',
        status_codes: [403],
        description: 'Generated from custom_error_codes',
        source: 'legacy_account_form'
      }
    ])
  })

  it('writes form rules to the unified payload schema', () => {
    const credentials: Record<string, unknown> = {}

    writeAccountErrorHandlingToCredentials(credentials, [
      {
        enabled: true,
        name: 'daily quota',
        priority: 10,
        status_codes: '429',
        error_codes: 'insufficient_quota',
        error_types: '',
        keywords: 'quota exceeded',
        action: 'rate_limited',
        durationMinutes: null,
        reset_strategy: 'daily',
        duration_hours: null,
        daily_reset_hour: 0,
        weekly_reset_day: null,
        weekly_reset_hour: null,
        description: 'recover at midnight'
      }
    ])

    expect(credentials.error_handling_rules).toEqual([
      {
        enabled: true,
        name: 'daily quota',
        priority: 10,
        action: 'rate_limited',
        status_codes: [429],
        error_codes: ['insufficient_quota'],
        keywords: ['quota exceeded'],
        description: 'recover at midnight',
        reset_strategy: 'daily',
        daily_reset_hour: 0
      }
    ])
  })
})
