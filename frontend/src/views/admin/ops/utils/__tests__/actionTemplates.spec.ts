import { describe, expect, it } from 'vitest'
import { resolveOpsStreamActionTemplate, summarizeOpsActionMetadata } from '../actionTemplates'

describe('ops action templates', () => {
  it('maps stream action labels to stable i18n template keys', () => {
    expect(resolveOpsStreamActionTemplate('retry_no_avoidance').key).toBe('retryNoAvoidance')
    expect(resolveOpsStreamActionTemplate('retry_next_account').key).toBe('retryNextAccount')
    expect(resolveOpsStreamActionTemplate('avoid_account_ttl').key).toBe('avoidAccountTtl')
    expect(resolveOpsStreamActionTemplate('avoid_upstream_bucket_ttl').key).toBe('avoidUpstreamBucketTtl')
  })

  it('falls back to observe template for unknown or missing labels', () => {
    expect(resolveOpsStreamActionTemplate('').key).toBe('observe')
    expect(resolveOpsStreamActionTemplate('unknown_action').key).toBe('observe')
  })

  it('summarizes action metadata in operator-facing order', () => {
    const fields = summarizeOpsActionMetadata({
      action_label: 'avoid_upstream_bucket_ttl',
      avoidance_scope: 'upstream_bucket',
      reason_scope: 'stream',
      retry_after: '2026-06-07T23:59:00Z',
      path_health_state: 'open_circuit',
      stream_rule_id: 'openai_stream_capacity_or_overload',
    })

    expect(fields.map((field) => field.key)).toEqual([
      'streamRule',
      'avoidanceScope',
      'reasonScope',
      'retryAfter',
      'pathHealthState',
    ])
  })
})
