UPDATE account_batch_test_items
SET category = 'rate_limited'
WHERE category <> 'rate_limited'
  AND (
    LOWER(COALESCE(error_message, '')) LIKE '%429%'
    OR LOWER(COALESCE(error_message, '')) LIKE '%rate limit%'
    OR LOWER(COALESCE(error_message, '')) LIKE '%rate_limited%'
    OR LOWER(COALESCE(error_message, '')) LIKE '%too many requests%'
  );
