ALTER TABLE payments
    ADD CONSTRAINT chk_payments_amount CHECK (amount >= 0),
    ADD CONSTRAINT uq_payments_order_id UNIQUE (order_id);
