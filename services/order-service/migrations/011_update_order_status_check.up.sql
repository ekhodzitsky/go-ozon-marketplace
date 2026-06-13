ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_status;

UPDATE orders SET status = 'paid' WHERE status = 'confirmed';

ALTER TABLE orders ADD CONSTRAINT chk_orders_status CHECK (status IN (
    'pending',
    'awaiting_payment',
    'paid',
    'processing',
    'shipped',
    'delivered',
    'cancelled',
    'refunded'
));
