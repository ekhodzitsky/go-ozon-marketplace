DROP TRIGGER IF EXISTS trg_payments_set_updated_at ON payments;

ALTER TABLE payments
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS updated_at;
