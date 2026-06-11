ALTER TABLE orders ALTER COLUMN total_amount TYPE DECIMAL(12,2) USING (total_amount / 100.0)::numeric(12,2);
ALTER TABLE order_items ALTER COLUMN price TYPE DECIMAL(12,2) USING (price / 100.0)::numeric(12,2);
