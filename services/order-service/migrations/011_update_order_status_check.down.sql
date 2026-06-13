ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS chk_orders_status,
    ADD CONSTRAINT chk_orders_status CHECK (status IN ('pending', 'awaiting_payment', 'confirmed', 'cancelled'));
