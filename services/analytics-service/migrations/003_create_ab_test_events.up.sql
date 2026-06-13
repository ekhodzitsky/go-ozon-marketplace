CREATE TABLE IF NOT EXISTS ab_test_events (
    event_id UUID,
    experiment String CODEC(ZSTD(1)),
    variation String CODEC(ZSTD(1)),
    user_id UUID,
    conversion Bool,
    revenue_minor Int64,
    created_at DateTime
) ENGINE = MergeTree()
ORDER BY created_at
