ALTER TABLE products
    DROP CONSTRAINT IF EXISTS chk_products_price,
    DROP CONSTRAINT IF EXISTS chk_products_stock;
