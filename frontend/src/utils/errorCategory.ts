/** 将错误 phase/type 映射为用户可理解的稳定分类码。 */
export function mapErrorCategory(phase?: string | null, errorType?: string | null): string {
  switch ((phase || '').toLowerCase()) {
    case 'auth':
      return 'auth'
    case 'routing':
      return 'service_unavailable'
    case 'upstream':
    case 'network':
      return 'upstream'
    case 'internal':
      return 'internal'
    case 'request':
      switch ((errorType || '').toLowerCase()) {
        case 'rate_limit_error':
          return 'rate_limit'
        case 'billing_error':
        case 'subscription_error':
          return 'quota'
        case 'invalid_request_error':
          return 'invalid_request'
        case 'cyber_policy':
          return 'cyber'
      }
  }
  return 'other'
}
