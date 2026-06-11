ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS chk_payments_amount,
    DROP CONSTRAINT IF EXISTS uq_payments_order_id;
