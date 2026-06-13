ALTER TABLE outbox
    ADD COLUMN IF NOT EXISTS retry_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_error TEXT,
    ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW();

CREATE TABLE IF NOT EXISTS outbox_dlq (
    id UUID PRIMARY KEY,
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    failed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_error TEXT,
    retry_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_outbox_dlq_aggregate ON outbox_dlq(aggregate_type, aggregate_id);
