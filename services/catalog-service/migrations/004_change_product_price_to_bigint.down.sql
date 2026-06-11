ALTER TABLE products ALTER COLUMN price TYPE DECIMAL(10, 2) USING (price / 100.0)::numeric(10,2);
