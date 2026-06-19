DROP TABLE IF EXISTS outbox_dlq;

ALTER TABLE outbox
    DROP COLUMN IF EXISTS retry_count,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS next_retry_at;
