export type OpsStreamActionLabel =
  | 'retry_no_avoidance'
  | 'retry_next_account'
  | 'avoid_account_ttl'
  | 'avoid_upstream_bucket_ttl'

export type OpsActionTemplateKey =
  | 'observe'
  | 'retryNoAvoidance'
  | 'retryNextAccount'
  | 'avoidAccountTtl'
  | 'avoidUpstreamBucketTtl'

export interface OpsActionTemplate {
  key: OpsActionTemplateKey
  labelKey: string
  descriptionKey: string
}

export interface OpsActionMetadataField {
  key: string
  labelKey: string
  value: string
}

const actionTemplateByLabel: Record<OpsStreamActionLabel, OpsActionTemplate> = {
  retry_no_avoidance: template('retryNoAvoidance'),
  retry_next_account: template('retryNextAccount'),
  avoid_account_ttl: template('avoidAccountTtl'),
  avoid_upstream_bucket_ttl: template('avoidUpstreamBucketTtl'),
}

const metadataFieldOrder = [
  ['stream_rule_id', 'streamRule'],
  ['avoidance_scope', 'avoidanceScope'],
  ['reason_scope', 'reasonScope'],
  ['retry_after', 'retryAfter'],
  ['path_health_state', 'pathHealthState'],
] as const

function template(key: OpsActionTemplateKey): OpsActionTemplate {
  return {
    key,
    labelKey: `admin.ops.actionTemplates.actions.${key}.label`,
    descriptionKey: `admin.ops.actionTemplates.actions.${key}.description`,
  }
}

export function resolveOpsStreamActionTemplate(label?: string | null): OpsActionTemplate {
  const normalized = String(label || '').trim() as OpsStreamActionLabel
  return actionTemplateByLabel[normalized] || template('observe')
}

export function summarizeOpsActionMetadata(metadata?: Record<string, unknown> | null): OpsActionMetadataField[] {
  if (!metadata) return []
  const fields: OpsActionMetadataField[] = []
  for (const [sourceKey, fieldKey] of metadataFieldOrder) {
    const value = metadata[sourceKey]
    if (value == null || value === '') continue
    fields.push({
      key: fieldKey,
      labelKey: `admin.ops.actionTemplates.fields.${fieldKey}`,
      value: String(value),
    })
  }
  return fields
}
