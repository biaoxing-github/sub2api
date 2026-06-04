-- 将旧版从 BazaarLink identity confidence 派生出来的历史 evidence 分数归零。
-- BazaarLink 探测得分只采用上游返回的顶层 score；confidence 只作为身份置信度展示。
WITH confidence_derived_scores AS (
    SELECT s.id
    FROM account_probe_samples s
    JOIN account_probe_runs r ON r.id = s.run_id
    CROSS JOIN LATERAL jsonb_array_elements(s.validation_evidence) AS evidence(elem)
    CROSS JOIN LATERAL (
        SELECT
            (evidence.elem->>'score')::int AS evidence_score,
            (evidence.elem->>'max_score')::int AS max_score,
            (
                substring(
                    evidence.elem->>'observed'
                    FROM 'confidence=([0-9]+[.]?[0-9]*)'
                )
            )::numeric AS confidence
    ) AS parsed
    WHERE (r.probe_source = 'bazaarlink_api' OR s.sample_type = 'bazaarlink_api')
      AND evidence.elem->>'key' = 'bazaarlink_identity'
      AND COALESCE(evidence.elem->>'score', '') ~ '^[1-9][0-9]*$'
      AND COALESCE(evidence.elem->>'max_score', '') ~ '^[1-9][0-9]*$'
      AND COALESCE(evidence.elem->>'observed', '') LIKE '%confidence=%'
      AND parsed.evidence_score = LEAST(
          GREATEST(
              ROUND(
                  CASE
                      WHEN parsed.confidence > 1 THEN parsed.confidence / 100
                      ELSE parsed.confidence
                  END * parsed.max_score
              )::int,
              0
          ),
          parsed.max_score
      )
)
UPDATE account_probe_samples s
SET validation_evidence = COALESCE((
    SELECT jsonb_agg(
        CASE
            WHEN evidence.elem->>'key' = 'bazaarlink_identity' THEN
                jsonb_set(
                    evidence.elem,
                    '{score}',
                    '0'::jsonb,
                    true
                )
            ELSE evidence.elem
        END
        ORDER BY evidence.ord
    )
    FROM jsonb_array_elements(s.validation_evidence) WITH ORDINALITY AS evidence(elem, ord)
), s.validation_evidence)
FROM confidence_derived_scores
WHERE s.id = confidence_derived_scores.id;
