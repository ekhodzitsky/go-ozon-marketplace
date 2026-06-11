ALTER TABLE payments
    ADD CONSTRAINT chk_payments_status CHECK (status IN ('pending', 'success', 'failed', 'refunded'));
