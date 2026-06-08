export type AccountErrorHandlingAction = 'retry_next' | 'rate_limited' | 'temp_unschedulable' | 'error_disabled'
export type AccountErrorHandlingResetStrategy = 'duration' | 'daily' | 'weekly'

export const accountErrorHandlingActionValues: AccountErrorHandlingAction[] = [
  'retry_next',
  'rate_limited',
  'temp_unschedulable',
  'error_disabled'
]

export interface AccountErrorHandlingRuleForm {
  enabled: boolean
  name: string
  priority: number | null
  status_codes: string
  error_codes: string
  error_types: string
  keywords: string
  action: AccountErrorHandlingAction
  durationMinutes: number | null
  reset_strategy: AccountErrorHandlingResetStrategy
  duration_hours: number | null
  daily_reset_hour: number | null
  weekly_reset_day: number | null
  weekly_reset_hour: number | null
  description: string
}

export interface AccountErrorHandlingRulePayload {
  enabled: boolean
  name: string
  priority: number
  status_codes?: number[]
  error_codes?: string[]
  error_types?: string[]
  keywords?: string[]
  action: AccountErrorHandlingAction
  durationMinutes?: number
  reset_strategy?: AccountErrorHandlingResetStrategy
  duration_hours?: number
  daily_reset_hour?: number
  weekly_reset_day?: number
  weekly_reset_hour?: number
  description?: string
}

export interface AccountErrorHandlingPreset {
  key: string
  label: string
  rule: AccountErrorHandlingRuleForm
}

export interface AccountErrorHandlingGuideContext {
  emptyDescription: string
}

export interface AccountErrorHandlingGuideField {
  key: string
  field: string
  source: string
  example: string
  note: string
}

export interface AccountErrorHandlingGuideSource {
  key: string
  name: string
  where: string
  note: string
}
