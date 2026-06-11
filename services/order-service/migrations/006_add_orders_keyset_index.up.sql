CREATE INDEX IF NOT EXISTS idx_orders_user_id_created_at_id ON orders(user_id, created_at DESC, id);
