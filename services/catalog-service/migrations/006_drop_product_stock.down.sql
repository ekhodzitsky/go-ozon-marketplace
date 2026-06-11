ALTER TABLE products
    ADD COLUMN stock INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT chk_products_stock CHECK (stock >= 0);
