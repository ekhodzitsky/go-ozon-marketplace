DROP TRIGGER IF EXISTS trg_orders_set_updated_at ON orders;
DROP TRIGGER IF EXISTS trg_outbox_set_updated_at ON outbox;

ALTER TABLE orders
    ALTER COLUMN created_at DROP NOT NULL,
    ALTER COLUMN updated_at DROP NOT NULL;

ALTER TABLE outbox
    ALTER COLUMN created_at DROP NOT NULL,
    ALTER COLUMN updated_at DROP NOT NULL;

ALTER TABLE outbox DROP COLUMN IF EXISTS updated_at;
