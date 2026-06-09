CREATE TABLE IF NOT EXISTS inventory (
    product_id UUID PRIMARY KEY,
    available INT NOT NULL DEFAULT 0,
    reserved INT NOT NULL DEFAULT 0
);
