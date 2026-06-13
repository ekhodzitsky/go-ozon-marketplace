DROP INDEX IF EXISTS idx_products_idempotency_key;
ALTER TABLE products DROP COLUMN IF EXISTS idempotency_key;
