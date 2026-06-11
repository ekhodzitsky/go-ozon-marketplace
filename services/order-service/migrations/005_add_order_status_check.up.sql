ALTER TABLE orders
    ADD CONSTRAINT chk_orders_status CHECK (status IN ('pending', 'awaiting_payment', 'confirmed', 'cancelled'));
