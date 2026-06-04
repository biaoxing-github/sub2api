-- 修正旧 BazaarLink 快速验证历史数据：从 V3 candidates 中回填声明模型候选分。
-- 旧记录可能只有 response_body 候选片段，但 validation_evidence.score/display_score 仍为 0/空。
DO $$
DECLARE
    sample_record RECORD;
    evidence_record RECORD;
    candidate_record RECORD;
    candidates_text TEXT;
    candidates_json JSONB;
    expected_model TEXT;
    normalized_expected TEXT;
    normalized_model_id TEXT;
    normalized_display_name TEXT;
    raw_candidate_score NUMERIC;
    percent_score NUMERIC;
    max_score INTEGER;
    rounded_score INTEGER;
    v3_fragment TEXT;
    updated_evidence JSONB;
    updated_item JSONB;
    changed BOOLEAN;
BEGIN
    FOR sample_record IN
        SELECT
            s.id,
            s.model AS sample_model,
            s.response_body,
            s.validation_evidence,
            r.model AS run_model
        FROM account_probe_samples s
        JOIN account_probe_runs r ON r.id = s.run_id
        WHERE (r.probe_source = 'bazaarlink_api' OR s.sample_type = 'bazaarlink_api')
          AND r.request_mode = 'quick'
          AND s.validation_evidence IS NOT NULL
          AND s.response_body LIKE '%"candidates"%'
    LOOP
        IF strpos(sample_record.response_body, '"v3"') > 0 THEN
            v3_fragment := substr(sample_record.response_body, strpos(sample_record.response_body, '"v3"'));
        ELSE
            v3_fragment := sample_record.response_body;
        END IF;

        candidates_text := substring(
            v3_fragment
            FROM '"candidates"[[:space:]]*:[[:space:]]*(\[[^]]+\])'
        );
        IF candidates_text IS NULL OR candidates_text = '' THEN
            CONTINUE;
        END IF;

        BEGIN
            candidates_json := candidates_text::jsonb;
        EXCEPTION WHEN others THEN
            CONTINUE;
        END;

        updated_evidence := '[]'::jsonb;
        changed := false;

        FOR evidence_record IN
            SELECT elem, ord
            FROM jsonb_array_elements(sample_record.validation_evidence) WITH ORDINALITY AS evidence(elem, ord)
        LOOP
            updated_item := evidence_record.elem;

            IF evidence_record.elem->>'key' = 'bazaarlink_identity'
               AND COALESCE(evidence_record.elem->>'max_score', '') ~ '^[1-9][0-9]*$' THEN
                expected_model := COALESCE(
                    NULLIF(evidence_record.elem->>'expected_model', ''),
                    NULLIF(evidence_record.elem->>'expected', ''),
                    NULLIF(sample_record.sample_model, ''),
                    NULLIF(sample_record.run_model, '')
                );
                normalized_expected := regexp_replace(
                    lower(regexp_replace(COALESCE(expected_model, ''), '^.*/', '')),
                    '[[:space:]_-]+',
                    '',
                    'g'
                );
                raw_candidate_score := NULL;

                IF normalized_expected <> '' THEN
                    FOR candidate_record IN
                        SELECT value AS elem
                        FROM jsonb_array_elements(candidates_json)
                    LOOP
                        IF COALESCE(candidate_record.elem->>'score', '') !~ '^[0-9]+([.][0-9]+)?$' THEN
                            CONTINUE;
                        END IF;

                        normalized_model_id := regexp_replace(
                            lower(regexp_replace(COALESCE(candidate_record.elem->>'modelId', ''), '^.*/', '')),
                            '[[:space:]_-]+',
                            '',
                            'g'
                        );
                        normalized_display_name := regexp_replace(
                            lower(regexp_replace(COALESCE(candidate_record.elem->>'displayName', ''), '^.*/', '')),
                            '[[:space:]_-]+',
                            '',
                            'g'
                        );

                        IF normalized_model_id = normalized_expected
                           OR normalized_display_name = normalized_expected
                           OR (normalized_model_id <> '' AND normalized_model_id LIKE '%' || normalized_expected)
                           OR (normalized_display_name <> '' AND normalized_display_name LIKE '%' || normalized_expected)
                           OR (normalized_expected <> '' AND normalized_expected LIKE '%' || normalized_model_id)
                           OR (normalized_expected <> '' AND normalized_expected LIKE '%' || normalized_display_name) THEN
                            raw_candidate_score := (candidate_record.elem->>'score')::numeric;
                            EXIT;
                        END IF;
                    END LOOP;
                END IF;

                IF raw_candidate_score IS NOT NULL THEN
                    percent_score := LEAST(
                        GREATEST(
                            CASE
                                WHEN raw_candidate_score <= 1 THEN raw_candidate_score * 100
                                ELSE raw_candidate_score
                            END,
                            0
                        ),
                        100
                    );
                    max_score := (evidence_record.elem->>'max_score')::int;
                    rounded_score := LEAST(
                        GREATEST(ROUND(percent_score * max_score / 100)::int, 0),
                        max_score
                    );
                    updated_item := evidence_record.elem
                        || jsonb_build_object('score', rounded_score, 'display_score', percent_score);
                    changed := true;
                END IF;
            END IF;

            updated_evidence := updated_evidence || jsonb_build_array(updated_item);
        END LOOP;

        IF changed THEN
            UPDATE account_probe_samples
            SET validation_evidence = updated_evidence
            WHERE id = sample_record.id;
        END IF;
    END LOOP;
END $$;
