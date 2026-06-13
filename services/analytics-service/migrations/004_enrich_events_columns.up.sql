ALTER TABLE events
    ADD COLUMN IF NOT EXISTS amount Float64 CODEC(ZSTD(3)),
    ADD COLUMN IF NOT EXISTS currency LowCardinality(String) CODEC(ZSTD(1)),
    ADD COLUMN IF NOT EXISTS occurred_at DateTime;
