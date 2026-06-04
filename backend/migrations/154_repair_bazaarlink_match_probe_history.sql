-- 修复旧版 BazaarLink 判定把 identityAssessment.status=match 且带 riskFlags 的结果误标为失败的历史数据。
-- 限定条件必须同时满足 BazaarLink 样本、旧 mismatch 错误码、HTTP 200、evidence 明确包含 status=match。
WITH matched_bazaarlink_samples AS (
    SELECT s.id
    FROM account_probe_samples s
    JOIN account_probe_runs r ON r.id = s.run_id
    WHERE (r.probe_source = 'bazaarlink_api' OR s.sample_type = 'bazaarlink_api')
      AND s.status = 'failed'
      AND s.http_status = 200
      AND s.error_code = 'bazaarlink_identity_mismatch'
      AND s.validation_evidence::text LIKE '%status=match%'
)
UPDATE account_probe_samples s
SET status = 'success',
    error_code = NULL,
    error_message = NULL,
    validation_evidence = COALESCE((
        SELECT jsonb_agg(
            CASE
                WHEN evidence.elem->>'key' = 'bazaarlink_identity' THEN
                    jsonb_set(
                        jsonb_set(
                            jsonb_set(
                                jsonb_set(
                                    evidence.elem,
                                    '{passed}',
                                    'true'::jsonb,
                                    true
                                ),
                                '{severity}',
                                CASE
                                    WHEN COALESCE(evidence.elem->>'observed', '') LIKE '%flags=%' THEN '"warning"'::jsonb
                                    ELSE '"info"'::jsonb
                                END,
                                true
                            ),
                            '{message}',
                            to_jsonb(
                                CASE
                                    WHEN COALESCE(evidence.elem->>'observed', '') LIKE '%flags=%'
                                        THEN 'BazaarLink 身份验证通过，存在风险提示：' || substring(evidence.elem->>'observed' from 'flags=(.*)$')
                                    ELSE 'BazaarLink 身份验证通过'
                                END
                            ),
                            true
                        ),
                        '{score}',
                        CASE
                            WHEN COALESCE(evidence.elem->>'score', '') ~ '^[1-9][0-9]*$' THEN to_jsonb((evidence.elem->>'score')::int)
                            WHEN COALESCE(evidence.elem->>'observed', '') LIKE '%confidence=0.98%' THEN '98'::jsonb
                            ELSE COALESCE(evidence.elem->'score', '0'::jsonb)
                        END,
                        true
                    )
                ELSE evidence.elem
            END
            ORDER BY evidence.ord
        )
        FROM jsonb_array_elements(s.validation_evidence) WITH ORDINALITY AS evidence(elem, ord)
    ), s.validation_evidence)
FROM matched_bazaarlink_samples m
WHERE s.id = m.id;

-- 依据修复后的样本状态重新汇总受影响 run，避免列表页继续显示 failed。
WITH affected_runs AS (
    SELECT DISTINCT s.run_id
    FROM account_probe_samples s
    JOIN account_probe_runs r ON r.id = s.run_id
    WHERE (r.probe_source = 'bazaarlink_api' OR s.sample_type = 'bazaarlink_api')
      AND s.http_status = 200
      AND s.validation_evidence::text LIKE '%status=match%'
),
run_stats AS (
    SELECT
        r.id,
        COUNT(s.id)::int AS sample_count,
        COUNT(*) FILTER (WHERE s.status = 'success')::int AS success_count,
        COUNT(*) FILTER (WHERE s.status = 'failed')::int AS failure_count,
        COALESCE(SUM(s.input_tokens), 0)::int AS input_tokens,
        COALESCE(SUM(s.output_tokens), 0)::int AS output_tokens,
        COALESCE(SUM(s.total_tokens), 0)::int AS total_tokens,
        COALESCE(ROUND(AVG(NULLIF(s.duration_ms, 0)))::int, 0) AS avg_ms,
        COALESCE(MAX(s.duration_ms), 0)::int AS max_ms
    FROM account_probe_runs r
    JOIN account_probe_samples s ON s.run_id = r.id
    JOIN affected_runs a ON a.run_id = r.id
    GROUP BY r.id
)
UPDATE account_probe_runs r
SET request_count = run_stats.sample_count,
    success_count = run_stats.success_count,
    failure_count = run_stats.failure_count,
    input_tokens = run_stats.input_tokens,
    output_tokens = run_stats.output_tokens,
    total_tokens = run_stats.total_tokens,
    avg_ms = run_stats.avg_ms,
    max_ms = run_stats.max_ms,
    status = CASE
        WHEN run_stats.failure_count = 0 THEN 'success'
        WHEN run_stats.success_count = 0 THEN 'failed'
        ELSE 'partial'
    END,
    error_message = CASE
        WHEN run_stats.failure_count = 0 THEN NULL
        ELSE r.error_message
    END,
    summary = CASE
        WHEN run_stats.failure_count = 0
            THEN format('完成 %s/%s 次请求，平均延迟 %s ms，消耗 %s tokens', run_stats.success_count, run_stats.sample_count, run_stats.avg_ms, run_stats.total_tokens)
        ELSE r.summary
    END
FROM run_stats
WHERE r.id = run_stats.id;
