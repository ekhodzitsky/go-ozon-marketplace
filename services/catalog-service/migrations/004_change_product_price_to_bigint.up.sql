ALTER TABLE products ALTER COLUMN price TYPE BIGINT USING (price * 100)::bigint;
