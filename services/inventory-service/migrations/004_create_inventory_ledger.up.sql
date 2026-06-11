CREATE TABLE inventory_ledger (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id UUID NOT NULL,
  order_id UUID,
  quantity_change INT NOT NULL,
  operation_type VARCHAR(50) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
