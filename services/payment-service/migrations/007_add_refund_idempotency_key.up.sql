ALTER TABLE refunds ADD COLUMN idempotency_key VARCHAR(255) NOT NULL DEFAULT '';
CREATE UNIQUE INDEX idx_refunds_idempotency_key ON refunds(idempotency_key) WHERE idempotency_key <> '';
