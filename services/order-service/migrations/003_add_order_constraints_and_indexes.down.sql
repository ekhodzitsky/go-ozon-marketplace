ALTER TABLE order_items
    DROP CONSTRAINT IF EXISTS chk_order_items_quantity,
    DROP CONSTRAINT IF EXISTS chk_order_items_price;

DROP INDEX IF EXISTS idx_order_items_order_id;
DROP INDEX IF EXISTS idx_orders_user_id_created_at;
