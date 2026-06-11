CREATE TABLE IF NOT EXISTS events (
    event_type LowCardinality(String) CODEC(ZSTD(1)),
    aggregate_id String CODEC(ZSTD(1)),
    payload String CODEC(ZSTD(1)),
    amount Float64 CODEC(ZSTD(3)),
    currency LowCardinality(String) CODEC(ZSTD(1)),
    occurred_at DateTime,
    created_at DateTime,
    aggregation_key String CODEC(ZSTD(1))
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (occurred_at, aggregate_id)
TTL occurred_at + INTERVAL 2 YEAR
SETTINGS index_granularity = 8192
