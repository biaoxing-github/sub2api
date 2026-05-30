ALTER TABLE account_batch_test_items
    ADD COLUMN IF NOT EXISTS first_token_ms INTEGER;

ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS first_token_ms BIGINT;
