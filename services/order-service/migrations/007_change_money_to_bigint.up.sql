ALTER TABLE orders ALTER COLUMN total_amount TYPE BIGINT USING (total_amount * 100)::bigint;
ALTER TABLE order_items ALTER COLUMN price TYPE BIGINT USING (price * 100)::bigint;
