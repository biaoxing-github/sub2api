import {
  loadAccountErrorHandlingRules as loadUnifiedAccountErrorHandlingRules,
  writeAccountErrorHandlingToCredentials
} from './accountErrorHandlingPayload'
import type { AccountErrorHandlingRuleForm } from './accountErrorHandlingTypes'

export {
  writeAccountErrorHandlingToCredentials
}

export function loadAccountErrorHandlingRules(credentials?: Record<string, unknown>): AccountErrorHandlingRuleForm[] {
  const unifiedRules = loadUnifiedAccountErrorHandlingRules(credentials)
  if (unifiedRules.length > 0) {
    return unifiedRules
  }
  if (!credentials) {
    return []
  }
  if (credentials.custom_error_codes_enabled !== true && credentials.temp_unschedulable_enabled !== true) {
    return []
  }

  const legacyCredentials = {
    customErrorCodesEnabled: credentials.custom_error_codes_enabled === true,
    customErrorCodes: Array.isArray(credentials.custom_error_codes)
      ? credentials.custom_error_codes.map((item) => Number(item))
      : [],
    tempUnschedEnabled: credentials.temp_unschedulable_enabled === true,
    tempUnschedRules: Array.isArray(credentials.temp_unschedulable_rules)
      ? (credentials.temp_unschedulable_rules as Array<{
          error_code: number
          keywords: string[]
          duration_minutes: number
          description: string
        }>)
      : []
  }

  const tempCredentials: Record<string, unknown> = {}
  applyLegacyErrorHandlingRules(tempCredentials, legacyCredentials)
  return loadUnifiedAccountErrorHandlingRules(tempCredentials)
}

export function applyLegacyErrorHandlingRules(
  credentials: Record<string, unknown>,
  input: {
    customErrorCodesEnabled: boolean
    customErrorCodes: number[]
    tempUnschedEnabled: boolean
    tempUnschedRules: Array<{
      error_code: number
      keywords: string[]
      duration_minutes: number
      description: string
    }>
  }
) {
  const existingRules = Array.isArray(credentials.error_handling_rules)
    ? credentials.error_handling_rules.filter((rule) => !isLegacyGeneratedRule(rule))
    : []
  const generatedRules = buildLegacyErrorHandlingRules(input)
  const nextRules = [...existingRules, ...generatedRules]

  if (nextRules.length > 0) {
    credentials.error_handling_rules = nextRules
  } else {
    delete credentials.error_handling_rules
  }
}

function buildLegacyErrorHandlingRules(input: {
  customErrorCodesEnabled: boolean
  customErrorCodes: number[]
  tempUnschedEnabled: boolean
  tempUnschedRules: Array<{
    error_code: number
    keywords: string[]
    duration_minutes: number
    description: string
  }>
}) {
  const rules: Array<Record<string, unknown>> = []
  let priority = 1

  if (input.customErrorCodesEnabled) {
    for (const code of normalizeStatusCodes(input.customErrorCodes)) {
      rules.push({
        enabled: true,
        name: `Legacy custom error ${code}`,
        priority,
        action: 'error_disabled',
        status_codes: [code],
        description: 'Generated from custom_error_codes',
        source: 'legacy_account_form'
      })
      priority += 1
    }
  }

  if (input.tempUnschedEnabled) {
    for (const rule of input.tempUnschedRules) {
      const code = Math.trunc(Number(rule.error_code))
      const duration = Math.trunc(Number(rule.duration_minutes))
      if (!isErrorStatusCode(code) || duration <= 0 || rule.keywords.length === 0) {
        continue
      }
      rules.push({
        enabled: true,
        name: rule.description || `Legacy temporary unavailable ${code}`,
        priority,
        action: 'temp_unschedulable',
        status_codes: [code],
        keywords: [...rule.keywords],
        durationMinutes: duration,
        description: rule.description || 'Generated from temp_unschedulable_rules',
        source: 'legacy_account_form'
      })
      priority += 1
    }
  }

  return rules
}

function isLegacyGeneratedRule(rule: unknown) {
  return Boolean(
    rule &&
      typeof rule === 'object' &&
      !Array.isArray(rule) &&
      (rule as Record<string, unknown>).source === 'legacy_account_form'
  )
}

function normalizeStatusCodes(codes: number[]) {
  const seen = new Set<number>()
  const out: number[] = []
  for (const code of codes) {
    const normalized = Math.trunc(Number(code))
    if (!isErrorStatusCode(normalized) || seen.has(normalized)) {
      continue
    }
    seen.add(normalized)
    out.push(normalized)
  }
  return out
}

function isErrorStatusCode(code: number) {
  return Number.isInteger(code) && code >= 100 && code <= 599 && (code < 200 || code > 299)
}
