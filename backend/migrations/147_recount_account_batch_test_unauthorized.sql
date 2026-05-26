UPDATE account_batch_test_runs r
SET unauthorized_count = COALESCE(items.unauthorized_count, 0)
FROM (
  SELECT run_id, COUNT(*) FILTER (WHERE category = 'unauthorized') AS unauthorized_count
  FROM account_batch_test_items
  GROUP BY run_id
) items
WHERE r.id = items.run_id;

UPDATE account_batch_test_runs
SET unauthorized_count = 0
WHERE id NOT IN (
  SELECT DISTINCT run_id
  FROM account_batch_test_items
  WHERE category = 'unauthorized'
);
