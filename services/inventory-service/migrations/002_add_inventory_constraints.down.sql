ALTER TABLE inventory
    DROP CONSTRAINT IF EXISTS chk_inventory_available,
    DROP CONSTRAINT IF EXISTS chk_inventory_reserved;
