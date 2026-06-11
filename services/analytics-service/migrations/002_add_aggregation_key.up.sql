ALTER TABLE events ADD COLUMN IF NOT EXISTS aggregation_key String CODEC(ZSTD(1))
