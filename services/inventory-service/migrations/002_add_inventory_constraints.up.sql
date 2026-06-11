ALTER TABLE inventory
    ADD CONSTRAINT chk_inventory_available CHECK (available >= 0),
    ADD CONSTRAINT chk_inventory_reserved CHECK (reserved >= 0);
