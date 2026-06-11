DROP TRIGGER IF EXISTS trg_products_set_updated_at ON products;

ALTER TABLE products
    ALTER COLUMN created_at DROP NOT NULL,
    ALTER COLUMN updated_at DROP NOT NULL;

ALTER TABLE products DROP COLUMN IF EXISTS updated_at;
