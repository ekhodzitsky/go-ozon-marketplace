ALTER TABLE products
    DROP CONSTRAINT IF EXISTS chk_products_stock,
    DROP COLUMN IF EXISTS stock;
