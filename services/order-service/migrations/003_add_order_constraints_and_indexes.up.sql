ALTER TABLE order_items
    ADD CONSTRAINT chk_order_items_quantity CHECK (quantity >= 0),
    ADD CONSTRAINT chk_order_items_price CHECK (price >= 0);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_orders_user_id_created_at ON orders(user_id, created_at DESC);
