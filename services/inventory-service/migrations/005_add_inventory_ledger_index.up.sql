CREATE INDEX IF NOT EXISTS idx_inventory_ledger_product_created_at
    ON inventory_ledger(product_id, created_at DESC);
