ALTER TABLE products
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255) UNIQUE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_products_idempotency_key ON products(idempotency_key);
