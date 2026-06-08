import type { AccountErrorHandlingAction, AccountErrorHandlingRuleForm } from './accountErrorHandlingTypes'

export const accountErrorHandlingActionSelectOptions = [
  { label: '只切号', value: 'retry_next' },
  { label: '限流', value: 'rate_limited' },
  { label: '临时不可调用', value: 'temp_unschedulable' },
  { label: '异常', value: 'error_disabled' }
] as const

export function accountErrorHandlingRuleKey(index: number): string {
  return `rule-${index}`
}

export function accountErrorHandlingActionLabel(action: AccountErrorHandlingAction): string {
  return accountErrorHandlingActionSelectOptions.find((item) => item.value === action)?.label ?? action
}

export function accountErrorHandlingActionColor(action: AccountErrorHandlingAction): string {
  if (action === 'retry_next') return 'blue'
  if (action === 'rate_limited') return 'orange'
  if (action === 'error_disabled') return 'red'
  return 'gold'
}

function compactAccountErrorValue(value: string): string {
  return value.trim().replace(/\s+/g, ' ')
}

export function accountErrorHandlingRuleConditionSummary(rule: AccountErrorHandlingRuleForm): string {
  const parts = [
    compactAccountErrorValue(rule.status_codes) ? `状态 ${compactAccountErrorValue(rule.status_codes)}` : '',
    compactAccountErrorValue(rule.error_codes) ? `码 ${compactAccountErrorValue(rule.error_codes)}` : '',
    compactAccountErrorValue(rule.error_types) ? `类型 ${compactAccountErrorValue(rule.error_types)}` : '',
    compactAccountErrorValue(rule.keywords) ? `关键词 ${compactAccountErrorValue(rule.keywords)}` : ''
  ].filter(Boolean)
  return parts.length > 0 ? parts.join(' / ') : '未配置匹配条件'
}
